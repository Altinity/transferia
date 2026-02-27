# Core2CH Matrix

This directory contains the portability contract and gate tooling for
`pg/mysql/mongo/kafka -> clickhouse`.

## Files

- `core2ch.yaml`: scenario contract (`C01..C18`) with per-source applicability.
- `go run ./tools/testmatrix gate ...`: report/gate utility for mandatory coverage.
- `go run ./tools/testmatrix suite ...`: CDC local suite manifest helper.
- `coverage_report.md`: generated report (wave-aware).
- `sources.yaml`: source variant matrix for image/version runs.

## Commands

- `make test-matrix-gap-report`
- `make test-matrix-core`
- `make test-matrix-wave1`
- `make test-matrix-wave2`

## Policy

Core parity covers only:

- `pg2ch`
- `mysql2ch`
- `mongo2ch`
- `kafka2ch`

Extension-only buckets are defined in `core2ch.yaml` and do not fail core parity.
