package migrate

import "testing"

func TestDownFilesAreGolangMigrates(t *testing.T) {
	for p, want := range map[string]bool{
		"db/000092_add_createat.down.sql": true,
		"db/000092_add_createat.up.sql":   false,
		"db/1_init.DOWN.SQL":              true,
		`db\2_x.down.sql`:                 true,
		"db/0003_countdown.sql":           false,
		"db/V1__down.sql":                 false,
	} {
		if got := Down(p); got != want {
			t.Errorf("Down(%q) = %v, want %v", p, got, want)
		}
	}
}

func TestUpBlanksDownSectionsLineForLine(t *testing.T) {
	for name, tc := range map[string]struct{ in, want string }{
		"goose": {
			"-- +goose Up\nCREATE TABLE t (id INT);\n-- +goose Down\nDROP TABLE t;\n",
			"-- +goose Up\nCREATE TABLE t (id INT);\n-- +goose Down\n\n",
		},
		"sql-migrate with options": {
			"-- +migrate Up notransaction\nALTER TABLE t ADD c INT;\n\n-- +migrate Down\nALTER TABLE t DROP c;\n",
			"-- +migrate Up notransaction\nALTER TABLE t ADD c INT;\n\n-- +migrate Down\n\n",
		},
		"dbmate, down then up again": {
			"-- migrate:up transaction:false\nCREATE TABLE a (x INT);\n-- migrate:down\nDROP TABLE a;\n-- migrate:up\nCREATE TABLE b (y INT);\n",
			"-- migrate:up transaction:false\nCREATE TABLE a (x INT);\n-- migrate:down\n\n-- migrate:up\nCREATE TABLE b (y INT);\n",
		},
		"CRLF line endings": {
			"-- +goose Up\r\nCREATE TABLE t (id INT);\r\n-- +goose Down\r\nDROP TABLE t;\r\n",
			"-- +goose Up\r\nCREATE TABLE t (id INT);\r\n-- +goose Down\r\n\r\n",
		},
		"no markers is unchanged": {
			"CREATE TABLE t (id INT);\n-- a comment about down migrations\n",
			"CREATE TABLE t (id INT);\n-- a comment about down migrations\n",
		},
		"a column named down is not a marker": {
			"CREATE TABLE t (\n  down BOOLEAN\n);\n",
			"CREATE TABLE t (\n  down BOOLEAN\n);\n",
		},
	} {
		if got := Up(tc.in); got != tc.want {
			t.Errorf("%s:\n got %q\nwant %q", name, got, tc.want)
		}
	}
}
