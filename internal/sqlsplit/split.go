// Package sqlsplit cuts a SQL file into statements and remembers the line
// each one started on. It understands the four things that hide a
// semicolon: line comments, block comments, quoted strings and identifiers,
// and Postgres dollar quoting. It does not parse SQL; the readers do that
// on the statements it hands them.
package sqlsplit

import "strings"

// Statement is one statement's text, trimmed, and the 1-based line of its
// first non-blank character.
type Statement struct {
	Text string
	Line int
}

// Split returns the statements in src. Comments are removed from the text so
// readers never see them, but the line count they occupied is preserved.
func Split(src string) []Statement {
	var (
		out     []Statement
		buf     strings.Builder
		line    = 1
		start   = 0 // line the current statement began on; 0 until first char
		i       = 0
		n       = len(src)
		dollarT string // the $tag$ that opened a dollar quote, "" when not inside one
	)
	flush := func() {
		text := strings.TrimSpace(buf.String())
		if text != "" {
			out = append(out, Statement{Text: text, Line: start})
		}
		buf.Reset()
		start = 0
	}
	emit := func(s string) {
		if start == 0 && strings.TrimSpace(s) != "" {
			start = line
		}
		buf.WriteString(s)
	}
	for i < n {
		c := src[i]
		switch {
		case dollarT != "":
			// Inside $tag$ ... $tag$: copy through until the closing tag.
			if strings.HasPrefix(src[i:], dollarT) {
				emit(dollarT)
				i += len(dollarT)
				dollarT = ""
				continue
			}
			if c == '\n' {
				line++
			}
			emit(string(c))
			i++
		case c == '-' && i+1 < n && src[i+1] == '-':
			// Line comment: drop to end of line, keep the newline.
			for i < n && src[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && src[i+1] == '*':
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				end = n - i - 2
			}
			nl := strings.Count(src[i:i+2+end], "\n")
			line += nl
			i += 2 + end + 2
			// Keep the newlines so a column's line inside the statement can
			// still be computed from the statement's start.
			emit(" " + strings.Repeat("\n", nl))
		case c == '\'' || c == '"' || c == '`':
			// Quoted string or identifier; doubled quote is an escape.
			q := c
			j := i + 1
			for j < n {
				if src[j] == '\n' {
					line++
				}
				if src[j] == q {
					if j+1 < n && src[j+1] == q {
						j += 2
						continue
					}
					break
				}
				j++
			}
			if j >= n {
				j = n - 1
			}
			emit(src[i : j+1])
			i = j + 1
		case c == '$':
			// $$ or $tag$ opens a dollar quote; a lone $ is just a character.
			if tag, ok := dollarTag(src[i:]); ok {
				dollarT = tag
				emit(tag)
				i += len(tag)
				continue
			}
			emit("$")
			i++
		case c == ';':
			flush()
			i++
		default:
			if c == '\n' {
				line++
			}
			emit(string(c))
			i++
		}
	}
	flush()
	return out
}

// dollarTag reports whether s starts with a dollar-quote opener ($$ or
// $word$) and returns it.
func dollarTag(s string) (string, bool) {
	if len(s) < 2 || s[0] != '$' {
		return "", false
	}
	j := 1
	for j < len(s) && (isIdent(s[j])) {
		j++
	}
	if j < len(s) && s[j] == '$' {
		return s[:j+1], true
	}
	return "", false
}

func isIdent(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}
