//go:build js

package update

import "errors"

// Restart: the browser build reloads its page instead (it never updates).
func Restart() error { return errors.New("update: no restart in the browser") }
