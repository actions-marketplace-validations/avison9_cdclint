package engine

import (
	"fmt"
	"strings"

	"github.com/avison9/cdclint/internal/model"
)

// mvColumnMatch applies ClickHouse's two matching rules. A streaming
// materialized view (TO target AS SELECT, fed by inserts) matches its
// SELECT's output names to the target's columns BY NAME; a refreshable one
// (REFRESH EVERY ...) runs INSERT ... SELECT, which is POSITIONAL. The same
// reorder is therefore harmless in one and silently wrong in the other, and
// between columns of the same type no version of ClickHouse reports it.
func mvColumnMatch(in *Input) []model.Finding {
	var fs []model.Finding
	tables := map[string]*model.SinkTable{}
	for _, s := range in.Sinks {
		for _, t := range s.Tables {
			if t.Sink == "clickhouse" {
				tables[strings.ToLower(t.Name)] = t
				tables[strings.ToLower(t.Table)] = t
			}
		}
	}
	for _, v := range in.Views {
		if v.Target == "" {
			continue
		}
		target := tables[strings.ToLower(v.Target)]
		if target == nil {
			continue
		}
		if v.Refreshable {
			n := len(v.Columns)
			if len(target.Columns) < n {
				n = len(target.Columns)
			}
			for i := 0; i < n; i++ {
				if strings.EqualFold(v.Columns[i], target.Columns[i].Name) {
					continue
				}
				fs = append(fs, model.Finding{
					Rule: "mv-column-match", Severity: model.Error, Pos: v.Pos,
					Message: fmt.Sprintf("%s is refreshable, so its SELECT is inserted into %s by position\ncolumn %d of the SELECT is %s but column %d of %s is %s\nbetween columns of the same type ClickHouse writes the wrong values and reports the refresh as finished", v.Name, target.Name, i+1, v.Columns[i], i+1, target.Name, target.Columns[i].Name),
					Fix:     fmt.Sprintf("reorder the SELECT in %s to match %s's column order exactly", v.Name, target.Name),
				})
				break
			}
			if len(v.Columns) != len(target.Columns) {
				fs = append(fs, model.Finding{
					Rule: "mv-column-match", Severity: model.Error, Pos: v.Pos,
					Message: fmt.Sprintf("%s is refreshable and selects %d columns, but %s has %d", v.Name, len(v.Columns), target.Name, len(target.Columns)),
					Fix:     fmt.Sprintf("make the SELECT in %s list every column of %s, in order", v.Name, target.Name),
				})
			}
			continue
		}
		for _, c := range v.Columns {
			found := false
			for _, tc := range target.Columns {
				if strings.EqualFold(tc.Name, c) {
					found = true
					break
				}
			}
			if !found {
				fs = append(fs, model.Finding{
					Rule: "mv-column-match", Severity: model.Error, Pos: v.Pos,
					Message: fmt.Sprintf("%s selects %s, which %s does not have; a streaming view matches by name and ClickHouse rejects the insert", v.Name, c, target.Name),
					Fix:     fmt.Sprintf("alias the expression to a column of %s or add the column to it", target.Name),
				})
			}
		}
	}
	return fs
}
