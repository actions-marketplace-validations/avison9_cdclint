package main

import (
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/avison9/cdclint/internal/engine"
)

// update rewrites every expected.txt from the current output. Run it after
// changing a message deliberately, then read the diff before committing:
// the corpus is the specification, so a regenerated expectation is a
// change to what the tool promises.
var update = flag.Bool("update", false, "rewrite corpus expected.txt files")

// TestCorpus runs the tool on every corpus entry exactly as the command line
// would, from inside corpus/ so the paths in findings are the entry's own.
//
// Layout of an entry:
//
//	migrations/           source migrations
//	connector.json        Debezium source connector
//	sink/                 ClickHouse DDL, or sink.<dialect>/ for another warehouse
//	sink-connector.json   optional Kafka Connect sink config
//	expected.txt          the exact text output
//	PENDING               optional; names the rule the entry waits for, and skips it
func TestCorpus(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "corpus"))
	if err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	ran := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			if b, err := os.ReadFile(filepath.Join(name, "PENDING")); err == nil {
				t.Skipf("waiting for rule %s", strings.TrimSpace(string(b)))
			}
			var sinks []string
			matches, _ := filepath.Glob(filepath.Join(name, "sink*"))
			sort.Strings(matches)
			for _, m := range matches {
				base := filepath.Base(m)
				if base == "sink-connector.json" {
					continue
				}
				if base == "sink" {
					sinks = append(sinks, m)
				} else if strings.HasPrefix(base, "sink.") {
					sinks = append(sinks, strings.TrimPrefix(base, "sink.")+":"+m)
				}
			}
			var sinkConns []string
			if _, err := os.Stat(filepath.Join(name, "sink-connector.json")); err == nil {
				sinkConns = append(sinkConns, filepath.Join(name, "sink-connector.json"))
			}
			in, err := Load(filepath.Join(name, "migrations"), filepath.Join(name, "connector.json"), sinks, sinkConns)
			if err != nil {
				t.Fatal(err)
			}
			got := Render(engine.Run(in))
			expectedPath := filepath.Join(name, "expected.txt")
			if *update {
				if err := os.WriteFile(expectedPath, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(expectedPath)
			if err != nil {
				t.Fatalf("no expected.txt; run with -update to create it, then read it:\n%s", got)
			}
			if string(want) != got {
				t.Errorf("output differs from expected.txt\n--- want\n%s--- got\n%s", want, got)
			}
			ran++
		})
	}
	if ran == 0 {
		t.Fatal("no corpus entries ran")
	}
}
