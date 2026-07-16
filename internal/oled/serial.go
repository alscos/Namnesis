package oled

import (
	"bytes"
	"context"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"go.bug.st/serial"
)

// OLEDSerial bridges preset/human -> Arduino Nano over USB serial.
type OLEDSerial struct {
	mu sync.Mutex

	portName string
	baud     int

	port serial.Port
	last string // last committed payload (normalized)
}

// NewOLEDSerial creates the bridge. portName can be "/dev/ttyNAMNESIS_OLED" (recommended via udev).
func NewOLEDSerial(portName string, baud int) *OLEDSerial {
	if baud <= 0 {
		baud = 115200
	}
	return &OLEDSerial{
		portName: portName,
		baud:     baud,
	}
}

// Start polls dump() on an interval, humanizes it, and writes it to Arduino.
// dump should be something like: sb.DumpProgram
func (o *OLEDSerial) Start(ctx context.Context, dump func() (string, error), interval time.Duration) {
	if interval <= 0 {
		interval = 400 * time.Millisecond
	}

	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			o.Close()
			return
		case <-t.C:
			raw, err := dump()
			if err != nil {
				continue
			}

			num, name, amp, fx := HumanizeFromDumpProgram(raw)
			stomps := StompsFromDumpProgram(raw)
			if num == "" && name == "" && amp == "" && fx == "" && stomps == "" {
				continue
			}

			payload := formatOLEDLines(num, name, amp, fx, stomps)
			if !o.shouldSend(payload) {
				continue
			}

			if err := o.send(payload); err != nil {
				// Important: without this we don't see permission/open failures.
				log.Printf("oled: send failed (%s): %v", o.portName, err)
				o.dropPort()
			}
		}
	}
}

func (o *OLEDSerial) Close() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.port != nil {
		_ = o.port.Close()
		o.port = nil
	}
}

func (o *OLEDSerial) shouldSend(payload string) bool {
	n := normalizePayload(payload)
	o.mu.Lock()
	defer o.mu.Unlock()
	return n != o.last
}

func (o *OLEDSerial) markSent(payload string) {
	n := normalizePayload(payload)
	o.mu.Lock()
	defer o.mu.Unlock()
	o.last = n
}

func (o *OLEDSerial) send(payload string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	opened := false

	if o.port == nil {
		mode := &serial.Mode{BaudRate: o.baud}
		p, err := serial.Open(o.portName, mode)
		if err != nil {
			return err
		}
		// Make line settings deterministic even if udev didn't run.
		_ = exec.Command("/usr/bin/stty", "-F", o.portName, "115200", "-echo", "-icanon", "-hupcl").Run()
		o.port = p
		opened = true
	}

	// Opening the serial port resets many Arduino Nano boards.
	// Wait before the first write so the bootloader and splash screen finish.
	if opened {
		time.Sleep(3 * time.Second)
	}

	if !strings.HasSuffix(payload, "\n\n") {
		if strings.HasSuffix(payload, "\n") {
			payload += "\n"
		} else {
			payload += "\n\n"
		}
	}

	_, err := o.port.Write([]byte(payload))
	return err
}

func (o *OLEDSerial) dropPort() {
	if o.port != nil {
		_ = o.port.Close()
		o.port = nil
	}
}

func normalizePayload(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSpace(s)
	s = strings.Join(strings.Fields(s), " ")
	return s
}

func formatOLEDLines(num, name, amp, fx, stomps string) string {
	num = strings.TrimSpace(num)
	name = strings.TrimSpace(name)
	amp = strings.TrimSpace(amp)
	fx = strings.TrimSpace(fx)
	stomps = strings.TrimSpace(stomps)

	var b bytes.Buffer
	if num != "" {
		b.WriteString("NUM: ")
		b.WriteString(num)
		b.WriteByte('\n')
	}
	if name != "" {
		b.WriteString("NAME: ")
		b.WriteString(name)
		b.WriteByte('\n')
	}
	if amp != "" {
		b.WriteString("AMP: ")
		b.WriteString(amp)
		b.WriteByte('\n')
	}
	if fx != "" {
		b.WriteString("FX: ")
		b.WriteString(fx)
		b.WriteByte('\n')
	}
	if stomps != "" {
		b.WriteString("STOMPS: ")
		b.WriteString(stomps)
		b.WriteByte('\n')
	}
	b.WriteByte('\n') // commit
	return b.String()
}

// StompsFromDumpProgram returns a 12-character bitmap, ordered as:
//
//	1  Boost_2
//	2  Screamer_2
//	3  Delay_2
//	4  ConvoReverb_2
//
//	5  Compressor_2
//	6  NoiseGate_2
//	7  Level
//	8  Fuzz_2
//
//	9  Phase90Script
//	10 Chorus_2
//	11 Tremolo_2
//	12 Vibrato_2
//
// "1" means Enabled 1, "0" means Enabled 0.
// Returns "" if none of these plugins were found in Dump Program.
func StompsFromDumpProgram(raw string) string {
	order := []string{
		"Boost_2", "Screamer_2", "Delay_2", "ConvoReverb_2",
		"Compressor_2", "NoiseGate_2", "Level", "Fuzz_2",
		"Phase90Script", "Chorus_2", "Tremolo_2", "Vibrato_2",
	}

	wanted := make(map[string]bool, len(order))
	for _, name := range order {
		wanted[name] = true
	}

	enabled := make(map[string]bool, len(order))
	found := false

	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "SetParam ") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		plugin := fields[1]
		param := fields[2]
		value := fields[3]

		if param != "Enabled" || !wanted[plugin] {
			continue
		}

		found = true
		enabled[plugin] = value == "1" || strings.EqualFold(value, "true")
	}

	if !found {
		return ""
	}

	var b strings.Builder
	b.Grow(12)

	for _, name := range order {
		if enabled[name] {
			b.WriteByte('1')
		} else {
			b.WriteByte('0')
		}
	}

	return b.String()
}
