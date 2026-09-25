package main

import (
	"fmt"
	"strings"

	"github.com/avison9/cdclint/internal/capture/debezium"
	"github.com/avison9/cdclint/internal/model"
	"github.com/avison9/cdclint/internal/source"
	"github.com/avison9/cdclint/internal/source/mysql"
	"github.com/avison9/cdclint/internal/source/postgres"
)

// sourceReader is the one place the source dialect is chosen. A
// "postgres:" or "mysql:" prefix on --migrations says it outright;
// otherwise the connector's class decides, since a MySqlConnector (or
// MariaDbConnector) reads a MySQL database and every other Debezium source
// cdclint knows reads Postgres.
type sourceReader struct {
	mysql    bool
	database string // MySQL: the database unqualified migrations run in
}

func pickSource(migrations string, c *debezium.Contract) (sourceReader, string, error) {
	r := sourceReader{mysql: c.MySQL()}
	dir := migrations
	if i := strings.Index(migrations, ":"); i > 0 && !strings.Contains(migrations[:i], "/") {
		switch strings.ToLower(migrations[:i]) {
		case "mysql", "mariadb":
			r.mysql, dir = true, migrations[i+1:]
		case "postgres", "postgresql":
			r.mysql, dir = false, migrations[i+1:]
		default:
			return r, "", fmt.Errorf("unknown source dialect %q in --migrations %q", migrations[:i], migrations)
		}
	}
	if r.mysql {
		r.database = c.DefaultDatabase()
	}
	return r, dir, nil
}

func (r sourceReader) readDir(dir string) (*model.Source, []string, error) {
	if r.mysql {
		return mysql.ReadDir(dir, r.database)
	}
	return postgres.ReadDir(dir)
}

func (r sourceReader) readFiles(files []source.NamedFile) (*model.Source, error) {
	if r.mysql {
		return mysql.ReadFiles(files, r.database)
	}
	return postgres.ReadFiles(files)
}
