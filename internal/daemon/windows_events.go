//go:build windows

package daemon

import (
	"os/exec"
	"strings"
)

func readWindowsEvents(source string) ([]string, error) {
	if source == "" {
		source = "OpenSSH/Operational"
	}
	out, err := exec.Command("wevtutil", "qe", source, "/c:80", "/rd:true", "/f:text").Output()
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, nil
}
