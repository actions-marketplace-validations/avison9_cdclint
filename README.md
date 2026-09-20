# cdclint

Lint the contract between your database, your Debezium connector and your
sink, before the deploy that silently drops a column.

**Status: pre-release.** The problem is real, the rules are designed, the code
is being written. Nothing runs yet.

## Why are my columns null?

A change-data-capture pipeline has three schemas that must agree and nothing
that makes them agree:

1. **The source schema.** Postgres tables, evolved by migrations, changed
   weekly by the application team.
2. **The capture contract.** The Debezium connector JSON: `table.include.list`,
   `column.include.list`, replica identity, topic naming. Written once by
   whoever set up the pipeline, changed rarely and by hand.
3. **The sink schema.** ClickHouse Kafka-engine tables and materialized views,
   or Snowflake, BigQuery, Iceberg or Elasticsearch behind a sink connector.
   Changed when somebody downstream needs a column.

Three text files, three authors, three moments in time. Nothing reads them
together, and the failure when they disagree is silent by design:

- Debezium's `column.include.list` drops any column not on it **before the
  message reaches Kafka**. That is the feature that keeps PII out of the
  stream. The consequence is that a new column is absent from every message,
  and every message is still well-formed.
- The sink ignores fields it does not know and fills fields it does not
  receive with the type's default. ClickHouse's Kafka engine writes `0`,
  `''` or `NULL`. Warehouse sink connectors with schema evolution off do the
  same.

No error, no log line, no metric. Lag is zero, offsets advance, the dashboard
is green, and a column is `0` on every row. It is found weeks later by whoever
first reads the column, who assumes the zeros are real or files the bug against
the wrong layer. The fix is one line in the connector. The repair is a
resnapshot of the table.

cdclint reads the three files in the pull request that changes any of them and
fails it with the line to add.

## What it will check

| rule | catches |
|---|---|
| `sink-column-not-captured` | the sink reads a column the connector does not include |
| `captured-column-missing` | the include list names a column the source does not have |
| `replica-identity` | a captured table's replica identity cannot supply what the sink reads |
| `schema-before-connector` | a diff adds a source column the sink reads and does not touch the connector |
| `mv-column-match` | ClickHouse regular materialized views match by name, refreshable ones by position; the mismatch is loud on 24.8 and silent on 26.8 |
| `topic-table-mapping` | a Kafka-engine table reads a topic the connector will not produce |
| `migration-numbering` | duplicate or gapped migration prefixes |

Findings name the file, the column, the rule and the fix:

```
error sink-column-not-captured
  public.reports.response_distance_m is read by analytics/schema/0020_report_validations.sql:14
  but is not in cdc/postgres-source.json column.include.list
  fix: add "public.report_validations.response_distance_m" to column.include.list,
       deploy the connector, then apply the sink schema
```

## How it will run

```
cdclint --migrations db/migrations \
        --connector cdc/postgres-source.json \
        --sink analytics/schema
```

Files in, findings out, non-zero exit. No database, no daemon, no credentials.
Under a second on a laptop. A GitHub Action makes it five lines of YAML. A
later live mode reads the deployed Postgres, Kafka Connect and sink to report
drift against what is actually running.

## Design

- **One job.** Lint the contract. Not a CDC platform, not a migration runner,
  not monitoring.
- **Pluggable ends.** Postgres and Debezium first as the source and capture
  readers, ClickHouse first as the sink reader, each behind a small interface
  so the next one is a package, not a rewrite.
- **The corpus is the spec.** [`corpus/`](corpus/) holds one directory per
  real failure shape with the three schemas and the expected findings. Every
  bug report becomes a corpus entry before it becomes a fix.
- **Boring technology.** Go, one static binary, Apache-2.0.

## Where it comes from

Extracted from a Postgres to ClickHouse pipeline that had the rule
"connector before schema" written into its repository and still hit this four
times in a year. A team that knows the trap hits it four times; a team that
does not hits it more and diagnoses it slower.

## License

Apache-2.0. See [LICENSE](LICENSE).
