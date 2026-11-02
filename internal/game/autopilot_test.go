package game

import (
	"os"
)

// The autopilot's tests.

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
