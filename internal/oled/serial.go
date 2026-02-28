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
			if num == "" && name == "" && amp == "" && fx == "" {
				continue
			}

			payload := formatOLEDLines(num, name, amp, fx)
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
	if n == o.last {
		return false
	}
	o.last = n
	return true
}

func (o *OLEDSerial) send(payload string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.port == nil {
		mode := &serial.Mode{BaudRate: o.baud}
		p, err := serial.Open(o.portName, mode)
		if err != nil {
			return err
		}
		// Make line settings deterministic even if udev didn't run.
		_ = exec.Command("/usr/bin/stty", "-F", o.portName, "115200", "-echo", "-icanon", "-hupcl").Run()
		o.port = p
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

func formatOLEDLines(num, name, amp, fx string) string {
	num = strings.TrimSpace(num)
	name = strings.TrimSpace(name)
	amp = strings.TrimSpace(amp)
	fx = strings.TrimSpace(fx)

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
	b.WriteByte('\n') // commit
	return b.String()
}
