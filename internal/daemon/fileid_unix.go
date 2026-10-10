//go:build !windows

package daemon

import (
	"fmt"
	"os"
	"syscall"
)

func fileID(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", nil
	}
	return fmt.Sprintf("%d:%d", stat.Dev, stat.Ino), nil
}
