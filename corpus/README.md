# Corpus

Each directory is one real failure shape: the source migrations, the
connector JSON and the sink DDL exactly as they disagreed, plus
`expected.txt`, the findings cdclint must produce for it. The corpus is the
specification. A rule is done when its corpus entries pass, and a bug report
becomes a corpus entry before it becomes a fix.

The first three entries reproduce incidents from the project this tool was
extracted from, a Postgres to ClickHouse pipeline through Debezium:

| entry | shape |
|---|---|
| `schema-before-connector` | a migration and a sink change merged without the connector change; every row of the new column landed empty |
| `column-never-captured` | a column added to the source a year earlier and never put on the include list, found when a screen first asked for it |
| `refreshable-mv-positional` | a refreshable materialized view whose SELECT order did not match the target table; loud on ClickHouse 24.8, silent on 26.8 |
