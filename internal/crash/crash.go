// Package crash keeps what a crash of the game leaves for a report to the
// lobby (port-only, docs/lobby/client.md, "Crash reports"): the Go
// runtime's crash output, the game's last log lines and its state as a
// save, in the game's directory. At the next start they become the
// pending report, which the player may send or drop.
package crash

import (
	"bytes"
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/openorion2/openorion2/lobbynet/lobbyv1"
)

// The files, in the game's directory: what this run writes if it crashes,
// and the report pending from an earlier one.
const (
	OutFile   = "CRASH.TXT"    // the runtime's crash output (debug.SetCrashOutput)
	LogFile   = "CRASHLOG.TXT" // the game's last log lines, written on a panic
	SaveFile  = "CRASH.GAM"    // the state, written on a panic in a game
	PendText  = "CRASHREP.TXT" // a pending report: the output and the log
	PendSave  = "CRASHREP.GAM" // and its save
	maxPart   = 1 << 20        // bytes of each part taken into a report
	tailLines = 200
)

// Start turns what an earlier run's crash left in dir into the pending
// report, then sends this run's crash output to dir's OutFile. A
// directory it cannot write leaves crashes unrecorded.
func Start(dir string) {
	prepare(dir)
	f, err := os.OpenFile(filepath.Join(dir, OutFile), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	if debug.SetCrashOutput(f, debug.CrashOptions{}) != nil {
		f.Close()
	}
	// f stays open for the runtime: it writes there as the process dies.
}

// prepare is Start but for the runtime's output: an earlier crash's files
// become the pending report, the rest is cleared.
func prepare(dir string) {
	out := filepath.Join(dir, OutFile)
	if st, err := os.Stat(out); err == nil && st.Size() > 0 {
		text := readPart(out)
		if l := readPart(filepath.Join(dir, LogFile)); len(l) > 0 {
			text = append(append(text, "\n--- the game's last log lines ---\n"...), l...)
		}
		if os.WriteFile(filepath.Join(dir, PendText), text, 0o644) == nil {
			os.Remove(filepath.Join(dir, PendSave))
			os.Rename(filepath.Join(dir, SaveFile), filepath.Join(dir, PendSave))
		}
	}
	os.Remove(out)
	os.Remove(filepath.Join(dir, LogFile))
	os.Remove(filepath.Join(dir, SaveFile))
}

func readPart(path string) []byte {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return b[:min(len(b), maxPart)]
}

// Pending reports whether dir holds a report from an earlier crash.
func Pending(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, PendText))
	return err == nil && st.Size() > 0
}

// Drop removes the pending report.
func Drop(dir string) {
	os.Remove(filepath.Join(dir, PendText))
	os.Remove(filepath.Join(dir, PendSave))
}

// Report is the pending report as the lobby takes it; build is the
// game's version.
func Report(dir, build string) (*lobbyv1.CrashReport, error) {
	text := readPart(filepath.Join(dir, PendText))
	if len(text) == 0 {
		return nil, errors.New("crash: no report")
	}
	r := &lobbyv1.CrashReport{Build: build, Os: runtime.GOOS + "/" + runtime.GOARCH, Crash: gz(text)}
	if s := readPart(filepath.Join(dir, PendSave)); len(s) > 0 {
		r.Save = gz(s)
	}
	return r, nil
}

func gz(b []byte) []byte {
	var z bytes.Buffer
	w := gzip.NewWriter(&z)
	w.Write(b)
	w.Close()
	return z.Bytes()
}

// Tail keeps the last log lines of a run; safe from any goroutine.
type Tail struct {
	mu    sync.Mutex
	lines []string
}

// Add keeps line (the oldest goes past tailLines).
func (t *Tail) Add(line string) {
	t.mu.Lock()
	if len(t.lines) == tailLines {
		copy(t.lines, t.lines[1:])
		t.lines = t.lines[:tailLines-1]
	}
	t.lines = append(t.lines, strings.TrimRight(line, "\n"))
	t.mu.Unlock()
}

// Write saves what a panic leaves before the runtime prints it: the tail
// to LogFile and, if not nil, the state to SaveFile.
func (t *Tail) Write(dir string, save []byte) {
	t.mu.Lock()
	text := strings.Join(t.lines, "\n") + "\n"
	t.mu.Unlock()
	os.WriteFile(filepath.Join(dir, LogFile), []byte(text), 0o644)
	if save != nil {
		os.WriteFile(filepath.Join(dir, SaveFile), save, 0o644)
	}
}
