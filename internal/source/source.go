// Package source holds what every migrations reader shares: a migration as
// a named text, and the directory listing every migration runner uses, all
// *.sql files in filename order.
package source

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// NamedFile is one migration's content, named by the path findings show.
type NamedFile struct {
	Path string
	Text string
}

// ReadDir returns every *.sql file in dir, sorted by name, with its text.
func ReadDir(dir string) ([]NamedFile, []string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".sql") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)
	var named []NamedFile
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, nil, err
		}
		named = append(named, NamedFile{Path: f, Text: string(b)})
	}
	return named, files, nil
}
