package application

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type fileDragOutState struct {
	paths     []string
	imagePath string
	timer     uintptr
	pressed   bool
	x, y      int
	active    bool
	cancelled bool
	ending    bool
	performed bool
}

func fileDragOutPaths(paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("file drag-out requires at least one path")
	}
	result := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		if !filepath.IsAbs(path) || strings.ContainsRune(path, 0) {
			return nil, fmt.Errorf("file drag-out requires absolute file paths")
		}
		path = filepath.Clean(path)
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("file drag-out: %w", err)
		}
		if !seen[path] {
			result = append(result, path)
			seen[path] = true
		}
	}
	return result, nil
}

func fileDragOutURIs(paths []string) string {
	var uris strings.Builder
	for _, path := range paths {
		uris.WriteString((&url.URL{Scheme: "file", Path: path}).String())
		uris.WriteString("\r\n")
	}
	return uris.String()
}

func (w *WebviewWindow) fileDragOutEnded(performed bool) {
	w.ExecJS(fmt.Sprintf("window._wails?.handleFileDragOutEnded?.(%t)", performed))
}
