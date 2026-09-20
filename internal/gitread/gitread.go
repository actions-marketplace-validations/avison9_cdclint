// Package gitread reads files as they were at a git ref, without checking
// anything out. It is what lets cdclint compare the pull request's working
// tree against its base: the base's migrations, connector and sink DDL are
// read straight from the object store and parsed in memory.
package gitread

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// File is one file's content at the ref, named by its repository path.
type File struct {
	Path string
	Text string
}

// Show returns the file at ref, or ok=false when the path did not exist
// there (a new file in the diff).
func Show(ref, path string) (text string, ok bool, err error) {
	out, err := run("show", ref+":"+toSlash(path))
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "exists on disk, but not in") {
			return "", false, nil
		}
		return "", false, err
	}
	return out, true, nil
}

// Dir returns every *.sql file under dir at ref, sorted by path. A dir that
// did not exist at the ref is an empty list, not an error.
func Dir(ref, dir string) ([]File, error) {
	out, err := run("ls-tree", "-r", "--name-only", ref, "--", toSlash(dir))
	if err != nil {
		if strings.Contains(err.Error(), "Not a valid object name") {
			return nil, err
		}
		return nil, err
	}
	var files []File
	for _, p := range strings.Split(strings.TrimSpace(out), "\n") {
		if p == "" || !strings.HasSuffix(strings.ToLower(p), ".sql") {
			continue
		}
		text, ok, err := Show(ref, p)
		if err != nil {
			return nil, err
		}
		if ok {
			files = append(files, File{Path: p, Text: text})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

// Resolve turns a ref into a full commit id, so findings can name it.
func Resolve(ref string) (string, error) {
	out, err := run("rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// toSlash makes a path git understands on every OS; git paths are always
// forward-slashed and relative to the repository root.
func toSlash(p string) string {
	return filepath.ToSlash(filepath.Clean(p))
}
