package scanner

import (
	"os"
	"path/filepath"
	"strings"
)

// RequestEntry represents a discovered request file.
type RequestEntry struct {
	Name     string
	Path     string
	Category string
}

// Scan walks the requests directory and returns all .curl files found.
func Scan(requestsDir string) ([]RequestEntry, error) {
	var entries []RequestEntry

	err := filepath.Walk(requestsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".curl") {
			return nil
		}

		rel, _ := filepath.Rel(requestsDir, path)
		name := strings.TrimSuffix(rel, ".curl")
		name = filepath.ToSlash(name)

		category := ""
		if dir := filepath.Dir(rel); dir != "." {
			category = filepath.ToSlash(dir)
		}

		entries = append(entries, RequestEntry{
			Name:     name,
			Path:     path,
			Category: category,
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	return entries, nil
}

// EnsureDirs creates the requests/, responses/, and history/ directories.
func EnsureDirs(base string) error {
	dirs := []string{
		filepath.Join(base, "requests"),
		filepath.Join(base, "responses"),
		filepath.Join(base, "history"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}
