package stompbox

import (
	"fmt"
	"strings"
)

type Program struct {
	ActivePreset string
	Chains       map[string][]string // ChainName -> ordered plugin instance names
	Slots        map[string]string   // SlotName -> plugin instance name
	Params       map[string]map[string]string // PluginName -> ParamName -> Value
}

func ParseDumpProgram(raw string) (*Program, error) {
	p := &Program{
		Chains: make(map[string][]string),
		Slots:  make(map[string]string),
		Params: make(map[string]map[string]string),
	}

	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// ignore terminators / ok
		if line == "EndProgram" || line == "Ok" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		switch fields[0] {
		case "SetPreset":
			// Allow empty preset name: some dumps emit "SetPreset" alone.
			if len(fields) < 2 {
				// Keep ActivePreset as-is (empty) and continue parsing.
				continue
			}
			p.ActivePreset = strings.Join(fields[1:], " ")


		case "SetChain":
			// SetChain <ChainName> <Plugin1> <Plugin2> ...
			if len(fields) < 2 {
				return nil, fmt.Errorf("malformed SetChain: %q", line)
			}
			chainName := fields[1]
			var plugins []string
			if len(fields) > 2 {
				plugins = fields[2:]
			} else {
				plugins = []string{}
			}
			p.Chains[chainName] = plugins

		case "SetPluginSlot":
			// SetPluginSlot <SlotName> <PluginName>
			if len(fields) < 3 {
				return nil, fmt.Errorf("malformed SetPluginSlot: %q", line)
			}
			slotName := fields[1]
			pluginName := fields[2]
			p.Slots[slotName] = pluginName

		case "SetParam":
			// SetParam <PluginName> <ParamName> <Value...>
			// Value may be omitted; treat as empty string.
			if len(fields) < 3 {
				return nil, fmt.Errorf("malformed SetParam: %q", line)
			}
			pluginName := fields[1]
			paramName := fields[2]

			value := ""
			if len(fields) >= 4 {
				value = strings.Join(fields[3:], " ")
			}


			if _, ok := p.Params[pluginName]; !ok {
				p.Params[pluginName] = make(map[string]string)
			}
			p.Params[pluginName][paramName] = value

		default:
			// ignore other lines for now
			continue
		}
	}

	return p, nil
}

