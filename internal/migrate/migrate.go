// Package migrate knows how the common migration tools mark the "down"
// half of a migration, which a forward migrate never runs and a reader of
// the schema must not apply either.
//
//   - golang-migrate keeps each half in its own file:
//     {version}_{title}.up.sql and {version}_{title}.down.sql.
//   - goose (-- +goose Up / -- +goose Down), sql-migrate (-- +migrate Up /
//     -- +migrate Down) and dbmate (-- migrate:up / -- migrate:down) keep
//     both halves in one file, the down section after the up one, each
//     marker optionally followed by options (notransaction,
//     transaction:false).
//
// Applied in name order, a separate down file runs just before its own up
// file and is usually a no-op; Mattermost's 000092 down file drops a
// column of another table (Reactions.CreateAt) and deleted it from the
// schema cdclint read. A down section in the same file runs right after
// its up section and undoes it every time: every goose table vanished.
package migrate

import (
	"path"
	"regexp"
	"strings"
)

// Down reports whether path names a golang-migrate down migration.
func Down(p string) bool {
	return strings.HasSuffix(strings.ToLower(path.Base(strings.ReplaceAll(p, "\\", "/"))), ".down.sql")
}

var marker = regexp.MustCompile(`(?i)^\s*--\s*(?:\+goose\s+(up|down)|\+migrate\s+(up|down)|migrate:(up|down))\b`)

// Up returns text with every down section blanked out, line for line, so
// the statements that remain keep their line numbers. Text with no markers
// is returned unchanged.
func Up(text string) string {
	lines := strings.SplitAfter(text, "\n")
	down, changed := false, false
	for i, l := range lines {
		m := marker.FindStringSubmatch(l)
		if m != nil {
			down = strings.EqualFold(m[1]+m[2]+m[3], "down")
			continue
		}
		if down {
			lines[i] = blank(l)
			changed = true
		}
	}
	if !changed {
		return text
	}
	return strings.Join(lines, "")
}

// blank keeps only the line's ending, so the line still counts.
func blank(l string) string {
	switch {
	case strings.HasSuffix(l, "\r\n"):
		return "\r\n"
	case strings.HasSuffix(l, "\n"):
		return "\n"
	}
	return ""
}
