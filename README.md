# cdclint

Lint the contract between your database, your Debezium connector and your
sink, before the deploy that silently drops a column.

**Status: v0.2.** Eight rules run against a corpus of real incidents; two
more are next. Released for macOS, Linux and Windows, on Homebrew and on the
GitHub Marketplace.

## Why are my columns null?

A change-data-capture pipeline has three schemas that must agree and nothing
that makes them agree:

1. **The source schema.** Postgres tables, evolved by migrations, changed
   weekly by the application team.
2. **The capture contract.** The Debezium connector JSON: `table.include.list`,
   `column.include.list`, replica identity, topic naming. Written once by
   whoever set up the pipeline, changed rarely and by hand.
3. **The sink schema.** The warehouse tables that read the stream: ClickHouse
   Kafka-engine tables and materialized views, or BigQuery, Snowflake and
   Iceberg tables behind a Kafka Connect sink connector. Changed when somebody
   downstream needs a column.

Three text files, three authors, three moments in time. Nothing reads them
together, and the failure when they disagree is silent by design:

- Debezium's `column.include.list` drops any column not on it **before the
  message reaches Kafka**. That is the feature that keeps PII out of the
  stream. The consequence is that a new column is absent from every message,
  and every message is still well-formed.
- The sink ignores fields it does not know and fills fields it does not
  receive with the type's default. ClickHouse's Kafka engine writes `0`,
  `''` or `NULL`. The BigQuery, Snowflake and Iceberg sink connectors with
  schema evolution off do the same.

No error, no log line, no metric. Lag is zero, offsets advance, the dashboard
is green, and a column is `0` on every row. It is found weeks later by whoever
first reads the column, who assumes the zeros are real or files the bug against
the wrong layer. The fix is one line in the connector. The repair is a
resnapshot of the table.

cdclint reads the three files in the pull request that changes any of them and
fails it with the line to add.

## What it checks

| rule | catches | status |
|---|---|---|
| `sink-column-not-captured` | the sink reads a column the connector does not include | v0.1 |
| `sink-table-not-captured` | the sink reads a table the connector does not include | v0.1 |
| `sink-column-unknown` | the sink expects a field the source table does not have (warning; renames and computed fields are legitimate) | v0.1 |
| `source-column-not-captured` | a source column nothing captures and nothing reads yet, so the day something asks for it is the day it is found missing (info) | v0.1 |
| `captured-column-missing` | the include list names a column the source does not have (warning) | v0.1 |
| `topic-table-mapping` | a Kafka-engine table reads a topic the connector will not produce | v0.1 |
| `mv-column-match` | ClickHouse streaming materialized views match by name, refreshable ones by position; the mismatch is loud on 24.8 and silent on 26.8 | v0.1 |
| `schema-before-connector` | this change adds a column to a captured table and does not touch the connector, the trap itself, judged on the diff (warning: leaving PII off is right, so it asks for the decision) | v0.2 |
| `replica-identity` | a captured table's replica identity cannot supply what the sink reads | next |
| `migration-numbering` | duplicate or gapped migration prefixes | next |

Findings name the file, the column, the rule and the fix:

```
error sink-column-not-captured
  public.reports.response_distance_m is read by analytics/schema/0020_report_validations.sql:14
  but is not in cdc/postgres-source.json column.include.list
  fix: add "public.report_validations.response_distance_m" to column.include.list,
       deploy the connector, then apply the sink schema
```

## Install

Per-OS steps, verification and uninstalling are in
[INSTALLING.md](INSTALLING.md). The short version:

Homebrew, on macOS or Linux:

```
brew install avison9/tap/cdclint
```

A release archive for your OS and CPU, from
[Releases](https://github.com/avison9/cdclint/releases): unpack it and put
`cdclint` on your PATH. `checksums.txt` beside the archives is signed with
cosign, keyless, by this repository's release workflow:

```
cosign verify-blob --certificate checksums.txt.pem --signature checksums.txt.sig \
  --certificate-identity-regexp 'github.com/avison9/cdclint' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com checksums.txt
```

With Go:

```
go install github.com/avison9/cdclint/cmd/cdclint@latest
```

## Run it

```
cdclint --migrations db/migrations \
        --connector cdc/postgres-source.json \
        --sink analytics/schema
```

For a warehouse behind a Kafka Connect sink connector, pass its config too, so
topics map to tables the way the connector maps them:

```
cdclint --migrations db/migrations \
        --connector cdc/postgres-source.json \
        --sink snowflake:warehouse/ddl \
        --sink-connector cdc/snowflake-sink.json
```

Add `--base origin/main` (any git ref) and the diff-aware rule judges the
change itself: a column added to a captured table with the connector left
untouched is raised while the author is still there. The action does this
on every pull request by default.

Files in, findings out, non-zero exit. No database, no daemon, no credentials.
Under a second on a laptop. `--fail-on warning` or `info` raises the bar;
`--format json` is for anything that wants to post findings somewhere. A
later live mode reads the deployed Postgres, Kafka Connect and sink to report
drift against what is actually running.

## In CI

```yaml
- uses: avison9/cdclint@v0
  with:
    migrations: db/migrations
    connector: cdc/postgres-source.json
    sink: analytics/schema
```

`@v0` follows the newest 0.x release; it becomes `@v1` at 1.0. The step
fails the pull request when the three files disagree, with the finding and
the fix in the log. Several sinks or sink connectors go one per
line; `fail-on`, `version` and `working-directory` are the other inputs. The
action downloads the release binary for the runner and verifies its
checksum before running it.

## Sinks, out of the box in v1

| sink | how tables are found | how a table maps to a topic |
|---|---|---|
| **ClickHouse**, Kafka engine | `CREATE TABLE ... ENGINE = Kafka` | `kafka_topic_list` |
| **ClickHouse**, Kafka Connect | table DDL + `ClickHouseSinkConnector` config | `topic2TableMap`, else the topic's table name |
| **BigQuery** | table DDL + `BigQuerySinkConnector` config | `topic2TableMap`, else the topic's table name |
| **Snowflake** | table DDL + `SnowflakeSinkConnector` config | `snowflake.topic2table.map`, else the topic's table name |
| **Iceberg** | table DDL (Spark or Trino form) + `IcebergSinkConnector` config | `iceberg.tables` with route regexes, else the topic's table name |

Without a sink connector config, a sink table maps to the source table of the
same name, and the finding says so. Debezium's `RegexRouter` transform is
applied when computing topic names. Other sources (MySQL, SQL Server) and
sinks are packages behind the same two interfaces.

## Design

- **One job.** Lint the contract. Not a CDC platform, not a migration runner,
  not monitoring.
- **Pluggable ends.** Postgres and Debezium as the source and capture
  readers; ClickHouse, BigQuery, Snowflake and Iceberg as sink readers, each
  behind a small interface so the next one is a package, not a rewrite.
- **The corpus is the spec.** [`corpus/`](corpus/) holds one directory per
  real failure shape with the three schemas and the expected findings. Every
  bug report becomes a corpus entry before it becomes a fix.
- **Boring technology.** Go, one static binary, Apache-2.0.

## Where it comes from

Extracted from a Postgres to ClickHouse pipeline that had the rule
"connector before schema" written into its repository and still hit this four
times in a year. A team that knows the trap hits it four times; a team that
does not hits it more and diagnoses it slower.

## Contributing

Start with [CONTRIBUTING.md](CONTRIBUTING.md): every change begins as a
corpus entry. AI coding agents read [AGENTS.md](AGENTS.md) first.

## License

Apache-2.0. See [LICENSE](LICENSE).
