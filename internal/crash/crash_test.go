package crash

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func unzip(t *testing.T, b []byte) string {
	t.Helper()
	r, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// TestPending: a crash's output, log tail and save become the pending
// report at the next start; its report holds them; Drop removes it. A run
// that did not crash leaves nothing pending.
func TestPending(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	prepare(dir)
	if Pending(dir) {
		t.Fatal("pending after a clean start")
	}
	var tail Tail
	for i := range tailLines + 5 {
		tail.Add("line " + strings.Repeat("x", i%3))
	}
	tail.Add("the last line\n")
	tail.Write(dir, []byte("SAVE"))
	os.WriteFile(filepath.Join(dir, OutFile), []byte("panic: boom\n"), 0o644)
	prepare(dir)
	if !Pending(dir) {
		t.Fatal("no report pending")
	}
	for _, f := range []string{OutFile, LogFile, SaveFile} {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			t.Errorf("%s left", f)
		}
	}
	r, err := Report(dir, "v1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	text := unzip(t, r.Crash)
	if !strings.HasPrefix(text, "panic: boom\n") || !strings.HasSuffix(text, "the last line\n") || strings.Count(text, "\n") != tailLines+3 {
		t.Errorf("report text %q", text)
	}
	if unzip(t, r.Save) != "SAVE" || r.Build != "v1.2.3" || r.Os == "" {
		t.Errorf("report %v", r)
	}
	// Not sent this time: a clean run keeps it pending.
	prepare(dir)
	if !Pending(dir) {
		t.Fatal("a clean start dropped the pending report")
	}
	Drop(dir)
	if Pending(dir) {
		t.Fatal("still pending after Drop")
	}
	if _, err := Report(dir, "v1"); err == nil {
		t.Fatal("a report without one pending")
	}
}
