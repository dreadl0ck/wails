package application

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileDragOutPaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample #1 ü.wav")
	if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	paths, err := fileDragOutPaths([]string{path, dir, path})
	if err != nil || len(paths) != 2 || paths[0] != path || paths[1] != dir {
		t.Fatalf("paths=%v err=%v", paths, err)
	}
	for _, paths := range [][]string{nil, {"relative.wav"}, {path + "\x00suffix"}, {filepath.Join(dir, "missing")}} {
		if _, err := fileDragOutPaths(paths); err == nil {
			t.Fatalf("accepted invalid paths %q", paths)
		}
	}
}

func TestFileDragOutURIs(t *testing.T) {
	paths := []string{"/tmp/sample #1 ü.wav", "/tmp/line\nbreak.wav"}
	lines := strings.Split(strings.TrimSuffix(fileDragOutURIs(paths), "\r\n"), "\r\n")
	if len(lines) != len(paths) {
		t.Fatalf("URI list split into %d entries", len(lines))
	}
	for i, line := range lines {
		u, err := url.Parse(line)
		if err != nil || u.Scheme != "file" || u.Path != paths[i] || u.Fragment != "" {
			t.Fatalf("invalid file URI %q: %v", line, err)
		}
	}
}

func TestFileDragOutOptIn(t *testing.T) {
	w := &WebviewWindow{}
	if err := w.ArmFileDragOut(nil, ""); err == nil {
		t.Fatal("disabled window accepted a file drag")
	}
}
