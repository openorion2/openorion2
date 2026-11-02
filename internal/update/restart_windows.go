//go:build windows

package update

import "os"

// Restart starts the installed binary with the same arguments and ends
// this process (Windows has no exec).
func Restart() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	p, err := os.StartProcess(exe, os.Args, &os.ProcAttr{Files: []*os.File{os.Stdin, os.Stdout, os.Stderr}})
	if err != nil {
		return err
	}
	p.Release()
	os.Exit(0)
	return nil
}
