// internal/sysinfo/sysinfo.go
package sysinfo

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Snapshot struct {
	TS int64 `json:"ts"`

	Jack JackInfo `json:"jack"`

	Routing RoutingInfo `json:"routing"`
	MIDI    MidiInfo    `json:"midi"`

	AudioIF AudioIFInfo `json:"audioif"`

	CPU  CPUInfo  `json:"cpu"`
	Mem  MemInfo  `json:"mem"`
	Disk DiskInfo `json:"disk"`

	Errors []string `json:"errors,omitempty"`
}

type JackInfo struct {
	Running bool `json:"running"`

	Driver  string `json:"driver"`
	Device  string `json:"device"`
	SR      int    `json:"sr"`
	Buf     int    `json:"buf"`
	Periods int    `json:"periods"`
	RTPrio  int    `json:"rtprio"`
	RT      bool   `json:"realtime"`

	Xruns      int    `json:"xruns"`
	XrunsDelta int    `json:"xruns_delta"`
	LastXrun   string `json:"last_xrun,omitempty"`

	LatencyRTMs float64 `json:"latency_rt_ms"`
}

type RoutingInfo struct {
	OK       bool     `json:"ok"`
	Missing  []string `json:"missing,omitempty"`
	Ports    int      `json:"ports"`
	Edges    int      `json:"edges"`
	Summary  []string `json:"summary,omitempty"`
	RawPorts []string `json:"raw_ports,omitempty"` // opcional; lo dejamos vacío por defecto
}

type MidiInfo struct {
	ALSA []string `json:"alsa,omitempty"`
	Jack []string `json:"jack,omitempty"`

	Connected bool   `json:"connected"`
	Details   string `json:"details,omitempty"`
}

type AudioIFInfo struct {
	AsoundCards []string `json:"asound_cards,omitempty"`
}

type CPUInfo struct {
	Load1     float64 `json:"load1"`
	Governor  string  `json:"governor,omitempty"`
	TempC     float64 `json:"temp_c,omitempty"`
	Throttled bool    `json:"throttled,omitempty"` // best-effort
}

type MemInfo struct {
	TotalMB int `json:"total_mb"`
	UsedMB  int `json:"used_mb"`
}

type DiskInfo struct {
	RootFreeGB float64 `json:"root_free_gb"`
}

type Collector struct {
	mu sync.Mutex

	cache      Snapshot
	cacheAt    time.Time
	cacheTTL   time.Duration
	lastXruns  int
	lastXrunAt string

	jackUnitPath string
}
func jackdUIDGID() (uid int, gid int, err error) {
	// Busca el PID de jackd
	out, err := exec.Command("pidof", "jackd").Output()
	if err != nil {
		return 0, 0, fmt.Errorf("pidof jackd failed: %w", err)
	}
	pids := strings.Fields(strings.TrimSpace(string(out)))
	if len(pids) == 0 {
		return 0, 0, fmt.Errorf("jackd pid not found")
	}

	// Usa el primer PID
	status := fmt.Sprintf("/proc/%s/status", pids[0])
	lines, err := readFileLines(status, 0)
	if err != nil {
		return 0, 0, err
	}

	for _, l := range lines {
		if strings.HasPrefix(l, "Uid:") {
			fmt.Sscanf(l, "Uid:\t%d", &uid)
		}
		if strings.HasPrefix(l, "Gid:") {
			fmt.Sscanf(l, "Gid:\t%d", &gid)
		}
	}
	if uid == 0 {
		return 0, 0, fmt.Errorf("could not resolve jackd uid/gid")
	}
	return uid, gid, nil
}

func NewCollector() *Collector {
	return &Collector{
		cacheTTL:     1500 * time.Millisecond,
		jackUnitPath: "/etc/systemd/system/jackd.service",
	}
}

func (c *Collector) Snapshot(ctx context.Context) Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.cacheAt.IsZero() && time.Since(c.cacheAt) < c.cacheTTL {
		return c.cache
	}

	snap := Snapshot{
		TS: time.Now().Unix(),
	}

	// JACK: parse unit file (authoritative) + runtime checks
	jackCfg, err := parseJackdUnit(c.jackUnitPath)
	if err != nil {
		snap.Errors = append(snap.Errors, "jack unit parse: "+err.Error())
	} else {
		snap.Jack.Driver = jackCfg.Driver
		snap.Jack.Device = jackCfg.Device
		snap.Jack.SR = jackCfg.SR
		snap.Jack.Buf = jackCfg.Buf
		snap.Jack.Periods = jackCfg.Periods
		snap.Jack.RTPrio = jackCfg.RTPrio
		snap.Jack.RT = jackCfg.RT
		if snap.Jack.SR > 0 && snap.Jack.Buf > 0 && snap.Jack.Periods > 0 {
			// round-trip estimate: (buf*periods*2)/sr * 1000
			snap.Jack.LatencyRTMs = (float64(snap.Jack.Buf*snap.Jack.Periods*2) / float64(snap.Jack.SR)) * 1000.0
		}
	}

	snap.Jack.Running = isServiceActive(ctx, "jackd")

	// XRuns: journalctl parse (cached via collector)
	xruns, lastLine, xerr := countXruns(ctx)
	if xerr != nil {
		snap.Errors = append(snap.Errors, "xruns: "+xerr.Error())
	} else {
		snap.Jack.Xruns = xruns
		snap.Jack.LastXrun = lastLine
		snap.Jack.XrunsDelta = xruns - c.lastXruns
		if lastLine != "" {
			c.lastXrunAt = lastLine
		}
		c.lastXruns = xruns
	}

	// JACK routing + ports
	portsOut, edgesOut, rSum, missing, rok, rerr := collectRouting(ctx)
	if rerr != nil {
		snap.Errors = append(snap.Errors, "routing: "+rerr.Error())
	} else {
		snap.Routing.Ports = portsOut
		snap.Routing.Edges = edgesOut
		snap.Routing.Summary = rSum
		snap.Routing.Missing = missing
		snap.Routing.OK = rok
	}

	// MIDI (ALSA + JACK MIDI ports + connection heuristic)
	alsa, aerr := runLines(ctx, 1200*time.Millisecond, "aconnect", "-l")
	if aerr != nil {
		snap.Errors = append(snap.Errors, "midi alsa: "+aerr.Error())
	} else {
		snap.MIDI.ALSA = alsa
	}

	jm, jerr := jackMidiPorts(ctx)
	if jerr != nil {
		snap.Errors = append(snap.Errors, "midi jack: "+jerr.Error())
	} else {
		snap.MIDI.Jack = jm
	}

	connected, details := midiConnectionStatus(rSum)
	snap.MIDI.Connected = connected
	snap.MIDI.Details = details

	// Audio interface identity (asound cards)
	asound, cerr := readFileLines("/proc/asound/cards", 120)
	if cerr == nil {
		snap.AudioIF.AsoundCards = asound
	}

	// CPU + governor + temp
	load1, _ := readLoad1()
	snap.CPU.Load1 = load1
	snap.CPU.Governor, _ = readGovernor()
	snap.CPU.TempC, _ = readTempC()

	// Mem + disk
	snap.Mem = readMem()
	snap.Disk = readDisk("/")

	// Store cache
	c.cache = snap
	c.cacheAt = time.Now()
	return snap
}

type jackUnitCfg struct {
	Driver  string
	Device  string
	SR      int
	Buf     int
	Periods int
	RTPrio  int
	RT      bool
}

func parseJackdUnit(path string) (jackUnitCfg, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return jackUnitCfg{}, err
	}
	txt := string(b)

	// Find ExecStart=...
	var execLine string
	sc := bufio.NewScanner(strings.NewReader(txt))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "ExecStart=") {
			execLine = strings.TrimPrefix(line, "ExecStart=")
			break
		}
	}
	if execLine == "" {
		return jackUnitCfg{}, errors.New("ExecStart not found")
	}

	// Tokenize (simple split; args have no quotes in your unit)
	args := strings.Fields(execLine)

	cfg := jackUnitCfg{
		Driver: "alsa",
		RT:     contains(args, "-R"),
	}

	// Parse -P95, -dhw:..., -r48000, -p256, -n2
	for i := 0; i < len(args); i++ {
		a := args[i]

		if strings.HasPrefix(a, "-P") {
			cfg.RTPrio, _ = strconv.Atoi(strings.TrimPrefix(a, "-P"))
			continue
		}
		if a == "-dalsa" {
			cfg.Driver = "alsa"
			continue
		}
		if strings.HasPrefix(a, "-d") && a != "-dalsa" {
			// e.g. -dhw:CARD=Audio,DEV=0
			cfg.Device = strings.TrimPrefix(a, "-d")
			continue
		}
		if strings.HasPrefix(a, "-r") {
			cfg.SR, _ = strconv.Atoi(strings.TrimPrefix(a, "-r"))
			continue
		}
		if strings.HasPrefix(a, "-p") {
			cfg.Buf, _ = strconv.Atoi(strings.TrimPrefix(a, "-p"))
			continue
		}
		if strings.HasPrefix(a, "-n") {
			cfg.Periods, _ = strconv.Atoi(strings.TrimPrefix(a, "-n"))
			continue
		}
	}

	// sanity: your unit uses hw:... in -d
	if cfg.Device == "" {
		cfg.Device = "hw:UNKNOWN"
	}
	return cfg, nil
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func isServiceActive(ctx context.Context, name string) bool {
	out, err := run(ctx, 900*time.Millisecond, "systemctl", "is-active", name)
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) == "active"
}

func countXruns(ctx context.Context) (count int, lastLine string, err error) {
	// We keep it short; if needed, increase -n. Cache avoids heavy calls.
	out, err := run(ctx, 1500*time.Millisecond, "journalctl", "-u", "jackd", "-n", "250", "--no-pager", "-o", "short-iso")
	if err != nil {
		return 0, "", err
	}

	sc := bufio.NewScanner(strings.NewReader(out))
	re := regexp.MustCompile(`(?i)\bxrun\b`)
	for sc.Scan() {
		line := sc.Text()
		if re.MatchString(line) {
			count++
			lastLine = line
		}
	}
	return count, lastLine, nil
}

func collectRouting(ctx context.Context) (ports int, edges int, summary []string, missing []string, ok bool, err error) {
	out, err := run(ctx, 1200*time.Millisecond, "jack_lsp", "-c")
	if err != nil && isServiceActive(ctx, "jackd") {
		if uid, gid, e := jackdUIDGID(); e == nil {
			out2, err2 := runAsUIDGID(ctx, 1500*time.Millisecond, uid, gid, "jack_lsp", "-c")
			if err2 == nil {
				out = out2
				err = nil
			}
		}
	}
	if err != nil {
		return 0, 0, nil, nil, false, err
	}

	// Parse jack_lsp -c:
	// PortName
	//    connectedPort
	//    connectedPort
	// NextPort
	type node struct {
		name  string
		conns []string
	}
	nodes := []node{}
	var cur *node

	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			n := node{name: strings.TrimSpace(line)}
			nodes = append(nodes, n)
			cur = &nodes[len(nodes)-1]
			continue
		}
		if cur != nil {
			cur.conns = append(cur.conns, strings.TrimSpace(line))
		}
	}

	ports = len(nodes)
	// Count edges
	for _, n := range nodes {
		edges += len(n.conns)
	}

	// Heuristics for NAMNESIS:
	// - We want at least one capture->stompbox in
	// - and one stompbox out->playback
	hasIn := false
	hasOut := false

	for _, n := range nodes {
		p := n.name
		for _, c := range n.conns {
			// summary lines (bounded)
			if len(summary) < 80 {
				summary = append(summary, fmt.Sprintf("%s -> %s", p, c))
			}

			pl := strings.ToLower(p)
			cl := strings.ToLower(c)

			if strings.Contains(pl, "capture") && strings.Contains(cl, "stompbox") && strings.Contains(cl, "in") {
				hasIn = true
			}
			if strings.Contains(pl, "stompbox") && strings.Contains(pl, "out") && strings.Contains(cl, "playback") {
				hasOut = true
			}
		}
	}

	// Missing indicators (soft, not strict port names)
	if !hasIn {
		missing = append(missing, "capture -> stompbox:in (no connection detected)")
	}
	if !hasOut {
		missing = append(missing, "stompbox:out -> playback (no connection detected)")
	}

	ok = (len(missing) == 0)
	return ports, edges, summary, missing, ok, nil
}

func jackMidiPorts(ctx context.Context) ([]string, error) {
	out, err := run(ctx, 1200*time.Millisecond, "jack_lsp")
	if err != nil && isServiceActive(ctx, "jackd") {
		if uid, gid, e := jackdUIDGID(); e == nil {
			out2, err2 := runAsUIDGID(ctx, 1500*time.Millisecond, uid, gid, "jack_lsp")
			if err2 == nil {
				out = out2
				err = nil
			}
		}
	}
	if err != nil {
		return nil, err
	}

	lines := []string{}
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		ll := strings.ToLower(l)
		if strings.Contains(ll, "midi") || strings.Contains(ll, "a2j") {
			lines = append(lines, l)
		}
	}
	return lines, nil
}



func midiConnectionStatus(routingSummary []string) (bool, string) {
	// Look for a2j + sinco -> stompbox midi_in (best-effort; port naming can vary).
	// Example ports could be: a2j:SINCO ... , stompbox:midi_in
	for _, e := range routingSummary {
		low := strings.ToLower(e)
		if strings.Contains(low, "a2j") && strings.Contains(low, "sinco") && strings.Contains(low, "stompbox") && strings.Contains(low, "midi") {
			return true, e
		}
	}
	return false, "a2j:SINCO -> stompbox:midi_in not detected"
}

func readLoad1() (float64, error) {
	b, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(b))
	if len(fields) < 1 {
		return 0, errors.New("bad loadavg")
	}
	return strconv.ParseFloat(fields[0], 64)
}

func readGovernor() (string, error) {
	// Most systems expose cpu0 governor here
	p := "/sys/devices/system/cpu/cpu0/cpufreq/scaling_governor"
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func readTempC() (float64, error) {
	// Read max temperature among thermal zones
	base := "/sys/class/thermal"
	entries, err := os.ReadDir(base)
	if err != nil {
		return 0, err
	}

	maxC := -1.0
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "thermal_zone") {
			continue
		}
		p := filepath.Join(base, e.Name(), "temp")
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		raw := strings.TrimSpace(string(b))
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			continue
		}
		// Most report millidegrees
		if v > 1000 {
			v = v / 1000.0
		}
		if v > maxC {
			maxC = v
		}
	}
	if maxC < 0 {
		return 0, errors.New("no thermal zones")
	}
	return maxC, nil
}

func readMem() MemInfo {
	mi := MemInfo{}
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return mi
	}
	var memTotalKB, memAvailKB int
	sc := bufio.NewScanner(strings.NewReader(string(b)))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fmt.Sscanf(line, "MemTotal: %d kB", &memTotalKB)
		}
		if strings.HasPrefix(line, "MemAvailable:") {
			fmt.Sscanf(line, "MemAvailable: %d kB", &memAvailKB)
		}
	}
	if memTotalKB > 0 {
		mi.TotalMB = memTotalKB / 1024
	}
	if memTotalKB > 0 && memAvailKB > 0 {
		usedKB := memTotalKB - memAvailKB
		mi.UsedMB = usedKB / 1024
	}
	return mi
}

func readDisk(path string) DiskInfo {
	di := DiskInfo{}
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return di
	}
	// free bytes = bavail * bsize
	free := float64(st.Bavail) * float64(st.Bsize)
	di.RootFreeGB = free / (1024.0 * 1024.0 * 1024.0)
	return di
}

func readFileLines(path string, limit int) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := []string{}
	sc := bufio.NewScanner(strings.NewReader(string(b)))
	for sc.Scan() {
		if limit > 0 && len(lines) >= limit {
			break
		}
		lines = append(lines, sc.Text())
	}
	return lines, nil
}

func runLines(ctx context.Context, timeout time.Duration, cmd string, args ...string) ([]string, error) {
	out, err := run(ctx, timeout, cmd, args...)
	if err != nil {
		return nil, err
	}
	lines := []string{}
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines, nil
}

func run(ctx context.Context, timeout time.Duration, cmd string, args ...string) (string, error) {
	return runEnv(ctx, timeout, nil, cmd, args...)
}

func runEnv(ctx context.Context, timeout time.Duration, env []string, cmd string, args ...string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	c := exec.CommandContext(cctx, cmd, args...)
	if env != nil {
		c.Env = append(os.Environ(), env...)
	}

	b, err := c.CombinedOutput()
	if cctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("%s timeout", cmd)
	}
	if err != nil {
		// include stderr/stdout for debugging
		return "", fmt.Errorf("%s: %v: %s", cmd, err, strings.TrimSpace(string(b)))
	}
	return string(b), nil
}

func runAsUIDGID(ctx context.Context, timeout time.Duration, uid, gid int, cmd string, args ...string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	c := exec.CommandContext(cctx, cmd, args...)
	c.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: uint32(uid),
			Gid: uint32(gid),
		},
	}

	b, err := c.CombinedOutput()
	if cctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("%s timeout", cmd)
	}
	if err != nil {
		return "", fmt.Errorf("%s: %v: %s", cmd, err, strings.TrimSpace(string(b)))
	}
	return string(b), nil
}

func uidOf(user string) (int, error) {
	lines, err := readFileLines("/etc/passwd", 0)
	if err != nil {
		return 0, err
	}
	for _, l := range lines {
		if strings.HasPrefix(l, user+":") {
			parts := strings.Split(l, ":")
			if len(parts) >= 3 {
				return strconv.Atoi(parts[2])
			}
		}
	}
	return 0, fmt.Errorf("user not found: %s", user)
}

func jackEnvForUser(user string) []string {
	uid, err := uidOf(user)
	if err != nil {
		return nil
	}
	return []string{
		fmt.Sprintf("XDG_RUNTIME_DIR=/run/user/%d", uid),
		"HOME=/home/" + user,
		"USER=" + user,
		"LOGNAME=" + user,
	}
}


// Optional: JSON helper for report endpoints later
func (s Snapshot) PrettyJSON() string {
	b, _ := json.MarshalIndent(s, "", "  ")
	return string(b)
}

// Silence unused imports when building without some paths
var _ fs.FileInfo
