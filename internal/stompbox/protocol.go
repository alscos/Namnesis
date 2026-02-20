package stompbox

import (
	"errors"
	"strings"
)

func normalizeCmd(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if !strings.HasSuffix(cmd, "\r\n") {
		cmd += "\r\n"
	}
	return cmd
}

func isErrorLine(line string) bool {
	return strings.HasPrefix(line, "Error ")
}
func (c *Client) SendOk(cmd string) error {
	resp, err := c.SendCommand(normalizeCmd(cmd))
	if err != nil {
		return err
	}

	// defensive: stompbox usually ends with Ok, but catch protocol errors
	for _, line := range strings.Split(resp, "\n") {
		line = strings.TrimSpace(line)
		if isErrorLine(line) {
			return errors.New(line)
		}
	}

	return nil
}
func (c *Client) SendAndRead(cmd string) (string, error) {
	resp, err := c.SendCommand(normalizeCmd(cmd))
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(resp, "\n") {
		line = strings.TrimSpace(line)
		if isErrorLine(line) {
			return "", errors.New(line)
		}
	}

	return resp, nil
}
