package gitread

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// A throwaway repository with a project in a subdirectory, the monorepo
// shape that broke the first version: from inside the subdirectory,
// "<ref>:<path>" resolved against the root and found nothing.
func setupRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t", "LC_ALL=C")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(rel, text string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q", "-b", "main")
	write("svc/db/migrations/0001_a.sql", "CREATE TABLE a (id UUID);")
	write("svc/db/migrations/notes.txt", "not sql")
	write("svc/db/migrations/sub/0002_nested.sql", "CREATE TABLE nested (id UUID);")
	write("svc/cdc/connector.json", `{"topic.prefix":"p"}`)
	git("add", ".")
	git("commit", "-q", "-m", "base")
	// The change: a new migration, the connector untouched on disk.
	write("svc/db/migrations/0002_b.sql", "ALTER TABLE a ADD COLUMN b TEXT;")
	return root
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
}

func TestReadsRelativeToTheCurrentDirectory(t *testing.T) {
	root := setupRepo(t)
	chdir(t, filepath.Join(root, "svc"))

	files, err := Dir("HEAD", "db/migrations")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != filepath.Join("db/migrations", "0001_a.sql") {
		t.Fatalf("Dir from a subdirectory = %+v, want the one base migration, not the nested one and not the txt", files)
	}
	if files[0].Text != "CREATE TABLE a (id UUID);" {
		t.Errorf("content = %q", files[0].Text)
	}

	text, err := Show("HEAD", "cdc/connector.json")
	if err != nil || text != `{"topic.prefix":"p"}` {
		t.Errorf("Show = %q, %v", text, err)
	}
	if _, err := Show("HEAD", "db/migrations/0002_b.sql"); err != ErrNotInRef {
		t.Errorf("a file new in the change should be ErrNotInRef, got %v", err)
	}

	abs := filepath.Join(root, "svc", "cdc", "connector.json")
	if text, err := Show("HEAD", abs); err != nil || text == "" {
		t.Errorf("absolute path under cwd should work, got %q %v", text, err)
	}
	if _, err := Show("HEAD", filepath.Join(root, "elsewhere.json")); err == nil {
		t.Error("absolute path outside cwd should be refused")
	}

	changed, err := Changed("HEAD", "cdc/connector.json")
	if err != nil || changed {
		t.Errorf("untouched connector reported changed=%v err=%v", changed, err)
	}
	if err := os.WriteFile(abs, []byte(`{"topic.prefix":"q"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if changed, _ := Changed("HEAD", "cdc/connector.json"); !changed {
		t.Error("edited connector reported unchanged")
	}
	if id, err := Resolve("HEAD"); err != nil || len(id) != 40 {
		t.Errorf("Resolve = %q, %v", id, err)
	}
}

func TestDirAbsentAtRefIsEmpty(t *testing.T) {
	root := setupRepo(t)
	chdir(t, root)
	files, err := Dir("HEAD", "svc/db/nowhere")
	if err != nil || files != nil {
		t.Errorf("absent dir = %v, %v; want nil, nil", files, err)
	}
}
