# Upstream Core CDC Import Report

Date: 2026-02-25
Branch: `codex/upstream-main-core-cdc-merge`
Base: `4865401f`
Upstream: `upstream/main`

## Scope Result
Imported runtime functionality for core CDC path (`pg/mysql/mongo/kafka -> clickhouse`) from upstream `main` using cherry-pick/manual partial waves.
Upstream test-file imports were removed from this branch.

## Imported Upstream SHAs

### Wave 1 (provider bugfixes)
- `72d663f9` (full cherry-pick)
- `57c28280` (full cherry-pick)
- `d9c85803` (full cherry-pick)
- `c7f81d64` (full cherry-pick)
- `a609bcd2` (full cherry-pick)
- `e081b290` (full cherry-pick)
- `75ed9994` (conflict-resolved cherry-pick, kept local deletion of `pkg/providers/kafka/reader/common.go`)

### Wave 2 (kafka refactor)
- `20846311` (runtime-only partial import)
- `51b23c57` (runtime-only partial import)

### Wave 3 (clickhouse schema/runtime)
- `a86ec0f0` (runtime-only partial import)
- `46414b0e` (runtime-only partial import)
- `f5dc0fe8` (runtime-only partial import)
- `3432ce46` (runtime-only partial import)
- `b1a95070` (runtime-only partial import)

### Wave 4 (coordinator/snapshot)
- `4385f970` (runtime-only partial import)
- `4d2d0255` (runtime-only partial import)
- `637857e1` (runtime-only partial import)
- `34ba65b1` (initial partial import, then reverted due global provider-factory signature blast radius)
- `d0bcc1d1` (runtime-only partial import)
- `41de8622` (full file import)

### Wave 5 (model/API)
- `53b35bc1` (runtime-only partial import)
- `bc166125` (runtime-only partial import for core providers)
- `60356947` (runtime-only partial import)

## Additional Upstream Dependency Imports (to restore consistency)
Pulled from `upstream/main` as minimal compatibility dependencies discovered during compile:
- `internal/logger/sanitizer_encoder.go`
- `pkg/abstract/expirer.go`
- `pkg/parsers/ysrable_parser.go`
- `pkg/parsers/abstract.go`
- `pkg/providers/postgres/schema.go`
- clickhouse model/recipe files aligned to upstream main:
  - `pkg/providers/clickhouse/model/model_ch_destination.go`
  - `pkg/providers/clickhouse/model/model_ch_source.go`
  - `pkg/providers/clickhouse/model/model_sink_params.go`
  - `pkg/providers/clickhouse/recipe/chrecipe.go`

## Compatibility Fixes Added
- `pkg/abstract/model/transfer.go`: pass transfer type to `DestinationCompatibility.Compatible`.
- `pkg/abstract/storage.go`:
  - restore `SampleableStorage` compatibility contract,
  - add `PartID()` compatibility alias to `TableDescription`.
- `pkg/providers/kafka/source.go`: add backward-compatible `NewSource(...)` wrapper and keep partition-aware constructor as `NewSourceWithPartition(...)`.
- `pkg/providers/kafka/provider.go` / `pkg/providers/kafka/source_multi_topics.go`: updated calls for compatibility.
- `pkg/coordinator/s3coordinator/coordinator_s3_test.go`: updated `FinishOperation` call for runID argument.
- `pkg/worker/tasks/*`: adjusted signatures for current interfaces (`Flush(bool)`, `CheckSecondaryWorkersDone(..., operationID)`, TPP getter args).
- `pkg/providers/clickhouse/sink_table.go`: pass CH server version to `InsertSettings().ToQueryOption(version)`.

## Explicitly Deferred / Not Imported
- `fbd30580` (pg sink inline bulk insert)
- `427dbf6a`, `dc6a632e` (pg_dump-only path)
- `8cc66756`, `aee67ebe` (mysql datetime fallback/revert pair)

## Test/Validation Summary

### Compile-level gates (PASS)
- `go test ./pkg/abstract/... ./pkg/coordinator/... ./pkg/runtime/... -run '^$'`
- `go test ./pkg/providers/kafka/... ./pkg/providers/mysql/... ./pkg/providers/mongo/... ./pkg/providers/postgres/... -run '^$'`
- `RECIPE_CLICKHOUSE_HTTP_PORT=8123 RECIPE_CLICKHOUSE_NATIVE_PORT=9000 RECIPE_CLICKHOUSE_PORT=9000 go test ./pkg/providers/clickhouse/... -run '^$'`

### Core smoke suites
- PASS: `make run-tests SUITE_PATH='pg2ch/replication'`
- FAIL (environment-precondition): `make run-tests SUITE_PATH='mysql2ch/replication'`
  - reason: required env vars (`RECIPE_MYSQL_*`, `RECIPE_CLICKHOUSE_*`) are not populated by this suite in local run path.
- PASS: `make run-tests SUITE_PATH='mongo2ch/snapshot'`
- PASS: `make run-tests SUITE_PATH='kafka2ch/replication'`

### Non-gate notes
- `go test ./cmd/...` fails in this environment due expected local PG endpoint absence (`localhost:6432`) in command-level integration tests.

## Files Added for Tracking
- `docs/upstream-core-cdc-import-list.md`
- `docs/upstream-core-cdc-import-report.md`
