// internal/sysinfo/sysinfo.go
package sysinfo

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

/*
UNIVERSAL SYSINFO (config-driven)

Key principles:
- Do not guess system intent; intent comes from config.
- Do not conflate "jack service active" with "JACK client tools can connect".
- Expose probe status explicitly so failures are diagnosable.
*/

type Snapshot struct {
	TS int64 `json:"ts"`

	Jack    JackInfo    `json:"jack"`
	Routing RoutingInfo `json:"routing"`
	MIDI    MidiInfo    `json:"midi"`
	AudioIF AudioIFInfo `json:"audioif"`
	CPU     CPUInfo     `json:"cpu"`
	Mem     MemInfo     `json:"mem"`
	Disk    DiskInfo    `json:"disk"`

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
	// probe_ok tells whether jack_lsp ran successfully (i.e. we could query JACK graph).
	ProbeOK    bool   `json:"probe_ok"`
	ProbeError string `json:"probe_error,omitempty"`

	// ok tells whether configured expectations are satisfied (only meaningful when ProbeOK=true).
	OK      bool     `json:"ok"`
	Missing []string `json:"missing,omitempty"`

	Ports   int      `json:"ports"`
	Edges   int      `json:"edges"`
	Summary []string `json:"summary,omitempty"`
}

type MidiInfo struct {
	ALSA []string `json:"alsa,omitempty"`

	// connected is computed from one (or both) of:
	// - ALSA listing match (midi.alsa_required_regex)
	// - Routing edge match (midi.connected_regex) when routing probe succeeded
	Connected bool   `json:"connected"`
	Details   string `json:"details,omitempty"`
}

type AudioIFInfo struct {
	AsoundCards []string `json:"asound_cards,omitempty"`
}

type CPUInfo struct {
	Load1    float64 `json:"load1"`
	Governor string  `json:"governor,omitempty"`
	TempC    float64 `json:"temp_c,omitempty"`
}

type MemInfo struct {
	TotalMB int `json:"total_mb"`
	UsedMB  int `json:"used_mb"`
}

type DiskInfo struct {
	RootFreeGB float64 `json:"root_free_gb"`
}

/* ---------------- Config ---------------- */

type Config struct {
	CacheTTLms int `json:"cache_ttl_ms"`

	Jack struct {
		Enabled     bool   `json:"enabled"`
		ServiceName string `json:"service_name"`
		UnitPath    string `json:"unit_path"`

		JournalUnit  string `json:"journal_unit"`
		JournalLines int    `json:"journal_lines"`
		XrunRegex    string `json:"xrun_regex"`
	} `json:"jack"`

	Routing struct {
		Enabled    bool              `json:"enabled"`
		JackLspCmd []string          `json:"jack_lsp_cmd"`
		Env        map[string]string `json:"env"` // env vars for jack_lsp (e.g. XDG_RUNTIME_DIR)

		Expect []struct {
			ID        string `json:"id"`
			FromRegex string `json:"from_regex"`
			ToRegex   string `json:"to_regex"`
			Message   string `json:"message"`
		} `json:"expect"`
	} `json:"routing"`

	Midi struct {
		Enabled     bool     `json:"enabled"`
		AconnectCmd []string `json:"aconnect_cmd"`

		// connected_regex is evaluated against routing summary edges (requires routing probe OK).
		ConnectedRegex string `json:"connected_regex"`

		// alsa_required_regex is evaluated against the full aconnect -l output (does NOT require routing).
		ALSARequiredRegex string `json:"alsa_required_regex"`
	} `json:"midi"`

	AudioIF struct {
		Enabled         bool   `json:"enabled"`
		AsoundCardsPath string `json:"asound_cards_path"`
	} `json:"audioif"`

	CPU struct {
		Enabled bool `json:"enabled"`
	} `json:"cpu"`
	Mem struct {
		Enabled bool `json:"enabled"`
	} `json:"mem"`
	Disk struct {
		Enabled bool   `json:"enabled"`
		Path    string `json:"path"`
	} `json:"disk"`
}

func defaultConfig() Config {
	var cfg Config
	cfg.CacheTTLms = 1500

	cfg.Jack.Enabled = true
	cfg.Jack.ServiceName = "jackd"
	cfg.Jack.UnitPath = "/etc/systemd/system/jackd.service"
	cfg.Jack.JournalUnit = "jackd"
	cfg.Jack.JournalLines = 250
	cfg.Jack.XrunRegex = `(?i)\bxrun\b`

	cfg.Routing.Enabled = true
	cfg.Routing.JackLspCmd = []string{"jack_lsp", "-c"}
	cfg.Routing.Env = map[string]string{}

	cfg.Midi.Enabled = true
	cfg.Midi.AconnectCmd = []string{"aconnect", "-l"}
	cfg.Midi.ConnectedRegex = `(?i)a2j:.*->.*stompbox:.*midi`
	cfg.Midi.ALSARequiredRegex = "" // optional

	cfg.AudioIF.Enabled = true
	cfg.AudioIF.AsoundCardsPath = "/proc/asound/cards"

	cfg.CPU.Enabled = true
	cfg.Mem.Enabled = true
	cfg.Disk.Enabled = true
	cfg.Disk.Path = "/"
	return cfg
}

func loadConfig() (Config, error) {
	cfg := defaultConfig()
	path := strings.TrimSpace(os.Getenv("SYSINFO_CONFIG"))
	if path == "" {
		path = "./sysinfo.json"
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("sysinfo config read (%s): %w", path, err)
	}

	var userCfg Config
	if err := json.Unmarshal(b, &userCfg); err != nil {
		return cfg, fmt.Errorf("sysinfo config json (%s): %w", path, err)
	}

	// Merge overrides conservatively.
	if userCfg.CacheTTLms != 0 {
		cfg.CacheTTLms = userCfg.CacheTTLms
	}
	if userCfg.Jack.ServiceName != "" || userCfg.Jack.UnitPath != "" || userCfg.Jack.JournalUnit != "" || userCfg.Jack.XrunRegex != "" || userCfg.Jack.JournalLines != 0 || userCfg.Jack.Enabled != cfg.Jack.Enabled {
		cfg.Jack = userCfg.Jack
	}
	if userCfg.Routing.JackLspCmd != nil || userCfg.Routing.Expect != nil || userCfg.Routing.Env != nil || userCfg.Routing.Enabled != cfg.Routing.Enabled {
		cfg.Routing = userCfg.Routing
		if cfg.Routing.Env == nil {
			cfg.Routing.Env = map[string]string{}
		}
	}
	if userCfg.Midi.AconnectCmd != nil || userCfg.Midi.ConnectedRegex != "" || userCfg.Midi.ALSARequiredRegex != "" || userCfg.Midi.Enabled != cfg.Midi.Enabled {
		cfg.Midi = userCfg.Midi
	}
	if userCfg.AudioIF.AsoundCardsPath != "" || userCfg.AudioIF.Enabled != cfg.AudioIF.Enabled {
		cfg.AudioIF = userCfg.AudioIF
	}
	cfg.CPU = userCfg.CPU
	cfg.Mem = userCfg.Mem
	cfg.Disk = userCfg.Disk
	if cfg.Disk.Path == "" {
		cfg.Disk.Path = "/"
	}

	return cfg, nil
}

/* ---------------- Collector ---------------- */

type Collector struct {
	mu sync.Mutex

	cache    Snapshot
	cacheAt  time.Time
	cacheTTL time.Duration

	lastXruns int

	cfg       Config
	cfgErrMsg string
}

func NewCollector() *Collector {
	cfg, err := loadConfig()
	c := &Collector{
		cfg: cfg,
	}
	if err != nil {
		c.cfgErrMsg = err.Error()
	}
	c.cacheTTL = time.Duration(cfg.CacheTTLms) * time.Millisecond
	return c
}

func (c *Collector) Snapshot(ctx context.Context) Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()

	// MOCK MODE for macOS dev: bypass all probing.
	if p := strings.TrimSpace(os.Getenv("SYSINFO_MOCK_FILE")); p != "" {
		s := snapshotFromMockFile(p)
		c.cache = s
		c.cacheAt = time.Now()
		return s
	}

	if !c.cacheAt.IsZero() && time.Since(c.cacheAt) < c.cacheTTL {
		return c.cache
	}

	snap := Snapshot{TS: time.Now().Unix()}
	if c.cfgErrMsg != "" {
		snap.Errors = append(snap.Errors, c.cfgErrMsg)
	}

	/* JACK */
	if c.cfg.Jack.Enabled {
		jackCfg, err := parseJackdUnit(c.cfg.Jack.UnitPath)
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
				snap.Jack.LatencyRTMs = (float64(snap.Jack.Buf*snap.Jack.Periods*2) / float64(snap.Jack.SR)) * 1000.0
			}
		}

		if c.cfg.Jack.ServiceName != "" {
			snap.Jack.Running = isServiceActive(ctx, c.cfg.Jack.ServiceName)
		}

		xruns, lastLine, xerr := countXruns(ctx, c.cfg.Jack.JournalUnit, c.cfg.Jack.JournalLines, c.cfg.Jack.XrunRegex)
		if xerr != nil {
			snap.Errors = append(snap.Errors, "xruns: "+xerr.Error())
		} else {
			snap.Jack.Xruns = xruns
			snap.Jack.LastXrun = lastLine
			snap.Jack.XrunsDelta = xruns - c.lastXruns
			c.lastXruns = xruns
		}
	}

	/* ROUTING */
	if c.cfg.Routing.Enabled {
		ports, edges, summary, missing, ok, probeErr := collectRouting(ctx, c.cfg.Routing.JackLspCmd, c.cfg.Routing.Env, c.cfg.Routing.Expect)

		if probeErr != nil {
			snap.Routing.ProbeOK = false
			snap.Routing.ProbeError = probeErr.Error()
			// Important: this is a probe failure, not "routing is wrong".
			// We keep OK=false because expectations cannot be evaluated without a graph.
			snap.Routing.OK = false
			snap.Errors = append(snap.Errors, "routing probe: "+probeErr.Error())
		} else {
			snap.Routing.ProbeOK = true
			snap.Routing.Ports = ports
			snap.Routing.Edges = edges
			snap.Routing.Summary = summary
			snap.Routing.Missing = missing
			snap.Routing.OK = ok
		}
	}

	/* MIDI */
	if c.cfg.Midi.Enabled {
		if len(c.cfg.Midi.AconnectCmd) > 0 {
			lines, err := runLines(ctx, 1200*time.Millisecond, c.cfg.Midi.AconnectCmd, nil)
			if err != nil {
				snap.Errors = append(snap.Errors, "midi alsa: "+err.Error())
			} else {
				snap.MIDI.ALSA = lines
			}
		}

		// 1) ALSA-based detection (independent of routing)
		alsaMatched := false
		if c.cfg.Midi.ALSARequiredRegex != "" && len(snap.MIDI.ALSA) > 0 {
			re, err := regexp.Compile(c.cfg.Midi.ALSARequiredRegex)
			if err != nil {
				snap.Errors = append(snap.Errors, "midi alsa_required_regex: "+err.Error())
			} else {
				all := strings.Join(snap.MIDI.ALSA, "\n")
				if re.MatchString(all) {
					alsaMatched = true
					snap.MIDI.Connected = true
					snap.MIDI.Details = "alsa: matched"
				}
			}
		}

		// 2) Routing-based detection (only if routing probe succeeded and summary exists)
		if !snap.MIDI.Connected && c.cfg.Midi.ConnectedRegex != "" && len(snap.Routing.Summary) > 0 {
			re, err := regexp.Compile(c.cfg.Midi.ConnectedRegex)
			if err != nil {
				snap.Errors = append(snap.Errors, "midi connected_regex: "+err.Error())
			} else {
				for _, e := range snap.Routing.Summary {
					if re.MatchString(e) {
						snap.MIDI.Connected = true
						snap.MIDI.Details = e
						break
					}
				}
				if !snap.MIDI.Connected && !alsaMatched {
					snap.MIDI.Details = "not detected"
				}
			}
		}

		// If neither method is configured, remain false but be explicit.
		if !snap.MIDI.Connected && c.cfg.Midi.ALSARequiredRegex == "" && c.cfg.Midi.ConnectedRegex == "" {
			snap.MIDI.Details = "no detection regex configured"
		}
	}

	/* AUDIO IF */
	if c.cfg.AudioIF.Enabled && c.cfg.AudioIF.AsoundCardsPath != "" {
		lines, err := readFileLines(c.cfg.AudioIF.AsoundCardsPath, 120)
		if err == nil {
			snap.AudioIF.AsoundCards = lines
		}
	}

	/* CPU/MEM/DISK */
	if c.cfg.CPU.Enabled {
		if v, err := readLoad1(); err == nil {
			snap.CPU.Load1 = v
		}
		if g, err := readGovernor(); err == nil {
			snap.CPU.Governor = g
		}
		if t, err := readTempC(); err == nil {
			snap.CPU.TempC = t
		}
	}
	if c.cfg.Mem.Enabled {
		snap.Mem = readMem()
	}
	if c.cfg.Disk.Enabled {
		snap.Disk = readDisk(c.cfg.Disk.Path)
	}

	c.cache = snap
	c.cacheAt = time.Now()
	return snap
}

/* ---------------- Mock ---------------- */

func snapshotFromMockFile(path string) Snapshot {
	now := time.Now().Unix()

	b, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{TS: now, Errors: []string{"sysinfo mock read: " + err.Error()}}
	}
	var s Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		return Snapshot{TS: now, Errors: []string{"sysinfo mock json: " + err.Error()}}
	}
	s.TS = now
	return s
}

/* ---------------- Jack unit parsing ---------------- */

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

	args := strings.Fields(execLine)

	cfg := jackUnitCfg{
		Driver: "alsa",
		RT:     contains(args, "-R"),
	}

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

/* ---------------- Probing helpers ---------------- */

func isServiceActive(ctx context.Context, name string) bool {
	out, err := run(ctx, 900*time.Millisecond, "systemctl", nil, name, "is-active")
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) == "active"
}

func countXruns(ctx context.Context, unit string, lines int, xrunRegex string) (count int, lastLine string, err error) {
	if unit == "" {
		return 0, "", nil
	}
	if lines <= 0 {
		lines = 250
	}
	if xrunRegex == "" {
		xrunRegex = `(?i)\bxrun\b`
	}
	re, err := regexp.Compile(xrunRegex)
	if err != nil {
		return 0, "", err
	}

	out, err := run(ctx, 1500*time.Millisecond, "journalctl", nil, "-u", unit, "-n", strconv.Itoa(lines), "--no-pager", "-o", "short-iso")
	if err != nil {
		return 0, "", err
	}

	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		if re.MatchString(line) {
			count++
			lastLine = line
		}
	}
	return count, lastLine, nil
}

func collectRouting(ctx context.Context, jackLspCmd []string, env map[string]string, expect []struct {
	ID        string `json:"id"`
	FromRegex string `json:"from_regex"`
	ToRegex   string `json:"to_regex"`
	Message   string `json:"message"`
}) (ports int, edges int, summary []string, missing []string, ok bool, err error) {
	if len(jackLspCmd) == 0 {
		jackLspCmd = []string{"jack_lsp", "-c"}
	}
	out, err := runCmd(ctx, 1200*time.Millisecond, jackLspCmd, env)
	if err != nil {
		return 0, 0, nil, nil, false, err
	}

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
	for _, n := range nodes {
		edges += len(n.conns)
	}

	for _, n := range nodes {
		p := n.name
		for _, c := range n.conns {
			if len(summary) < 200 {
				summary = append(summary, fmt.Sprintf("%s -> %s", p, c))
			}
		}
	}

	// Expectations are purely config-driven.
	matched := make(map[string]bool)
	for _, ex := range expect {
		fromRe, e1 := regexp.Compile(ex.FromRegex)
		toRe, e2 := regexp.Compile(ex.ToRegex)
		if e1 != nil || e2 != nil {
			if e1 != nil {
				missing = append(missing, "routing expect regex error: "+ex.ID+": "+e1.Error())
			}
			if e2 != nil {
				missing = append(missing, "routing expect regex error: "+ex.ID+": "+e2.Error())
			}
			continue
		}
		for _, edge := range summary {
			parts := strings.Split(edge, " -> ")
			if len(parts) != 2 {
				continue
			}
			if fromRe.MatchString(parts[0]) && toRe.MatchString(parts[1]) {
				matched[ex.ID] = true
				break
			}
		}
	}

	for _, ex := range expect {
		if ex.ID != "" && !matched[ex.ID] {
			msg := ex.Message
			if msg == "" {
				msg = "missing: " + ex.ID
			}
			missing = append(missing, msg)
		}
	}

	ok = (len(missing) == 0)
	return ports, edges, summary, missing, ok, nil
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
	p := "/sys/devices/system/cpu/cpu0/cpufreq/scaling_governor"
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func readTempC() (float64, error) {
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

func runLines(ctx context.Context, timeout time.Duration, cmd []string, env map[string]string) ([]string, error) {
	out, err := runCmd(ctx, timeout, cmd, env)
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

func run(ctx context.Context, timeout time.Duration, cmd string, env map[string]string, args ...string) (string, error) {
	full := append([]string{cmd}, args...)
	return runCmd(ctx, timeout, full, env)
}

func runCmd(ctx context.Context, timeout time.Duration, cmd []string, env map[string]string) (string, error) {
	if len(cmd) == 0 {
		return "", errors.New("empty command")
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	c := exec.CommandContext(cctx, cmd[0], cmd[1:]...)
	if env != nil && len(env) > 0 {
		merged := os.Environ()
		for k, v := range env {
			merged = append(merged, fmt.Sprintf("%s=%s", k, v))
		}
		c.Env = merged
	}

	b, err := c.CombinedOutput()
	if cctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("%s timeout", cmd[0])
	}
	if err != nil {
		return "", fmt.Errorf("%s: %v: %s", cmd[0], err, strings.TrimSpace(string(b)))
	}
	return string(b), nil
}
