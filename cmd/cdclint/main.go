// Command cdclint lints the contract between a source database schema, a
// change-data-capture connector configuration and the warehouse schema
// that reads the captured stream.
//
// The three are text files in a repository, written by different people at
// different times, and nothing reads them together. A column present in the
// source and read by the sink but absent from the connector's include list is
// dropped before it reaches the stream, silently: every message is
// well-formed and the sink fills the column with its type's default on every
// row. cdclint fails the pull request instead.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/avison9/cdclint/internal/capture/debezium"
	"github.com/avison9/cdclint/internal/engine"
	"github.com/avison9/cdclint/internal/gitread"
	"github.com/avison9/cdclint/internal/model"
	"github.com/avison9/cdclint/internal/sink/clickhouse"
	"github.com/avison9/cdclint/internal/sink/connect"
	"github.com/avison9/cdclint/internal/sink/sqlddl"
	"github.com/avison9/cdclint/internal/source/postgres"
)

// version is set by the release build with -ldflags "-X main.version=...".
var version = "dev"

type multi []string

func (m *multi) String() string     { return strings.Join(*m, ",") }
func (m *multi) Set(v string) error { *m = append(*m, v); return nil }

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("cdclint", flag.ContinueOnError)
	var (
		migrations = fs.String("migrations", "", "directory of source migrations, applied in name order (Postgres)")
		connector  = fs.String("connector", "", "Debezium source connector JSON")
		sinks      multi
		sinkConns  multi
		format     = fs.String("format", "text", "output format: text or json")
		minSev     = fs.String("fail-on", "error", "exit non-zero at this severity or above: error, warning, info")
		base       = fs.String("base", "", "git ref of the change's base (a branch, a commit, origin/main); enables schema-before-connector, which judges the diff")
		showVer    = fs.Bool("version", false, "print the version and exit")
	)
	fs.Var(&sinks, "sink", "sink DDL directory as [dialect:]DIR; dialect is clickhouse (default), bigquery, snowflake or iceberg; repeatable")
	fs.Var(&sinkConns, "sink-connector", "Kafka Connect sink connector JSON; repeatable")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: cdclint --migrations DIR --connector FILE --sink [dialect:]DIR [--sink-connector FILE]...")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *showVer || (fs.NArg() > 0 && fs.Arg(0) == "version") {
		fmt.Println("cdclint", version)
		return 0
	}
	if *migrations == "" || *connector == "" || len(sinks) == 0 {
		fs.Usage()
		return 2
	}
	in, err := load(*migrations, *connector, sinks, sinkConns)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cdclint:", err)
		return 2
	}
	if *base != "" {
		b, err := LoadBase(*base, *migrations, *connector)
		if err != nil {
			fmt.Fprintln(os.Stderr, "cdclint:", err)
			return 2
		}
		in.Base = b
	}
	findings := engine.Run(in)
	switch *format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		type out struct {
			Rule     string `json:"rule"`
			Severity string `json:"severity"`
			File     string `json:"file,omitempty"`
			Line     int    `json:"line,omitempty"`
			Message  string `json:"message"`
			Fix      string `json:"fix,omitempty"`
		}
		var rows []out
		for _, f := range findings {
			rows = append(rows, out{f.Rule, f.Severity.String(), f.Pos.File, f.Pos.Line, f.Message, f.Fix})
		}
		if rows == nil {
			rows = []out{}
		}
		_ = enc.Encode(rows)
	default:
		os.Stdout.WriteString(Render(findings))
	}
	threshold := model.Error
	switch *minSev {
	case "warning":
		threshold = model.Warning
	case "info":
		threshold = model.Info
	}
	for _, f := range findings {
		if f.Severity >= threshold {
			return 1
		}
	}
	return 0
}

// Render is the text output: findings, then a one-line summary.
func Render(findings []model.Finding) string {
	var b strings.Builder
	counts := map[model.Severity]int{}
	for _, f := range findings {
		b.WriteString(f.Format())
		counts[f.Severity]++
	}
	if len(findings) == 0 {
		b.WriteString("ok: source, connector and sink agree\n")
		return b.String()
	}
	fmt.Fprintf(&b, "%d error(s), %d warning(s), %d info\n", counts[model.Error], counts[model.Warning], counts[model.Info])
	return b.String()
}

// Load reads every input named on the command line.
func load(migrations, connector string, sinks, sinkConns []string) (*engine.Input, error) {
	return Load(migrations, connector, sinks, sinkConns)
}

// Load is exported for the corpus test, which runs the tool exactly as the
// command line does.
func Load(migrations, connector string, sinks, sinkConns []string) (*engine.Input, error) {
	src, _, err := readSource(migrations)
	if err != nil {
		return nil, err
	}
	c, err := debezium.ReadFile(connector)
	if err != nil {
		return nil, err
	}
	in := &engine.Input{Source: src, Contract: c, Patterns: c}
	for _, f := range sinkConns {
		m, err := connect.ReadFile(f)
		if err != nil {
			return nil, err
		}
		in.Mappers = append(in.Mappers, m)
	}
	for _, s := range sinks {
		dialect, dir := "clickhouse", s
		if i := strings.Index(s, ":"); i > 0 && !strings.Contains(s[:i], "/") {
			dialect, dir = strings.ToLower(s[:i]), s[i+1:]
		}
		switch dialect {
		case "clickhouse":
			r, err := clickhouse.ReadDir(dir)
			if err != nil {
				return nil, err
			}
			in.Sinks = append(in.Sinks, r.Sink)
			in.Views = append(in.Views, r.Views...)
		case "bigquery", "snowflake", "iceberg":
			ts, _, err := sqlddl.ReadDir(dir, dialect, nil)
			if err != nil {
				return nil, err
			}
			in.Sinks = append(in.Sinks, &model.Sink{Tables: ts.List})
		default:
			return nil, fmt.Errorf("unknown sink dialect %q in %q", dialect, s)
		}
	}
	return in, nil
}

// LoadBase reads the migrations and the connector as they were at ref,
// straight from git, for the diff-aware rule. The sink is not needed: the
// rule asks what the change did to the source and the connector. A
// connector that did not exist at the base has no contract to compare
// with; creating it is the deliberate act the rule looks for.
func LoadBase(ref, migrations, connector string) (*engine.Base, error) {
	id, err := gitread.Resolve(ref)
	if err != nil {
		return nil, fmt.Errorf("--base: %w", err)
	}
	files, err := gitread.Dir(ref, migrations)
	if err != nil {
		return nil, fmt.Errorf("--base: %w", err)
	}
	var named []postgres.NamedFile
	for _, f := range files {
		named = append(named, postgres.NamedFile{Path: f.Path, Text: f.Text})
	}
	exists, err := gitread.Exists(ref, connector)
	if err != nil {
		return nil, fmt.Errorf("--base: %w", err)
	}
	if !exists {
		return baseFrom(id, named, nil, true)
	}
	changed, err := gitread.Changed(ref, connector)
	if err != nil {
		return nil, fmt.Errorf("--base: %w", err)
	}
	text, err := gitread.Show(ref, connector)
	if err != nil {
		return nil, fmt.Errorf("--base: %w", err)
	}
	return baseFrom(id, named, []byte(text), changed)
}

// BaseFromFiles builds the base from migrations already in memory and the
// connector's text at the base and now; the corpus test feeds it from a
// base/ directory, so the diff rule is tested without a repository.
func BaseFromFiles(ref string, migrations []postgres.NamedFile, baseConnector, headConnector string) (*engine.Base, error) {
	return baseFrom(ref, migrations, []byte(baseConnector), baseConnector != headConnector)
}

// baseFrom parses the base's migrations and connector. A nil connector
// means the file did not exist at the base; a parse failure there is an
// error, because the base once ran and its config once parsed, so the
// failure is in the reader and hiding it would hide the rule.
func baseFrom(ref string, migrations []postgres.NamedFile, connector []byte, changed bool) (*engine.Base, error) {
	src, err := postgres.ReadFiles(migrations)
	if err != nil {
		return nil, fmt.Errorf("--base %s: %w", ref, err)
	}
	b := &engine.Base{Ref: ref, Source: src, ConnectorChanged: changed}
	if connector != nil {
		c, err := debezium.Parse(connector)
		if err != nil {
			return nil, fmt.Errorf("--base %s: connector: %w", ref, err)
		}
		b.Contract = c
	}
	return b, nil
}
