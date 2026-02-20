package stompbox

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type DumpConfigParsed struct {
	Plugins map[string]*PluginDef `json:"plugins"`
	Order   []string              `json:"order,omitempty"` // optional stable order if you want
}

type PluginDef struct {
	Name             string                  `json:"name"`
	BackgroundColor  string                  `json:"backgroundColor,omitempty"`
	ForegroundColor  string                  `json:"foregroundColor,omitempty"`
	IsUserSelectable *bool                   `json:"isUserSelectable,omitempty"`
	Description      string                  `json:"description,omitempty"`
	Params           map[string]*ParamDef    `json:"params,omitempty"`
	FileTrees        map[string]*FileTreeDef `json:"fileTrees,omitempty"`
}

type ParamDef struct {
	Plugin           string            `json:"plugin"`
	Name             string            `json:"name"`
	Type             string            `json:"type,omitempty"`
	MinValue         *float64          `json:"minValue,omitempty"`
	MaxValue         *float64          `json:"maxValue,omitempty"`
	DefaultValue     *float64          `json:"defaultValue,omitempty"`
	RangePower       *float64          `json:"rangePower,omitempty"`
	ValueFormat      string            `json:"valueFormat,omitempty"`
	CanSyncToHostBPM *bool             `json:"canSyncToHostBPM,omitempty"`
	IsAdvanced       *bool             `json:"isAdvanced,omitempty"`
	IsOutput         *bool             `json:"isOutput,omitempty"`
	Description      string            `json:"description,omitempty"`
	RawKV            map[string]string `json:"rawKV,omitempty"` // keeps unknown keys without losing info
}

func ParseDumpConfig(raw string) (*DumpConfigParsed, error) {
	out := &DumpConfigParsed{
		Plugins: make(map[string]*PluginDef),
	}

	var currentPlugin string // used for recovery when lines omit the plugin name (NAMMulti case)

	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == "Ok" {
			break
		}

		toks := splitQuoted(line)
		if len(toks) == 0 {
			continue
		}

		switch toks[0] {

		case "PluginConfig":
			// PluginConfig <Plugin> BackgroundColor #... ForegroundColor #... IsUserSelectable 1 Description "..."
			if len(toks) < 2 {
				// malformed, ignore
				continue
			}
			pname := toks[1]
			currentPlugin = pname
			p := ensurePlugin(out, pname)
			applyPluginKV(p, toks[2:])

		case "ParameterConfig":
			// ParameterConfig <Plugin> <Param> Type Knob MinValue ... Description "..."
			//
			// BUT: you have malformed lines like:
			// ParameterConfig  Gain Type Knob ...
			// (plugin omitted) -> recover using currentPlugin, and treat first token after ParameterConfig as Param
			if len(toks) < 3 {
				// too short to be meaningful
				continue
			}

			var pname, param string
			startKV := 0

			// Normal case: ParameterConfig <Plugin> <Param> ...
			if toks[1] != "" && toks[2] != "" && toks[1] != "Type" {
				pname = toks[1]
				param = toks[2]
				startKV = 3
			} else {
				// Recovery case: assume toks[1] is param and plugin is currentPlugin
				pname = currentPlugin
				param = toks[1]
				startKV = 2
			}

			if pname == "" || param == "" {
				// can't safely attach
				continue
			}

			p := ensurePlugin(out, pname)
			if p.Params == nil {
				p.Params = make(map[string]*ParamDef)
			}
			def := &ParamDef{
				Plugin: pname,
				Name:   param,
				RawKV:  make(map[string]string),
			}
			applyParamKV(def, toks[startKV:])
			p.Params[param] = def

		case "ParameterFileTree":
			if len(toks) < 4 {
				continue
			}
			pname := toks[1]
			param := toks[2]
			category := toks[3]
			currentPlugin = pname

			p := ensurePlugin(out, pname)
			if p.FileTrees == nil {
				p.FileTrees = make(map[string]*FileTreeDef)
			}

			items := []string{}
			if len(toks) > 4 {
				items = toks[4:]
			}

			tree := &FileTreeDef{
				Plugin:   pname,
				Param:    param,
				Category: category,
				Items:    items,
				Options:  fileOptionsFromItems(items),
			}
			p.FileTrees[param] = tree

		case "EndConfig":
			// end of plugin block (we keep currentPlugin as last plugin for recovery)
			continue

		default:
			// ignore unknown lines
			continue
		}
	}

	return out, nil
}

func ensurePlugin(out *DumpConfigParsed, name string) *PluginDef {
	p, ok := out.Plugins[name]
	if !ok {
		p = &PluginDef{
			Name: name,
		}
		out.Plugins[name] = p
		out.Order = append(out.Order, name)
	}
	return p
}
func fileOptionsFromItems(items []string) []FileOption {
	opts := make([]FileOption, 0, len(items))
	for _, it := range items {
		opts = append(opts, FileOption{
			Label: it,
			Value: it,
		})
	}
	return opts
}

func applyPluginKV(p *PluginDef, kv []string) {
	for i := 0; i < len(kv); i++ {
		k := kv[i]
		if i+1 >= len(kv) {
			break
		}
		v := kv[i+1]

		switch k {
		case "BackgroundColor":
			p.BackgroundColor = v
			i++
		case "ForegroundColor":
			p.ForegroundColor = v
			i++
		case "IsUserSelectable":
			b := parseBool01(v)
			p.IsUserSelectable = &b
			i++
		case "Description":
			p.Description = v
			i++
		default:
			// unknown plugin-level keys are ignored for now
			i++
		}
	}
}

func applyParamKV(p *ParamDef, kv []string) {
	for i := 0; i < len(kv); i++ {
		k := kv[i]
		if i+1 >= len(kv) {
			break
		}
		v := kv[i+1]

		switch k {
		case "Type":
			p.Type = v
			i++
		case "MinValue":
			p.MinValue = parseFloatPtr(v)
			i++
		case "MaxValue":
			p.MaxValue = parseFloatPtr(v)
			i++
		case "DefaultValue":
			p.DefaultValue = parseFloatPtr(v)
			i++
		case "RangePower":
			p.RangePower = parseFloatPtr(v)
			i++
		case "ValueFormat":
			p.ValueFormat = v
			i++
		case "CanSyncToHostBPM":
			b := parseBool01(v)
			p.CanSyncToHostBPM = &b
			i++
		case "IsAdvanced":
			b := parseBool01(v)
			p.IsAdvanced = &b
			i++
		case "IsOutput":
			b := parseBool01(v)
			p.IsOutput = &b
			i++
		case "Description":
			p.Description = v
			i++
		default:
			// preserve unhandled keys for future UI/debug
			if p.RawKV == nil {
				p.RawKV = make(map[string]string)
			}
			p.RawKV[k] = v
			i++
		}
	}
}

func parseFloatPtr(s string) *float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &f
}

func parseBool01(s string) bool {
	switch s {
	case "1", "true", "TRUE", "True":
		return true
	default:
		return false
	}
}

// splitQuoted splits a line into tokens while preserving quoted strings (without quotes).
// Example: Description "Clean boost effect" -> ["Description", "Clean boost effect"]
func splitQuoted(s string) []string {
	var out []string
	var cur strings.Builder

	inQuote := false
	escaped := false

	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}

	for _, r := range s {
		if escaped {
			cur.WriteRune(r)
			escaped = false
			continue
		}

		if r == '\\' && inQuote {
			escaped = true
			continue
		}

		if r == '"' {
			if inQuote {
				// closing quote
				inQuote = false
				flush()
			} else {
				// opening quote; flush token built so far
				flush()
				inQuote = true
			}
			continue
		}

		if !inQuote && unicode.IsSpace(r) {
			flush()
			continue
		}

		cur.WriteRune(r)
	}
	flush()

	// normalize: remove any empty tokens
	n := out[:0]
	for _, t := range out {
		if strings.TrimSpace(t) != "" {
			n = append(n, t)
		}
	}
	return n
}

func (p *ParamDef) Validate() error {
	if p.Plugin == "" || p.Name == "" {
		return fmt.Errorf("param missing plugin/name")
	}
	return nil
}
