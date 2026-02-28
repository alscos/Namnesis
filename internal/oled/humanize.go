package oled

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

// -------------------------
// DumpProgram parsing + humanizing
// -------------------------

// HumanizeFromDumpProgram parses stompbox DumpProgram raw text and returns OLED-friendly fields:
// num, name, amp, fx.
func HumanizeFromDumpProgram(raw string) (num, name, amp, fx string) {
	enabled := map[string]bool{} // per-block enabled state
	var namModelRaw, cabImpulseRaw string

	lines := strings.Split(raw, "\n")
	for _, ln := range lines {
		line := strings.TrimSpace(ln)
		if line == "" {
			continue
		}

		// Preset line
		if strings.HasPrefix(line, "SetPreset ") {
			token := strings.TrimSpace(strings.TrimPrefix(line, "SetPreset "))
			num, name = splitPresetNumName(token)
			continue
		}

		// Params
		if strings.HasPrefix(line, "SetParam ") {
			fields := strings.Fields(line)
			if len(fields) < 4 {
				continue
			}
			blk := fields[1]
			key := strings.ToLower(fields[2])

			// Enabled
			if key == "enabled" {
				v := strings.ToLower(fields[3])
				enabled[blk] = (v == "1" || v == "true" || v == "on")
				continue
			}

			// NAM model
			if blk == "NAM" && key == "model" && namModelRaw == "" {
				if q := extractQuoted(line); q != "" {
					namModelRaw = q
				} else {
					namModelRaw = strings.Trim(strings.Join(fields[3:], " "), "\"")
				}
				continue
			}

			// Cabinet impulse
			if strings.EqualFold(blk, "Cabinet") && key == "impulse" && cabImpulseRaw == "" {
				if q := extractQuoted(line); q != "" {
					cabImpulseRaw = q
				} else {
					cabImpulseRaw = strings.Trim(strings.Join(fields[3:], " "), "\"")
				}
				continue
			}
		}
	}

	// ---- AMP + CAB (OLED-safe) ----
	ampShort := humanizeAmpModel(namModelRaw)
	cabShort := humanizeCabImpulse(cabImpulseRaw)

	if ampShort != "" && cabShort != "" {
		amp = fmt.Sprintf("%s | %s", ampShort, cabShort)
	} else if ampShort != "" {
		amp = ampShort
	} else if cabShort != "" {
		amp = cabShort
	} else {
		amp = ""
	}

	// ---- FX tokens (fixed order) ----
	// Canonical order: GATE COMP BST OD FUZZ WAH MOD DLY REV NAM CAB EQ
	var tokens []string

	// Helper: enabled by base-name (strip "_2", "_3"...)
	isEnabledBase := func(base string) bool {
		base = strings.TrimSpace(base)
		if base == "" {
			return false
		}
		for k, v := range enabled {
			if !v {
				continue
			}
			if strings.EqualFold(stripInstanceSuffix(k), base) {
				return true
			}
		}
		return false
	}

	// Group detectors
	hasGate := isEnabledBase("NoiseGate") || isEnabledBase("SimpleGate")
	hasComp := isEnabledBase("Compressor")
	hasBoost := isEnabledBase("Boost")
	hasOD := isEnabledBase("Screamer") || isEnabledBase("Overdrive") || isEnabledBase("OD")
	hasFuzz := isEnabledBase("Fuzz")
	hasWah := isEnabledBase("Wah") || isEnabledBase("AutoWah")
	hasMod := isEnabledBase("Chorus") || isEnabledBase("Flanger") || isEnabledBase("Phaser") ||
		isEnabledBase("Vibrato") || isEnabledBase("Tremolo")
	hasDly := isEnabledBase("Delay")
	hasRev := isEnabledBase("Reverb") || isEnabledBase("ConvoReverb")
	hasEQ := isEnabledBase("EQ-7") || isEnabledBase("BEQ-7") || isEnabledBase("HighLow") || isEnabledBase("EQ")

	// NAM/CAB are special: they appear as "NAM" and "Cabinet"
	hasNAM := enabled["NAM"]
	hasCAB := enabled["Cabinet"]

	if hasGate {
		tokens = append(tokens, "GATE")
	}
	if hasComp {
		tokens = append(tokens, "COMP")
	}
	if hasBoost {
		tokens = append(tokens, "BST")
	}
	if hasOD {
		tokens = append(tokens, "OD")
	}
	if hasFuzz {
		tokens = append(tokens, "FUZZ")
	}
	if hasWah {
		tokens = append(tokens, "WAH")
	}
	if hasMod {
		tokens = append(tokens, "MOD")
	}
	if hasDly {
		tokens = append(tokens, "DLY")
	}
	if hasRev {
		tokens = append(tokens, "REV")
	}
	if hasNAM {
		tokens = append(tokens, "NAM")
	}
	if hasCAB {
		tokens = append(tokens, "CAB")
	}
	if hasEQ {
		tokens = append(tokens, "EQ")
	}

	fx = strings.Join(tokens, " ")

	return strings.TrimSpace(num), strings.TrimSpace(name), strings.TrimSpace(amp), strings.TrimSpace(fx)
}

// -------------------------
// AMP humanizing
// -------------------------

func humanizeAmpModel(model string) string {
	m := strings.TrimSpace(model)
	if m == "" {
		return ""
	}

	base := path.Base(m)
	base = strings.TrimSpace(base)
	base = strings.Trim(base, "\"")

	low := strings.ToLower(base)

	mode := ""
	if strings.Contains(low, "clean") || strings.Contains(low, "crystal") {
		mode = "CLEAN"
	} else if strings.Contains(low, "crunch") {
		mode = "CRUNCH"
	} else if strings.Contains(low, "lead") {
		mode = "LEAD"
	}

	alias := ""
	switch {
	case strings.Contains(low, "vox") && strings.Contains(low, "ac15"):
		alias = "AC15"
	case strings.Contains(low, "vox") && strings.Contains(low, "ac30"):
		alias = "AC30"
	case strings.Contains(low, "jcm800") || strings.Contains(low, "2203"):
		alias = "JCM800"
	case strings.Contains(low, "jc-120") || strings.Contains(low, "jc120") || strings.Contains(low, "jazz chorus"):
		alias = "JC120"
	case strings.Contains(low, "jp2c") || strings.Contains(low, "mark v") || strings.Contains(low, "markv"):
		alias = "JP2C"
	case strings.Contains(low, "bassman") && (strings.Contains(low, "50") || strings.Contains(low, "bman")):
		alias = "BMAN50"
	case strings.Contains(low, "bassman"):
		alias = "BMAN"
	default:
		alias = shortFromWords(base, 2, 12)
	}

	if mode != "" {
		return strings.TrimSpace(alias + " " + mode)
	}
	return strings.TrimSpace(alias)
}

func shortFromWords(s string, n int, maxLen int) string {
	t := strings.TrimSpace(s)
	t = strings.ReplaceAll(t, "_", " ")
	t = strings.ReplaceAll(t, "-", " ")
	parts := strings.Fields(t)
	if len(parts) == 0 {
		return ""
	}
	if n <= 0 {
		n = 1
	}
	if len(parts) < n {
		n = len(parts)
	}
	out := strings.Join(parts[:n], " ")
	out = strings.TrimSpace(out)
	if maxLen > 0 && len(out) > maxLen {
		out = out[:maxLen]
		out = strings.TrimSpace(out)
	}
	return out
}

// -------------------------
// CAB humanizing
// -------------------------

var reCabSize = regexp.MustCompile(`^\d+x\d+$`) // e.g. 2x12, 4x10

func humanizeCabImpulse(imp string) string {
	s := strings.TrimSpace(imp)
	if s == "" {
		return ""
	}

	base := path.Base(s)
	base = strings.Trim(base, "\"")
	low := strings.ToLower(base)
	for _, ext := range []string{".wav", ".aiff", ".flac"} {
		if strings.HasSuffix(low, ext) {
			base = base[:len(base)-len(ext)]
			break
		}
	}

	norm := base
	norm = strings.ReplaceAll(norm, " ", "_")
	norm = strings.ReplaceAll(norm, "-", "_")
	norm = strings.ReplaceAll(norm, "__", "_")

	parts := strings.Split(norm, "_")
	parts = filterParts(parts, func(p string) bool {
		pl := strings.ToLower(p)
		if pl == "" {
			return false
		}
		switch pl {
		case "yrk", "audio", "yrk_audio", "york", "yorka", "yorkaudio", "york_audio":
			return false
		}
		if strings.HasPrefix(pl, "mix") {
			return false
		}
		return true
	})

	size := ""
	speaker := ""

	for _, p := range parts {
		pl := strings.ToLower(p)
		if reCabSize.MatchString(pl) {
			size = strings.ToUpper(pl)
			break
		}
	}

	known := []string{"blue", "v30", "greenback", "creamback", "alnico", "g12", "t75", "h30", "m25"}
	for _, p := range parts {
		pl := strings.ToLower(p)
		for _, k := range known {
			if pl == k || strings.Contains(pl, k) {
				speaker = strings.ToUpper(k)
				if speaker == "GREENBACK" {
					speaker = "GB"
				}
				if speaker == "CREAMBACK" {
					speaker = "CB"
				}
				break
			}
		}
		if speaker != "" {
			break
		}
	}

	if speaker == "" {
		for _, p := range parts {
			pl := strings.ToLower(p)
			if pl == "" || reCabSize.MatchString(pl) {
				continue
			}
			if strings.Contains(pl, "vox") || strings.Contains(pl, "ac15") || strings.Contains(pl, "ac30") ||
				strings.Contains(pl, "deluxe") || strings.Contains(pl, "verb") || strings.Contains(pl, "bassman") {
				continue
			}
			if pl == "impulse" || pl == "ir" || pl == "cab" {
				continue
			}
			speaker = strings.ToUpper(p)
			break
		}
	}

	if size != "" && speaker != "" {
		return strings.TrimSpace(size + " " + speaker)
	}
	if size != "" {
		return strings.TrimSpace(size)
	}
	if speaker != "" {
		return strings.TrimSpace(speaker)
	}

	return shortFromWords(base, 2, 16)
}

func filterParts(in []string, keep func(string) bool) []string {
	out := make([]string, 0, len(in))
	for _, p := range in {
		p = strings.TrimSpace(p)
		if keep(p) {
			out = append(out, p)
		}
	}
	return out
}

// -------------------------
// Small helpers
// -------------------------

var rePresetNumName = regexp.MustCompile(`^\s*(\d+)\s*[_\-\s]+\s*(.+?)\s*$`)

func splitPresetNumName(token string) (num, name string) {
	t := strings.TrimSpace(token)
	t = strings.Trim(t, "\"")
	if t == "" {
		return "", ""
	}

	if m := rePresetNumName.FindStringSubmatch(t); len(m) == 3 {
		num = strings.TrimSpace(m[1])
		name = strings.TrimSpace(m[2])
	} else {
		name = t
	}

	name = strings.ReplaceAll(name, "_", " ")
	name = strings.TrimSpace(name)
	return num, name
}

func extractQuoted(line string) string {
	i := strings.IndexByte(line, '"')
	if i < 0 {
		return ""
	}
	j := strings.LastIndexByte(line, '"')
	if j <= i {
		return ""
	}
	return strings.TrimSpace(line[i+1 : j])
}

func stripInstanceSuffix(block string) string {
	b := strings.TrimSpace(block)
	if b == "" {
		return ""
	}
	if strings.Contains(b, "_") {
		parts := strings.Split(b, "_")
		if len(parts) >= 2 {
			last := parts[len(parts)-1]
			if isAllDigits(last) {
				return strings.Join(parts[:len(parts)-1], "_")
			}
		}
	}
	return b
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
