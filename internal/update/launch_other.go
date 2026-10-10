//go:build !windows

package update

func Launch(opts Options) (bool, error) { return false, nil }
