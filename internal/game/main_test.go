package game

import (
	"runtime"
)

// The main goroutine keeps the main thread, where macOS wants windows.
func init() { runtime.LockOSThread() }
