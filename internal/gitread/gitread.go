// Package gitread reads files as they were at a git ref, without checking
// anything out. It is what lets cdclint compare a change against its base:
// the base's migrations and connector are read from the object store and
// parsed in memory, the way the working tree's are parsed from disk.
//
// PATHS ARE RELATIVE TO THE CURRENT DIRECTORY, the same way the paths on
// the command line are. git's own "<ref>:<path>" is relative to the
// repository root, which is wrong the moment cdclint runs from a
// subdirectory (a monorepo, the action's working-directory); "<ref>:./<path>"
// is the cwd-relative form and is used throughout. An absolute path is
// made cwd-relative first.
package gitread

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// File is one file's content at the ref, named by the path the caller
// used, so findings and the working tree agree on names.
type File struct {
	Path string
	Text string
}

// ErrNotInRef is returned by Show when the path did not exist at the ref.
var ErrNotInRef = errors.New("path does not exist at ref")

// Show returns the file at ref. A path that did not exist there (a file
// new in the change) is ErrNotInRef, told apart from every other failure
// by asking git first rather than by reading its error text.
func Show(ref, path string) (string, error) {
	spec, err := spec(ref, path)
	if err != nil {
		return "", err
	}
	if _, err := run("cat-file", "-e", spec); err != nil {
		return "", ErrNotInRef
	}
	return run("show", spec)
}

// Dir returns the *.sql files directly under dir at ref, sorted by path:
// the same selection postgres.ReadDir makes on disk, files only, no
// recursion. One git process serves the whole directory. A dir absent at
// the ref is an empty list.
func Dir(ref, dir string) ([]File, error) {
	spec, err := spec(ref, dir)
	if err != nil {
		return nil, err
	}
	if _, err := run("cat-file", "-e", spec); err != nil {
		return nil, nil
	}
	// git archive refuses a tree that does not contain the current
	// directory, so it is given the root-relative spec and run from the
	// repository's top level; the result is the same tree either way.
	top, err := run("rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	prefix, err := run("rev-parse", "--show-prefix")
	if err != nil {
		return nil, err
	}
	rel, _ := relative(dir)
	cmd := command("archive", "--format=tar", ref+":"+strings.TrimSpace(prefix)+rel)
	cmd.Dir = strings.TrimSpace(top)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git archive %s: %s", ref, strings.TrimSpace(stderr.String()))
	}
	out := stdout.String()
	var files []File
	tr := tar.NewReader(strings.NewReader(out))
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag != tar.TypeReg || strings.Contains(h.Name, "/") || !strings.HasSuffix(strings.ToLower(h.Name), ".sql") {
			continue
		}
		b, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		files = append(files, File{Path: filepath.Join(dir, h.Name), Text: string(b)})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

// Changed reports whether path differs between ref and the working tree,
// as git sees it: line-ending conversion and the like do not count, so a
// CRLF checkout of an unchanged file is unchanged.
func Changed(ref, path string) (bool, error) {
	rel, err := relative(path)
	if err != nil {
		return false, err
	}
	cmd := command("diff", "--quiet", ref, "--", rel)
	err = cmd.Run()
	if err == nil {
		return false, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return true, nil
	}
	return false, fmt.Errorf("git diff --quiet %s -- %s: %w", ref, rel, err)
}

// Resolve turns a ref into a full commit id, so findings can name it.
func Resolve(ref string) (string, error) {
	out, err := run("rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// spec builds the cwd-relative "<ref>:./<path>" form.
func spec(ref, path string) (string, error) {
	rel, err := relative(path)
	if err != nil {
		return "", err
	}
	return ref + ":./" + rel, nil
}

// relative makes path relative to the current directory, forward-slashed,
// which is how git wants it on every OS.
func relative(path string) (string, error) {
	if filepath.IsAbs(path) {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(wd, path)
		if err != nil || strings.HasPrefix(rel, "..") {
			return "", fmt.Errorf("%s is outside the current directory; run cdclint from the repository", path)
		}
		path = rel
	}
	return filepath.ToSlash(filepath.Clean(path)), nil
}

func command(args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	// git's messages are read nowhere, but a fixed locale keeps the
	// behaviour of every subprocess the same on every machine.
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	return cmd
}

func run(args ...string) (string, error) {
	cmd := command(args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
