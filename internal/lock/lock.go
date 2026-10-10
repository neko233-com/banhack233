// Package lock serializes state writers and update installers across processes.
package lock

import (
	"fmt"
	"github.com/gofrs/flock"
	"os"
	"path/filepath"
)

func Acquire(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	f := flock.New(path, flock.SetPermissions(0600))
	ok, err := f.TryLock()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if !ok {
		_ = f.Close()
		return nil, fmt.Errorf("another instance holds %s", path)
	}
	return func() { _ = f.Close() }, nil
}
