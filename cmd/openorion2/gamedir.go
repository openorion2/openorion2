package main

import (
	"os"
	"path/filepath"
	"strings"
)

// gameDir is where the original's files are when -game does not say: the
// program's own directory (it can be put into the game's), the current
// one, or a directory named game in either; "game" when none has them.
func gameDir() string {
	var dirs []string
	if exe, err := os.Executable(); err == nil {
		if exe, err := filepath.EvalSymlinks(exe); err == nil {
			dirs = append(dirs, filepath.Dir(exe))
		}
	}
	if wd, err := os.Getwd(); err == nil {
		dirs = append(dirs, wd)
	}
	for _, sub := range []string{"", "game"} {
		for _, d := range dirs {
			if p := filepath.Join(d, sub); hasGameFiles(p) {
				return p
			}
		}
	}
	return "game"
}

// hasGameFiles: dir holds the original's FONTS.LBX (in any case).
func hasGameFiles(dir string) bool {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range ents {
		if strings.EqualFold(e.Name(), "FONTS.LBX") {
			return true
		}
	}
	return false
}
