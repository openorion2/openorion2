//go:build !windows && !js

package update

import (
	"os"
	"syscall"
)

// Restart runs the installed binary in place of this process, with the
// same arguments.
func Restart() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return syscall.Exec(exe, os.Args, os.Environ())
}
