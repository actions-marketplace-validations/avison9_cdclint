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
	// ConnectorChanged is whether the connector file differs from the
	// base as git sees it, so a CRLF checkout of an unchanged file does
	// not silence the rule and a deliberate edit does.
	ConnectorChanged bool
}

// schemaBeforeConnector is the incident itself, judged on the diff: this
// change adds a column to a table the connector captures and does not
// touch the connector. The static rules can only see the state after the
// sink change lands, weeks later; this one sees the pull request that
// creates the column, which is the moment the author is still there and
// the fix is one line in the same PR.
//
// It is a WARNING, not an error, because leaving a column off the include
// list is often right (PII, a large blob, a column the warehouse has no
// use for). The finding asks for the decision to be made on purpose: add
// the column to the list, or change the connector file in any way that
// shows the omission was seen. A connector file changed in the diff, even
// without the column, silences it.
//
// A column that a sink already reads is not reported here; the static
// sink-column-not-captured rule reports that as the error it is.
//
// The second return is the set of columns raised, keyed the way
// sourceColumnNotCaptured keys its own, so the inventory does not list a
// column twice.
func schemaBeforeConnector(in *Input, reads []Read) ([]model.Finding, map[string]bool) {
	raised := map[string]bool{}
	if in.Base == nil || in.Base.Source == nil || in.Base.ConnectorChanged {
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
				Message: fmt.Sprintf("this change adds %s to a captured table and does not touch %s (compared with %s)\nthe column will not be in the stream; if a sink is later given it, every row will be the default until a snapshot", q, in.Contract.Pos().File, short(in.Base.Ref)),
				Fix:     fmt.Sprintf("either %s in the same change, or leave it off on purpose and say so in the connector file (any edit to it silences this)", edit(in.Contract.ColumnListSetting(), q)),
			})
		}
	}
	return fs, raised
}

// short abbreviates a full commit id the way git does; anything else (a
// branch name, a path) is shown as given.
func short(ref string) string {
	if len(ref) == 40 && strings.Trim(ref, "0123456789abcdef") == "" {
		return ref[:12]
	}
	return ref
}
