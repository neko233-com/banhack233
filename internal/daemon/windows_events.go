//go:build windows

package daemon

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

func readWindowsEvents(source string, cursor int64, startAtEnd bool) ([]string, int64, error) {
	if source == "" {
		source = "OpenSSH/Operational"
	}
	query := func(args ...string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return exec.CommandContext(ctx, "wevtutil", append([]string{"qe", source, "/f:xml", "/e:Events"}, args...)...).Output()
	}
	latest, err := query("/c:1", "/rd:true")
	if err != nil {
		return nil, cursor, err
	}
	_, head, err := parseWindowsEvents(latest, 0)
	if err != nil {
		return nil, cursor, err
	}
	if startAtEnd {
		return nil, head, nil
	}
	if head < cursor {
		cursor = 0
	}
	data, err := query("/c:1024", "/rd:false", fmt.Sprintf("/q:*[System[EventRecordID > %d]]", cursor))
	if err != nil {
		return nil, cursor, err
	}
	return parseWindowsEvents(data, cursor)
}
