//go:build !js

package main

import "github.com/openorion2/openorion2/internal/netplay"

// webNet is the browser build's; native builds use TCP directly.
func webNet(*netplay.TCP) {}
