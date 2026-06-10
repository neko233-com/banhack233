//go:build !windows

package daemon

func readWindowsEvents(source string) ([]string, error) {
	return nil, nil
}
