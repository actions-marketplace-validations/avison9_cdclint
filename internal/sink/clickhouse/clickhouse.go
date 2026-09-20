// Package clickhouse reads a directory of ClickHouse DDL. Two kinds of table
// matter to the contract:
//
//   - Kafka-engine tables, which name the topic they consume in
//     kafka_topic_list and whose columns are the fields read from each
//     message. A field the message lacks is filled with the type's default,
//     silently, which is the failure this tool exists for.
//   - ordinary tables, which only read the stream when a ClickHouse Kafka
//     Connect sink writes into them; those are mapped through the sink
//     connector config, so they are kept with no topic.
//
// Materialized views are collected for the mv-column-match rule.
package clickhouse

import (
	"regexp"
	"strings"

	"github.com/avison9/cdclint/internal/ddl"
	"github.com/avison9/cdclint/internal/model"
	"github.com/avison9/cdclint/internal/sink/sqlddl"
)

// View is a materialized view: which table it writes to, whether it is
// refreshable (positional insert) or streaming (matched by name), and the
// output names of its SELECT.
type View struct {
	Name        string
	Target      string
	Refreshable bool
	Columns     []string
	Pos         model.Pos
}

// Result is everything the reader found.
type Result struct {
	Sink  *model.Sink
	Views []View
	Files []string
}

type hook struct {
	views []View
}

var topicList = regexp.MustCompile(`(?i)kafka_topic_list\s*=\s*'([^']*)'`)

func (h *hook) Statement(st sqlddl.Statement, ts *sqlddl.Tables) bool {
	w := st.Words
	// EXCHANGE TABLES a AND b swaps two tables atomically; it is how a
	// column is renamed on a ReplacingMergeTree without a window where
	// the table is missing. After it, each name has the other's columns.
	if ddl.HasPrefixFold(w, "EXCHANGE", "TABLES") && len(w) >= 5 && strings.EqualFold(w[3], "AND") {
		a, b := ts.Find(w[2]), ts.Find(w[4])
		if a != nil && b != nil {
			a.Name, b.Name = b.Name, a.Name
			a.Table, b.Table = b.Table, a.Table
		}
		return true
	}
	// RENAME TABLE a TO b [, c TO d]
	if ddl.HasPrefixFold(w, "RENAME", "TABLE") {
		for _, pair := range ddl.SplitTop(strings.Join(w[2:], " ")) {
			pw := ddl.Words(pair)
			if len(pw) == 3 && strings.EqualFold(pw[1], "TO") {
				if t := ts.Find(pw[0]); t != nil {
					parts := ddl.SplitName(pw[2])
					t.Name = strings.Join(parts, ".")
					t.Table = parts[len(parts)-1]
				}
			}
		}
		return true
	}
	if ddl.HasPrefixFold(w, "CREATE", "MATERIALIZED", "VIEW") || ddl.HasPrefixFold(w, "CREATE", "OR", "REPLACE", "MATERIALIZED", "VIEW") {
		h.views = append(h.views, parseView(st))
		return true
	}
	if ddl.HasPrefixFold(w, "DROP", "VIEW") {
		i := 2
		if ddl.HasPrefixFold(w[i:], "IF", "EXISTS") {
			i += 2
		}
		if i < len(w) {
			name := strings.Join(ddl.SplitName(w[i]), ".")
			for j := range h.views {
				if strings.EqualFold(h.views[j].Name, name) {
					h.views = append(h.views[:j], h.views[j+1:]...)
					break
				}
			}
		}
		return true
	}
	return false
}

func (h *hook) Created(t *model.SinkTable, after string) {
	up := strings.ToUpper(after)
	if idx := strings.Index(up, "ENGINE"); idx >= 0 {
		eng := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(up[idx+len("ENGINE"):]), "="))
		if strings.HasPrefix(eng, "KAFKA") {
			if m := topicList.FindStringSubmatch(after); m != nil {
				topics := strings.Split(m[1], ",")
				t.Topic = strings.TrimSpace(topics[0])
			}
		}
	}
}

func parseView(st sqlddl.Statement) View {
	w := st.Words
	v := View{Pos: st.Pos}
	i := 0
	for i < len(w) && !strings.EqualFold(w[i], "VIEW") {
		i++
	}
	i++
	if ddl.HasPrefixFold(w[i:], "IF", "NOT", "EXISTS") {
		i += 3
	}
	if i < len(w) {
		v.Name = strings.Join(ddl.SplitName(w[i]), ".")
	}
	for j := i; j < len(w); j++ {
		switch strings.ToUpper(w[j]) {
		case "REFRESH":
			v.Refreshable = true
		case "TO":
			if j+1 < len(w) {
				v.Target = strings.Join(ddl.SplitName(w[j+1]), ".")
			}
		}
	}
	// Output names: the last word of each top-level SELECT item, or the
	// item itself when there is no alias. Words keeps every parenthesised
	// group as one word, so the first SELECT and the first FROM seen here
	// are the outer query's; subqueries and function calls are inside
	// groups and never split.
	sel := -1
	for j := i; j < len(w); j++ {
		if strings.EqualFold(w[j], "SELECT") {
			sel = j
			break
		}
	}
	if sel < 0 {
		return v
	}
	end := len(w)
	for j := sel + 1; j < len(w); j++ {
		if strings.EqualFold(w[j], "FROM") {
			end = j
			break
		}
	}
	for _, item := range ddl.SplitTop(strings.Join(w[sel+1:end], " ")) {
		iw := ddl.Words(item)
		if len(iw) == 0 {
			continue
		}
		name := iw[len(iw)-1]
		if len(iw) == 1 {
			parts := ddl.SplitName(iw[0])
			name = parts[len(parts)-1]
		}
		v.Columns = append(v.Columns, ddl.Unquote(name))
	}
	return v
}

// ReadDir reads every *.sql file in dir in name order.
func ReadDir(dir string) (*Result, error) {
	h := &hook{}
	ts, files, err := sqlddl.ReadDir(dir, "clickhouse", h)
	if err != nil {
		return nil, err
	}
	return &Result{Sink: &model.Sink{Tables: ts.List}, Views: h.views, Files: files}, nil
}

// ReadFiles reads DDL already in memory, in the order given.
func ReadFiles(files []sqlddl.NamedFile) *Result {
	h := &hook{}
	ts := sqlddl.ReadFiles(files, "clickhouse", h)
	var names []string
	for _, f := range files {
		names = append(names, f.Path)
	}
	return &Result{Sink: &model.Sink{Tables: ts.List}, Views: h.views, Files: names}
}
