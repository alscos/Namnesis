package stompbox

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

type Client struct {
	Addr        string
	DialTimeout time.Duration
	ReadTimeout time.Duration
	MaxBytes    int
}

func New(addr string) *Client {
	return &Client{
		Addr:        addr,
		DialTimeout: 2 * time.Second,
		ReadTimeout: 10 * time.Second,
		MaxBytes:    2_000_000,
	}
}
func (c *Client) LoadPreset(name string) error {
    // Whatever you already use for sending commands (WriteLine / Do / SendCommand)
    // The TCP line should be: LoadPreset <presetname>
    _, err := c.SendCommand("LoadPreset " + name)
    return err
}
func (c *Client) SavePreset(name string) error {
	_, err := c.SendCommand("SavePreset " + name)
	return err
}

// doUntil sends a single command (must include \r\n) and reads lines until stop(line,state) returns true.
// It refreshes read deadlines per read so large dumps don’t time out mid-stream.
func (c *Client) doUntil(command string, stop func(lineTrim string, st *termState) bool) (string, error) {
	conn, err := net.DialTimeout("tcp", c.Addr, c.DialTimeout)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte(command)); err != nil {
		return "", err
	}

	reader := bufio.NewReader(conn)
	var buf bytes.Buffer
	st := &termState{}

	for {
		_ = conn.SetReadDeadline(time.Now().Add(c.ReadTimeout))
		line, readErr := reader.ReadString('\n')

		// accept partial line on EOF/no newline
		if line != "" {
			buf.WriteString(line)
			if buf.Len() > c.MaxBytes {
				return buf.String(), errors.New("response exceeded max size")
			}
			trim := strings.TrimSpace(line)
			if trim != "" {
				st.lastLine = trim
			}
			if stop(trim, st) {
				return buf.String(), nil
			}
		}

		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			break
		}
	}

	return buf.String(), fmt.Errorf("incomplete response (last line=%q)", st.lastLine)
}

type termState struct {
	seenEndProgram bool
	seenEndConfig  bool
	seenOk         bool
	lastLine       string
}


// DumpConfig reads until:
//   EndConfig
//   Ok
func (c *Client) DumpConfig() (string, error) {
	return c.doUntil("Dump Config\r\n", func(line string, st *termState) bool {
		if line == "EndConfig" {
			st.seenEndConfig = true
			return false
		}
		if st.seenEndConfig && line == "Ok" {
			return true
		}
		return false
	})
}

// DumpProgram reads until:
//   EndProgram
//   Ok
func (c *Client) DumpProgram() (string, error) {
	return c.doUntil("Dump Program\r\n", func(line string, st *termState) bool {
		if line == "EndProgram" {
			st.seenEndProgram = true
			return false
		}
		if st.seenEndProgram && line == "Ok" {
			return true
		}
		return false
	})
}

func (c *Client) SendCommand(cmd string) (string, error) {
	// Ensure CRLF terminator (stompbox protocol expects \r\n)
	if !strings.HasSuffix(cmd, "\r\n") {
		cmd += "\r\n"
	}

	return c.doUntil(cmd, func(line string, st *termState) bool {
		// Stop when we get the Ok terminator
		if line == "Ok" {
			st.seenOk = true
			return true
		}
		return false
	})
}


// ListPresets reads until:
//   Ok
func (c *Client) ListPresets() (string, error) {
	return c.doUntil("List Presets\r\n", func(line string, st *termState) bool {
		return line == "Ok"
	})
}
func (c *Client) SetParam(plugin, param, value string) error {
	_, err := c.SendCommand("SetParam " + plugin + " " + param + " " + value)
	return err
}
