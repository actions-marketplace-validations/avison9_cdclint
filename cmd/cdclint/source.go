package main

import (
	"github.com/avison9/cdclint/internal/model"
	"github.com/avison9/cdclint/internal/source/postgres"
)

// readSource is the one place the source dialect is chosen. Postgres is the
// only one in v1; a MySQL reader slots in here.
func readSource(dir string) (*model.Source, []string, error) {
	return postgres.ReadDir(dir)
}
