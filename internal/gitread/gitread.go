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
	"path"
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

// Exists reports whether path is present at ref. Errors are git failing,
// never a missing path. rev-parse --verify --quiet is the one form that
// answers with an exit code, 1 for "cannot resolve", where cat-file -e
// says "fatal" and exits 128 for a missing path the same as for a broken
// repository. The ref itself is checked by Resolve before this is asked,
// so 1 here means the path.
func Exists(ref, p string) (bool, error) {
	spec, err := spec(ref, p)
	if err != nil {
		return false, err
	}
	cmd := command("rev-parse", "--verify", "--quiet", spec)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err == nil {
		return true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("git rev-parse --verify %s: %s", spec, strings.TrimSpace(stderr.String()))
}

// Show returns the file at ref. A path that did not exist there (a file
// new in the change) is ErrNotInRef, told apart from every other failure
// by asking git first rather than by reading its error text.
func Show(ref, p string) (string, error) {
	ok, err := Exists(ref, p)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrNotInRef
	}
	spec, _ := spec(ref, p)
	return run("show", spec)
}

// Dir returns the *.sql files directly under dir at ref, sorted by path:
// the same selection postgres.ReadDir makes on disk, files only, no
// recursion. One git process serves the whole directory. A dir absent at
// the ref is an empty list.
func Dir(ref, dir string) ([]File, error) {
	ok, err := Exists(ref, dir)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	// git archive refuses a tree that does not contain the current
	// directory, so it is given the root-relative spec and run from the
	// repository's top level; the result is the same tree either way.
	// path.Join folds a "../" in the caller's path into the prefix, which
	// a plain concatenation would hand to git as "svc/../db", a path the
	// tree does not have.
	//
	// CAVEAT: git archive honours the export-ignore attribute, so a
	// migration a .gitattributes marks export-ignore is invisible at the
	// base. Nobody marks migrations that way, and a file that is in the
	// working tree but not at the base reads as added, which is the
	// cautious direction: the rule may raise a column that was there all
	// along, never miss one that was not.
	top, err := run("rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	prefix, err := run("rev-parse", "--show-prefix")
	if err != nil {
		return nil, err
	}
	rel, _ := relative(dir)
	cmd := command("archive", "--format=tar", ref+":"+path.Join(strings.TrimSpace(prefix), rel))
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
func Changed(ref, p string) (bool, error) {
	rel, err := relative(p)
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
func spec(ref, p string) (string, error) {
	rel, err := relative(p)
	if err != nil {
		return "", err
	}
	return ref + ":./" + rel, nil
}

// relative makes path relative to the current directory, forward-slashed,
// which is how git wants it on every OS.
func relative(p string) (string, error) {
	if filepath.IsAbs(p) {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(wd, p)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("%s is outside the current directory; run cdclint from the repository", p)
		}
		p = rel
	}
	return filepath.ToSlash(filepath.Clean(p)), nil
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
