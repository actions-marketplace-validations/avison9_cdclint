// Package sqlddl reads warehouse table DDL in the dialects that share one
// shape: CREATE TABLE name (column type, ...), ALTER TABLE ADD/DROP/RENAME
// COLUMN, DROP TABLE. BigQuery, Snowflake, Iceberg (Spark and Trino forms)
// and ClickHouse's column lists all fit it; each dialect's reader wraps this
// with what is specific to it.
//
// Files are applied in name order so a table created in one file and
// altered in a later one ends up with the columns the warehouse has.
package sqlddl

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/avison9/cdclint/internal/ddl"
	"github.com/avison9/cdclint/internal/model"
	"github.com/avison9/cdclint/internal/sqlsplit"
)

// Statement is one parsed DDL statement, kept so dialect readers can look at
// the parts this package does not interpret (ENGINE clauses, settings).
type Statement struct {
	Text string
	Pos  model.Pos
	// Words is the statement split on top-level whitespace.
	Words []string
}

// Hook lets a dialect reader see every statement, and every table as it is
// created, before the generic handling runs.
type Hook interface {
	// Statement is called for each statement; return true to claim it so
	// the generic reader skips it.
	Statement(st Statement, tables *Tables) bool
	// Created is called after CREATE TABLE produced a table, with the text
	// after the column list (engine, options, partitioning).
	Created(t *model.SinkTable, after string)
}

// Tables is the ordered set of tables seen so far.
type Tables struct {
	List []*model.SinkTable
	Sink string
}

// Find returns the table whose qualified name or last segment matches.
func (ts *Tables) Find(name string) *model.SinkTable {
	parts := ddl.SplitName(name)
	last := parts[len(parts)-1]
	full := strings.Join(parts, ".")
	for _, t := range ts.List {
		if strings.EqualFold(t.Name, full) {
			return t
		}
	}
	for _, t := range ts.List {
		if strings.EqualFold(t.Table, last) {
			return t
		}
	}
	return nil
}

func (ts *Tables) remove(name string) {
	t := ts.Find(name)
	if t == nil {
		return
	}
	for i := range ts.List {
		if ts.List[i] == t {
			ts.List = append(ts.List[:i], ts.List[i+1:]...)
			return
		}
	}
}

// ReadDir applies every *.sql file in dir in name order.
func ReadDir(dir, sink string, hook Hook) (*Tables, []string, error) {
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
	ts := &Tables{Sink: sink}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, nil, err
		}
		Apply(ts, f, string(b), hook)
	}
	return ts, files, nil
}

// Apply runs one file's statements.
func Apply(ts *Tables, file, text string, hook Hook) {
	for _, st := range sqlsplit.Split(text) {
		s := Statement{Text: st.Text, Pos: model.Pos{File: file, Line: st.Line}, Words: ddl.Words(st.Text)}
		if hook != nil && hook.Statement(s, ts) {
			continue
		}
		w := s.Words
		switch {
		case isCreateTable(w):
			createTable(ts, s, hook)
		case ddl.HasPrefixFold(w, "ALTER", "TABLE"):
			alterTable(ts, s)
		case ddl.HasPrefixFold(w, "DROP", "TABLE"):
			i := 2
			if ddl.HasPrefixFold(w[i:], "IF", "EXISTS") {
				i += 2
			}
			if i < len(w) {
				ts.remove(w[i])
			}
		}
	}
}

// createModifiers are the words that may sit between CREATE and TABLE.
var createModifiers = map[string]bool{"OR": true, "REPLACE": true, "TEMP": true, "TEMPORARY": true, "TRANSIENT": true, "EXTERNAL": true, "ICEBERG": true, "DYNAMIC": true, "HYBRID": true}

func isCreateTable(w []string) bool {
	if len(w) < 3 || !strings.EqualFold(w[0], "CREATE") {
		return false
	}
	for i := 1; i < len(w); i++ {
		if strings.EqualFold(w[i], "TABLE") {
			return true
		}
		if !createModifiers[strings.ToUpper(w[i])] {
			return false
		}
	}
	return false
}

func createTable(ts *Tables, s Statement, hook Hook) {
	w := s.Words
	i := 1
	for !strings.EqualFold(w[i], "TABLE") {
		i++
	}
	i++
	if ddl.HasPrefixFold(w[i:], "IF", "NOT", "EXISTS") {
		i += 3
	}
	if i >= len(w) {
		return
	}
	name := w[i]
	if p := strings.IndexByte(name, '('); p > 0 {
		name = name[:p]
	}
	nameAt := strings.Index(s.Text, w[i])
	body, after, ok := ddl.Body(s.Text[nameAt:])
	if !ok {
		return
	}
	parts := ddl.SplitName(name)
	t := &model.SinkTable{Sink: ts.Sink, Name: strings.Join(parts, "."), Table: parts[len(parts)-1], Pos: s.Pos}
	cursor := 0
	for _, item := range ddl.SplitTop(body) {
		line := s.Pos.Line + ddl.ItemLine(s.Text, item, &cursor) - 1
		iw := ddl.Words(item)
		if len(iw) < 1 {
			continue
		}
		switch strings.ToUpper(iw[0]) {
		case "CONSTRAINT", "PRIMARY", "UNIQUE", "FOREIGN", "INDEX", "PROJECTION", "CHECK", "LIKE", "CLUSTER":
			continue
		}
		col := model.Column{Name: ddl.Unquote(iw[0]), Pos: model.Pos{File: s.Pos.File, Line: line}}
		if len(iw) > 1 {
			col.Type = iw[1]
		}
		t.Columns = append(t.Columns, col)
	}
	// A later CREATE of the same name replaces the earlier one: CREATE OR
	// REPLACE does so by definition, and DROP + CREATE is the common
	// ClickHouse pattern. IF NOT EXISTS on an existing table is a no-op.
	if existing := ts.Find(t.Name); existing != nil {
		if ddl.HasPrefixFold(w[i-3:i], "IF", "NOT", "EXISTS") {
			return
		}
		ts.remove(t.Name)
	}
	ts.List = append(ts.List, t)
	if hook != nil {
		hook.Created(t, after)
	}
}

func alterTable(ts *Tables, s Statement) {
	w := s.Words
	i := 2
	if ddl.HasPrefixFold(w[i:], "IF", "EXISTS") {
		i += 2
	}
	if i >= len(w) {
		return
	}
	t := ts.Find(w[i])
	if t == nil {
		return
	}
	i++
	// ClickHouse: ON CLUSTER x
	if ddl.HasPrefixFold(w[i:], "ON", "CLUSTER") {
		i += 3
	}
	for _, a := range ddl.SplitTop(strings.Join(w[i:], " ")) {
		aw := ddl.Words(a)
		if len(aw) < 2 {
			continue
		}
		switch {
		case ddl.HasPrefixFold(aw, "ADD", "COLUMN"), ddl.HasPrefixFold(aw, "ADD", "COLUMNS"):
			j := 2
			// BigQuery: ADD COLUMN IF NOT EXISTS; Spark: ADD COLUMNS (a int, b int)
			if ddl.HasPrefixFold(aw[j:], "IF", "NOT", "EXISTS") {
				j += 3
			}
			rest := strings.Join(aw[j:], " ")
			if strings.HasPrefix(rest, "(") {
				body, _, ok := ddl.Body(rest)
				if !ok {
					continue
				}
				for _, item := range ddl.SplitTop(body) {
					iw := ddl.Words(item)
					if len(iw) > 0 {
						addColumn(t, iw, s.Pos)
					}
				}
				continue
			}
			addColumn(t, aw[j:], s.Pos)
		case ddl.HasPrefixFold(aw, "DROP", "COLUMN"), ddl.HasPrefixFold(aw, "DROP", "COLUMNS"):
			j := 2
			if ddl.HasPrefixFold(aw[j:], "IF", "EXISTS") {
				j += 2
			}
			rest := strings.Join(aw[j:], " ")
			names := []string{rest}
			if strings.HasPrefix(rest, "(") {
				body, _, _ := ddl.Body(rest)
				names = ddl.SplitTop(body)
			}
			for _, n := range names {
				nw := ddl.Words(n)
				if len(nw) > 0 {
					dropColumn(t, ddl.Unquote(nw[0]))
				}
			}
		case ddl.HasPrefixFold(aw, "RENAME", "COLUMN"):
			if len(aw) >= 5 && strings.EqualFold(aw[3], "TO") {
				if c := findColumn(t, ddl.Unquote(aw[2])); c != nil {
					c.Name = ddl.Unquote(aw[4])
				}
			}
		}
	}
}

func addColumn(t *model.SinkTable, iw []string, pos model.Pos) {
	name := ddl.Unquote(iw[0])
	if findColumn(t, name) != nil {
		return
	}
	col := model.Column{Name: name, Pos: pos}
	if len(iw) > 1 {
		col.Type = iw[1]
	}
	t.Columns = append(t.Columns, col)
}

func dropColumn(t *model.SinkTable, name string) {
	for i := range t.Columns {
		if strings.EqualFold(t.Columns[i].Name, name) {
			t.Columns = append(t.Columns[:i], t.Columns[i+1:]...)
			return
		}
	}
}

func findColumn(t *model.SinkTable, name string) *model.Column {
	for i := range t.Columns {
		if strings.EqualFold(t.Columns[i].Name, name) {
			return &t.Columns[i]
		}
	}
	return nil
}
