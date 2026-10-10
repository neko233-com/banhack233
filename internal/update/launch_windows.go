//go:build windows

package update

import (
	"os"
	"os/exec"
	"syscall"
)

// Launch runs an independent helper so Windows can replace the installed executable.
func Launch(opts Options) (bool, error) {
	if opts.Target != "" || (!opts.Apply && !opts.Rollback) {
		return false, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return false, err
	}
	path, err := windowsScript(exe, opts.Config)
	if err != nil {
		return false, err
	}
	args := []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-WindowStyle", "Hidden", "-File", path}
	if opts.Rollback {
		args = append(args, "-Rollback")
	}
	if !opts.Restart {
		args = append(args, "-NoRestart")
	}
	cmd := exec.Command("powershell.exe", args...)
	// CREATE_NO_WINDOW keeps PowerShell independent without invalidating its standard handles.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x00000200 | 0x08000000}
	log, err := os.OpenFile(exe+".update-launcher.log", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return false, err
	}
	defer log.Close()
	cmd.Stdout, cmd.Stderr = log, log
	if err = cmd.Start(); err != nil {
		return false, err
	}
	_ = cmd.Process.Release()
	return true, nil
}
