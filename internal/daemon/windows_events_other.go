//go:build !windows

package daemon

import "fmt"

func readWindowsEvents(source string, cursor int64, startAtEnd bool) ([]string, int64, error) {
	return nil, cursor, fmt.Errorf("eventlog sources require Windows: %s", source)
}
