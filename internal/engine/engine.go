// Package engine ties the three schemas together: it resolves every sink
// table to the source table that feeds it, then runs the rules over the
// result. Readers produce the schemas; this decides what they mean for each
// other.
package engine

import (
	"fmt"
	"strings"

	"github.com/avison9/cdclint/internal/model"
	"github.com/avison9/cdclint/internal/sink/clickhouse"
	"github.com/avison9/cdclint/internal/sink/connect"
)

// Input is everything one run reads.
type Input struct {
	Source   *model.Source
	Contract model.Contract
	// Patterns, when the contract can list them, lets captured-column-missing
	// check each include pattern against the schema.
	Patterns PatternLister
	Sinks    []*model.Sink
	Views    []clickhouse.View
	Mappers  []*connect.Mapper
	// Base is set when --base was given; it enables the diff-aware rule.
	Base *Base
}

// PatternLister is implemented by contracts whose column list is a set of
// patterns that can be checked one by one.
type PatternLister interface {
	ColumnPatterns() []string
}

// Read is one sink column resolved to the source column it expects.
type Read struct {
	Sink    *model.SinkTable
	Column  model.Column
	Table   *model.Table // nil when the sink table maps to no source table
	Mapping model.Mapping
	Topic   string
	Mapper  *connect.Mapper
}

// isMetadata reports whether a sink column is one the connector or the
// sink adds around the row rather than a field of it.
func isMetadata(name string) bool {
	if strings.HasPrefix(name, "_") {
		return true
	}
	switch strings.ToUpper(name) {
	case "RECORD_METADATA", "RECORD_CONTENT":
		return true
	}
	return false
}

// resolve maps every sink table to a source table. Tables it cannot map are
// returned with Table nil so the topic rule can report the ones that named
// a topic nothing produces.
func resolve(in *Input) []Read {
	var reads []Read
	byTopic := map[string]*model.Table{}
	for _, t := range in.Source.Tables {
		byTopic[in.Contract.Topic(t.Schema, t.Name)] = t
	}
	for _, s := range in.Sinks {
		for _, st := range s.Tables {
			var (
				table   *model.Table
				mapping model.Mapping
				mapper  *connect.Mapper
				topic   = st.Topic
			)
			switch {
			case st.Topic != "":
				table = byTopic[st.Topic]
				mapping = model.MapByTopic
			default:
				for _, m := range in.Mappers {
					if m.Kind != connect.Unknown && string(m.Kind) != st.Sink {
						continue
					}
					for tp, t := range byTopic {
						if name, _, ok := m.TableFor(tp); ok && connect.Matches(st.Name, name) {
							table, mapping, mapper, topic = t, model.MapByConnector, m, tp
							break
						}
					}
					if table != nil {
						break
					}
				}
				// A warehouse table with no connector mapping is assumed to
				// be fed from the source table of the same name. ClickHouse
				// is excluded: its ordinary tables are usually materialized
				// view targets with transformed columns, and only its
				// Kafka-engine tables read the stream directly.
				if table == nil && st.Sink != "clickhouse" {
					if t := in.Source.Table("", st.Table); t != nil {
						table, mapping, topic = t, model.MapByName, in.Contract.Topic(t.Schema, t.Name)
					}
				}
			}
			if table == nil {
				if st.Topic != "" {
					reads = append(reads, Read{Sink: st, Mapping: model.MapByTopic, Topic: st.Topic})
				}
				continue
			}
			for _, c := range st.Columns {
				if isMetadata(c.Name) {
					continue
				}
				reads = append(reads, Read{Sink: st, Column: c, Table: table, Mapping: mapping, Topic: topic, Mapper: mapper})
			}
		}
	}
	return reads
}

// Run resolves and applies every rule, returning sorted findings.
func Run(in *Input) []model.Finding {
	reads := resolve(in)
	var fs []model.Finding
	fs = append(fs, topicTableMapping(in, reads)...)
	fs = append(fs, sinkTableNotCaptured(in, reads)...)
	fs = append(fs, sinkColumnNotCaptured(in, reads)...)
	fs = append(fs, sinkColumnUnknown(in, reads)...)
	// The diff rule runs before the inventory so a column it raised as a
	// warning is not listed again as information.
	diff, raised := schemaBeforeConnector(in, reads)
	fs = append(fs, diff...)
	fs = append(fs, sourceColumnNotCaptured(in, reads, raised)...)
	fs = append(fs, capturedColumnMissing(in)...)
	fs = append(fs, mvColumnMatch(in)...)
	model.Sort(fs)
	return fs
}

// edit words the remedy for the contract's list mode: a column is added to
// an include list and removed from an exclude list.
func edit(setting, name string) string {
	if excludes(setting) {
		return fmt.Sprintf("remove %s from %s", name, setting)
	}
	return fmt.Sprintf("add %s to %s", name, setting)
}

func how(r Read) string {
	switch r.Mapping {
	case model.MapByTopic:
		return fmt.Sprintf("reads topic %s", r.Topic)
	case model.MapByConnector:
		return fmt.Sprintf("is fed topic %s by %s", r.Topic, r.Mapper.Pos.File)
	default:
		return fmt.Sprintf("is assumed to be fed from %s.%s by table name (no sink connector config maps it)", r.Table.Schema, r.Table.Name)
	}
}

func topicTableMapping(in *Input, reads []Read) []model.Finding {
	var fs []model.Finding
	seen := map[*model.SinkTable]bool{}
	for _, r := range reads {
		if r.Table != nil || seen[r.Sink] {
			continue
		}
		seen[r.Sink] = true
		fs = append(fs, model.Finding{
			Rule: "topic-table-mapping", Severity: model.Error, Pos: r.Sink.Pos,
			Message: fmt.Sprintf("%s reads topic %s, which no table in the source schema produces through %s", r.Sink.Name, r.Topic, in.Contract.Pos().File),
			Fix:     "check the topic name against topic.prefix and any RegexRouter transform, or add the table it expects to the source migrations",
		})
	}
	return fs
}

func sinkTableNotCaptured(in *Input, reads []Read) []model.Finding {
	var fs []model.Finding
	seen := map[*model.SinkTable]bool{}
	for _, r := range reads {
		if r.Table == nil || seen[r.Sink] || in.Contract.CapturesTable(r.Table.Schema, r.Table.Name) {
			continue
		}
		seen[r.Sink] = true
		fs = append(fs, model.Finding{
			Rule: "sink-table-not-captured", Severity: model.Error, Pos: r.Sink.Pos,
			Message: fmt.Sprintf("%s %s, but %s.%s is not matched by %s in %s\nnothing will ever arrive on that topic", r.Sink.Name, how(r), r.Table.Schema, r.Table.Name, in.Contract.TableListSetting(), in.Contract.Pos().File),
			Fix:     edit(in.Contract.TableListSetting(), r.Table.Schema+"."+r.Table.Name) + " and deploy the connector before the sink schema",
		})
	}
	return fs
}

func sinkColumnNotCaptured(in *Input, reads []Read) []model.Finding {
	var fs []model.Finding
	for _, r := range reads {
		if r.Table == nil || !in.Contract.CapturesTable(r.Table.Schema, r.Table.Name) {
			continue
		}
		src := r.Table.Column(r.Column.Name)
		if src == nil || in.Contract.CapturesColumn(r.Table.Schema, r.Table.Name, src.Name) {
			continue
		}
		q := fmt.Sprintf("%s.%s.%s", r.Table.Schema, r.Table.Name, src.Name)
		fs = append(fs, model.Finding{
			Rule: "sink-column-not-captured", Severity: model.Error, Pos: r.Column.Pos,
			Message: fmt.Sprintf("%s is read by %s (%s) but is not matched by %s in %s\nevery row will carry the column's default, with no error anywhere", q, r.Sink.Name, how(r), in.Contract.ColumnListSetting(), in.Contract.Pos().File),
			Fix:     edit(in.Contract.ColumnListSetting(), q) + ", deploy the connector, then apply the sink schema; rows already written need a snapshot",
		})
	}
	return fs
}

func sinkColumnUnknown(in *Input, reads []Read) []model.Finding {
	var fs []model.Finding
	for _, r := range reads {
		if r.Table == nil || r.Table.Column(r.Column.Name) != nil {
			continue
		}
		fs = append(fs, model.Finding{
			Rule: "sink-column-unknown", Severity: model.Warning, Pos: r.Column.Pos,
			Message: fmt.Sprintf("%s expects a field %s that %s.%s does not have (%s)\nif the connector renames or computes it, this is fine; otherwise it will be the default on every row", r.Sink.Name, r.Column.Name, r.Table.Schema, r.Table.Name, how(r)),
		})
	}
	return fs
}

func sourceColumnNotCaptured(in *Input, reads []Read, raised map[string]bool) []model.Finding {
	var fs []model.Finding
	declared := map[string]bool{}
	for _, r := range reads {
		if r.Table != nil {
			declared[strings.ToLower(r.Table.Qualified()+"."+r.Column.Name)] = true
		}
	}
	for _, t := range in.Source.Tables {
		if !in.Contract.CapturesTable(t.Schema, t.Name) {
			continue
		}
		for _, c := range t.Columns {
			if in.Contract.CapturesColumn(t.Schema, t.Name, c.Name) || declared[strings.ToLower(t.Qualified()+"."+c.Name)] {
				continue
			}
			if raised[strings.ToLower(t.Qualified()+"."+c.Name)] {
				continue
			}
			fs = append(fs, model.Finding{
				Rule: "source-column-not-captured", Severity: model.Info, Pos: c.Pos,
				Message: fmt.Sprintf("%s.%s exists and is not matched by %s in %s; nothing reads it yet\nthe day a sink asks for it is the day it is found missing", t.Qualified(), c.Name, in.Contract.ColumnListSetting(), in.Contract.Pos().File),
			})
		}
	}
	return fs
}

// capturedColumnMissing checks each include-list pattern against the
// schema. A pattern that matches nothing is usually a typo or a column that
// was dropped, and Debezium does not complain.
func capturedColumnMissing(in *Input) []model.Finding {
	if in.Patterns == nil {
		return nil
	}
	var fs []model.Finding
	for _, p := range in.Patterns.ColumnPatterns() {
		for _, name := range expand(p) {
			if matchesAny(in, name) {
				continue
			}
			fs = append(fs, model.Finding{
				Rule: "captured-column-missing", Severity: model.Warning, Pos: in.Contract.Pos(),
				Message: fmt.Sprintf("%s in %s matches no column in the source schema", name, in.Contract.ColumnListSetting()),
				Fix:     "remove it, or check the spelling against the migrations",
			})
		}
	}
	return fs
}

// expand turns the common `schema.table\.(a|b|c)` shape into one pattern
// per alternative so each column is checked on its own. Any other shape is
// returned as is.
func expand(p string) []string {
	open := strings.LastIndex(p, "(")
	if open < 0 || !strings.HasSuffix(p, ")") {
		return []string{p}
	}
	inner := p[open+1 : len(p)-1]
	if strings.ContainsAny(inner, "()[]*+?.\\") {
		return []string{p}
	}
	var out []string
	for _, alt := range strings.Split(inner, "|") {
		out = append(out, p[:open]+alt)
	}
	return out
}

func matchesAny(in *Input, pattern string) bool {
	m, err := compileAnchored(pattern)
	if err != nil {
		return true
	}
	for _, t := range in.Source.Tables {
		for _, c := range t.Columns {
			if m(t.Qualified() + "." + c.Name) {
				return true
			}
		}
	}
	return false
}
