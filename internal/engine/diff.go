package engine

import (
	"fmt"
	"strings"

	"github.com/avison9/cdclint/internal/model"
)

// Base is what the same inputs looked like at the pull request's base ref,
// when the run was given one. Only the parts the diff rule compares are
// kept.
type Base struct {
	// Ref names the base in findings: the full commit id when it came
	// from git, or whatever the caller passed.
	Ref    string
	Source *model.Source
	// Contract is the connector as it was at the base, nil when the file
	// did not exist there. The rule compares its decisions with the head
	// contract's, table by table.
	Contract model.Contract
	// ConnectorChanged is whether the connector file differs from the
	// base as git sees it. False means no table's decisions can differ
	// and the comparison is skipped; a CRLF checkout of an unchanged
	// file is unchanged.
	ConnectorChanged bool
}

// schemaBeforeConnector is the incident itself, judged on the diff: this
// change adds a column to a table the connector captures and does not
// change what the connector captures for that table. The static rules can
// only see the state after the sink change lands, weeks later; this one
// sees the pull request that creates the column, which is the moment the
// author is still there and the fix is one line in the same PR.
//
// It is a WARNING, not an error, because leaving a column off the include
// list is often right (PII, a large blob, a column the warehouse has no
// use for). The finding asks for the decision to be made on purpose.
//
// "Touched" is judged PER TABLE, on the connector's decisions, not on the
// file's bytes. The first release silenced the rule whenever the connector
// file had changed at all, and RefuseRadar's first promotion range (#966)
// showed why that is wrong: #962 grew the report_validations include list
// and #963 added three reports columns, and the rule said nothing about
// reports because "the connector changed". A change to what the connector
// captures for report_validations says nothing about reports. So a table
// counts as touched when the base and head contracts disagree on whether
// the table is captured, or on whether any of its columns is; a table
// whose decisions are identical was not looked at.
//
// A column that a sink already reads is not reported here; the static
// sink-column-not-captured rule reports that as the error it is.
//
// The second return is the set of columns raised, keyed the way
// sourceColumnNotCaptured keys its own, so the inventory does not list a
// column twice.
func schemaBeforeConnector(in *Input, reads []Read) ([]model.Finding, map[string]bool) {
	raised := map[string]bool{}
	if in.Base == nil || in.Base.Source == nil {
		return nil, raised
	}
	if in.Base.ConnectorChanged && in.Base.Contract == nil {
		// The connector is new in this change: every table on it is a
		// fresh decision, and there is nothing to compare against.
		return nil, raised
	}
	read := map[string]bool{}
	for _, r := range reads {
		if r.Table != nil {
			read[strings.ToLower(r.Table.Qualified()+"."+r.Column.Name)] = true
		}
	}
	var fs []model.Finding
	for _, t := range in.Source.Tables {
		if !in.Contract.CapturesTable(t.Schema, t.Name) {
			continue
		}
		bt := in.Base.Source.Table(t.Schema, t.Name)
		if bt == nil {
			// A table that is new in this diff and already on the include
			// list was added deliberately; its columns are not a surprise.
			continue
		}
		if in.Base.ConnectorChanged && touched(in.Base.Contract, in.Contract, bt, t) {
			continue
		}
		for _, c := range t.Columns {
			if bt.Column(c.Name) != nil || in.Contract.CapturesColumn(t.Schema, t.Name, c.Name) {
				continue
			}
			if read[strings.ToLower(t.Qualified()+"."+c.Name)] {
				continue
			}
			q := fmt.Sprintf("%s.%s", t.Qualified(), c.Name)
			raised[strings.ToLower(q)] = true
			fs = append(fs, model.Finding{
				Rule: "schema-before-connector", Severity: model.Warning, Pos: c.Pos,
				Message: fmt.Sprintf("this change adds %s to a captured table and does not change what %s captures for %s (compared with %s)\nthe column will not be in the stream; if a sink is later given it, every row will be the default until a snapshot", q, in.Contract.Pos().File, t.Qualified(), short(in.Base.Ref)),
				Fix:     fmt.Sprintf("%s in the same change, or leave it off on purpose and let this warning stand as the record of that (it blocks only under --fail-on warning)", edit(in.Contract.ColumnListSetting(), q)),
			})
		}
	}
	return fs, raised
}

// touched reports whether the change altered what the connector captures
// for this one table: the table's own capture, or any column's, judged on
// every column the table has at either side. Columns new at the head are
// included so that an exclude-list connector which excludes the new column
// in the same change counts as a decision made.
func touched(base, head model.Contract, bt, t *model.Table) bool {
	if base.CapturesTable(t.Schema, t.Name) != head.CapturesTable(t.Schema, t.Name) {
		return true
	}
	seen := map[string]bool{}
	for _, cols := range [][]model.Column{bt.Columns, t.Columns} {
		for _, c := range cols {
			k := strings.ToLower(c.Name)
			if seen[k] {
				continue
			}
			seen[k] = true
			if base.CapturesColumn(t.Schema, t.Name, c.Name) != head.CapturesColumn(t.Schema, t.Name, c.Name) {
				return true
			}
		}
	}
	return false
}

// short abbreviates a full commit id the way git does; anything else (a
// branch name, a path) is shown as given.
func short(ref string) string {
	if len(ref) == 40 && strings.Trim(ref, "0123456789abcdef") == "" {
		return ref[:12]
	}
	return ref
}
