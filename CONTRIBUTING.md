# Contributing to cdclint

cdclint has one job: prove that a CDC pipeline's three schemas agree before
the deploy that would silently drop a column. Contributions that sharpen
that job are welcome; contributions that give it a second job are not, and
the section on scope below says where the line is.

## Start with the corpus

Every bug report and every new rule begins as a corpus entry, not as code.

1. Create `corpus/<shape>/` with the three files as they disagreed:
   `migrations/`, `connector.json`, `sink/` (or `sink.<dialect>/`), and
   `sink-connector.json` if a Kafka Connect sink is involved. Reduce them to
   the statements that matter, keep the real names. See
   [`corpus/README.md`](corpus/README.md) for the layout.
2. Write `expected.txt` by hand: the findings cdclint should print. If the
   rule does not exist yet, add a `PENDING` file naming it and the test will
   skip the entry until it does.
3. Run `go test ./cmd/cdclint -run TestCorpus`. It fails. Now change the
   code until it passes.

An entry is the specification for the behaviour it demonstrates. It stays in
the repository forever, so the case can never regress silently. A pull
request that changes what the tool prints must change `expected.txt` files,
and the diff to those files is the first thing a reviewer reads.

**Real inputs beat invented ones.** The parsers have been wrong in ways the
invented entries did not catch and a real repository did (a `FROM` at the
start of a line; `EXCHANGE TABLES`). If you can reduce your own pipeline's
files to an entry, do that rather than writing one from imagination.

## Building and testing

Go 1.25, standard library only. There are no runtime dependencies and
adding one needs a reason stated in the pull request.

```
go vet ./... && go build ./... && go test ./...
```

Without a local Go toolchain:

```
docker run --rm -v "$PWD":/src -w /src golang:1.25-bookworm \
  sh -c "go vet ./... && go build -buildvcs=false ./... && go test ./..."
```

`gofmt` is not optional; CI runs `go vet`, and unformatted code is a review
comment you can avoid.

To regenerate every `expected.txt` after a deliberate message change:

```
go test ./cmd/cdclint -run TestCorpus -update
```

Then read `git diff corpus/` before committing. A regenerated expectation is
a change to what the tool promises, and it is reviewed as one.

## What a rule must be

- **One rule, one disagreement.** A rule answers a single question about the
  three schemas and its name says which. `sink-column-not-captured` is a
  rule; "schema consistency" is not.
- **A finding is an edit.** Every finding names the file and line of the
  thing to change, says what is wrong in one sentence, says what will happen
  if it is not fixed, and gives the fix as text a person can paste. Write it
  for someone reading a CI log at the end of the day with the files open.
- **Severity is a promise.** `error` means the pipeline will misbehave;
  `warning` means it probably will; `info` means "worth knowing". Do not
  raise a severity to make a rule feel important. `--fail-on` lets users
  decide what blocks a merge.
- **No false positives on the reference pipelines.** The repository this tool
  was extracted from runs clean. A rule that produces findings on a working
  pipeline is wrong until proven otherwise, however plausible the rule.

## Adding a source or a sink

Readers live behind two small interfaces in `internal/model`: `Contract`
(four questions: table captured, column captured, topic, setting to edit)
and the `SinkTable` shape. A new source (MySQL, SQL Server) is a package
under `internal/source/` that produces `*model.Source`; a new sink
(Elasticsearch, Redshift) is a package under `internal/sink/` that produces
`*model.Sink`, with a `connect.Kind` if a Kafka Connect sink connector maps
its topics. The engine and the rules do not change.

Each reader ships with at least one corpus entry showing it resolving a
table and catching a missing column.

## Scope

In scope: reading schemas from files or from live systems, comparing them,
and reporting. Out of scope: moving data, running migrations, registering
connectors, evolving sink schemas automatically, monitoring at runtime.
Those are what managed CDC services sell; this tool is the check that runs
before them, and it stays small enough to be adopted in an afternoon and
removed in a minute.

## Pull requests

- One rule, one reader, or one fix per pull request. Corpus entries for it
  in the same PR.
- The title is a sentence that says what changed for a user of the tool.
  The body says why, and what was considered and rejected.
- Keep the README's rule table honest: a rule is `v0.1` when its corpus
  entries pass, and not before.
- Commits carry the human author's name and nothing else. No `Co-Authored-By`
  trailers for tools, no "generated with" lines. See [AGENTS.md](AGENTS.md).

## Writing

Comments explain why, not what. A comment that restates the line below it
is deleted in review; a comment that records the incident a line exists
because of is the most valuable text in the repository. Write in plain
sentences.

## License

Apache-2.0. By contributing you agree your contribution is licensed the
same way.
