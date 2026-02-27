# Test Layout and Manual Execution

This repository keeps multiple test layers. Active product focus for unification:
`Postgres/MySQL/Mongo -> ClickHouse` for core, plus optional source suites.

## Source Families and Versions

Current supported source families and test variants:

| Family | Variants | Container Image |
|---|---|---|
| `postgres` | `17`, `18` | per-variant recipe image |
| `mysql` | `mysql84`, `mariadb118` | `mysql:8.4`, `mariadb:11.8` |
| `mongo` | `6`, `7` | per-variant recipe image |
| `kafka` | `confluent75`, `redpanda24` | per-variant recipe image |

`SOURCE_VARIANT` format:
- `family/variant`
- Examples: `mysql/mysql84`, `mysql/mariadb118`, `postgres/18`

## Layers

Flow layers (DB flow aliases):
- `tests/e2e/{pg2ch,mysql2ch,mongo2ch,kafka2ch}`
- `tests/e2e/{eventhub2ch,kinesis2ch,airbyte2ch,oracle2ch,ch2ch}` (optional flows)
- `tests/evolution/{pg2ch,mysql2ch,mongo2ch,kafka2ch}`
- `tests/resume/{pg2ch,mysql2ch,mongo2ch,kafka2ch}`
- `tests/large/{pg2ch,mysql2ch,mongo2ch,kafka2ch}`

Component layers (source adapters):
- `tests/storage/{postgres,mysql,mongo}`
- `tests/canon/{postgres,mysql,mongo}`

Shared infra:
- `tests/helpers`
- `tests/tcrecipes`

## Notes on Current State

`e2e` runs from `tests/e2e/<flow>` in the layered system.

Core parity scope is limited to:
- `pg2ch`
- `mysql2ch`
- `mongo2ch`

Optional source scope:
- `kafka2ch`
- `eventhub2ch`
- `kinesis2ch`
- `airbyte2ch`
- `oracle2ch`
- `ch2ch`

Deprecated/out-of-scope stacks are removed from this test layout.

## Manual Commands

- List supported layers and aliases:
  `make test-list`
- Run one layer for one DB:
  `make test-layer LAYER=e2e DB=pg2ch`
- Run one layer for all supported DBs:
  `make test-layer-all LAYER=resume`
- Run all layers for one DB:
  `make test-db DB=mysql2ch`
- Run local core gate for all supported DBs:
  `make test-core`
- Run all supported layers for all supported DBs:
  `make test-all-supported`
- Run one optional flow:
  `make test-layer-optional DB=kinesis2ch`
- Run full optional gate:
  `make test-cdc-optional`

## Source Variant Matrix (Manual)

`SOURCE_VARIANT` controls test source backend/image for matrix runs.

Examples by variant:
- `make test-source-variant SOURCE_VARIANT=postgres/18`
- `make test-source-variant SOURCE_VARIANT=mysql/mysql84`
- `make test-source-variant SOURCE_VARIANT=mysql/mariadb118`
- `make test-source-variant SOURCE_VARIANT=mongo/7`
- `make test-source-variant SOURCE_VARIANT=kafka/redpanda24`

Examples by family:
- `make test-source-family MATRIX_FAMILY=postgres`
- `make test-source-family MATRIX_FAMILY=mysql`
- `make test-source-family MATRIX_FAMILY=mongo`
- `make test-source-family MATRIX_FAMILY=kafka`

Run all configured variants:
- `make test-source-matrix`

Per-layer/per-DB with explicit variant:
- `SOURCE_VARIANT=mysql/mysql84 make test-layer LAYER=e2e DB=mysql2ch`
- `SOURCE_VARIANT=mysql/mariadb118 make test-layer LAYER=resume DB=mysql2ch`
- `SOURCE_VARIANT=mysql/mysql84 make test-db DB=mysql2ch`

Matrix definition file:
- `tests/e2e/matrix/sources.yaml`

Core matrix contract/report:
- `tests/e2e/matrix/core2ch.yaml`
- `tests/e2e/matrix/coverage_report.md`

## Resume Layer Behavior

Resume tests are executed with a test-name filter (`ResumeFromCoordinator|Resume`).

Core resume suites are now defined for:
- `tests/resume/pg2ch/replication`
- `tests/resume/mysql2ch/replication`
- `tests/resume/mongo2ch/snapshot`
- `tests/resume/mongo2ch/snapshot_flatten`
- `tests/resume/kafka2ch/replication`

## Stable Compare Fallback

`tests/helpers/compare_storages.go` supports deterministic fallback when checksum
comparison flakes on ordering differences.

- `StableFallback` defaults to `false`
- `StableRowLimit` defaults to `10000`
- `DebugSampleRows` defaults to `20`

Enable explicitly per test:
- `helpers.NewCompareStorageParams().WithStableFallback(true)`

Fallback behavior:
- first tries checksum compare;
- on checksum error and `StableFallback=true`, compares deterministically sorted
  rows by key with existing priority comparators;
- emits compact mismatch diagnostics including table/key/column samples.

## Core2CH Matrix Commands

- Generate and enforce wave-1 parity report:
  `make test-matrix-gap-report`
- Run all required wave-1 matrix suites:
  `make test-matrix-core`
- Run explicit wave:
  `make test-matrix-wave1`
  `make test-matrix-wave2`

## Full Local CDC Suite Gate (Authoritative)

Strict local core gate for the in-scope product surface:
- sources: `postgres`, `mysql/mariadb`, `mongo`
- destination: `clickhouse`
- layers: `providers`, `storage-canon`, `e2e`, `evolution`, `resume`, `large`

Wave definitions:

| Wave | What it runs | Goal |
|---|---|---|
| `providers` | package-level provider tests (`pkg/providers/...`) + shared test infra checks | catch adapter/runtime regressions early |
| `storage-canon` | `tests/storage/*` and `tests/canon/*` | validate storage/canonical compare correctness |
| `e2e` | core flow e2e suites for `pg2ch/mysql2ch/mongo2ch` | verify end-to-end data movement works |
| `evolution` | `tests/evolution/*` | verify schema/type evolution behavior |
| `resume` | `tests/resume/*` | verify checkpoint restore and restart semantics |
| `large` | `tests/large/*` | verify larger-volume and batching stability |

Wave execution details:
- `test-cdc-full` runs waves in this order:
  1. `providers`
  2. `storage-canon`
  3. `e2e`
  4. `evolution`
  5. `resume`
  6. `large`
- `resume` wave runs once with default coordinator backend for the active scope.

Manifest and helper:
- `tests/e2e/matrix/cdc_local_suite.yaml`
- `tests/e2e/matrix/cdc_optional_suite.yaml`
- `go run ./tools/testmatrix suite ...` (invoked by Makefile targets)

Primary commands:
- Show exact allowlist (waves, suites, packages, variants):
  `make test-cdc-list`
- Verify required suites are not empty:
  `make test-cdc-verify`
- Run one wave (fail-fast):
  `make test-cdc-wave WAVE=providers`
  `make test-cdc-wave WAVE=storage-canon`
  `make test-cdc-wave WAVE=e2e`
  `make test-cdc-wave WAVE=evolution`
  `make test-cdc-wave WAVE=resume`
  `make test-cdc-wave WAVE=large`
- Run full source-variant matrix:
  `make test-cdc-matrix`
  `make test-cdc-matrix SOURCE_VARIANT=postgres/18`
- Run complete strict local gate:
  `make test-cdc-full`
  (`test-cdc-full` does not run matrix; run matrix separately)
- Show optional allowlist:
  `make test-cdc-optional-list`
- Verify optional suites are not empty:
  `make test-cdc-optional-verify`
- Run optional gate:
  `make test-cdc-optional`
- Run one optional wave:
  `make test-cdc-optional-wave WAVE=optional-queues`

Wave pass-state cache:
- Cache directory: `.teststate/waves`
- Cache mechanism: native make dependencies + timestamped `.ok` stamp per wave.
- A wave reruns when any dependency in its scope is newer than `.teststate/waves/<wave>.ok`.
- Dependency scope for invalidation:
  - shared: `library`, `pkg`, `vendor_patched`, `tools/testmatrix`
  - wave-specific:
    - `providers`: `tests/helpers`, `tests/tcrecipes`
    - `storage-canon`: `tests/storage`, `tests/canon`
    - `e2e`: `tests/e2e`
    - `evolution`: `tests/evolution`
    - `resume`: `tests/resume`
    - `large`: `tests/large`
  - control files: `Makefile`, `go.mod`, `go.sum`, matrix manifest/contract
- List cached waves:
  `make test-state-list`
- Clear one wave cache:
  `make test-state-clear WAVE=resume`
- Clear all cache:
  `make test-state-clear-all`
- Bypass cache for one run:
  `make test-cdc-wave WAVE=providers FORCE=1`
  `make test-cdc-full FORCE=1`

Strict-mode diagnostics (disable gotestsum retry):
- `make test-cdc-wave WAVE=providers RERUN_FAILS=0 FORCE=1`
- `make test-cdc-full RERUN_FAILS=0 FORCE=1`
- `make test-cdc-matrix RERUN_FAILS=0 FORCE=1`

Matrix pass-state cache:
- Cache directory: `.teststate/matrix`
- Cache key unit: one `.ok` stamp per `SOURCE_VARIANT`
- List matrix cache:
  `make test-state-matrix-list`
- Clear one matrix variant cache:
  `make test-state-matrix-clear SOURCE_VARIANT=postgres/18`
- Clear all matrix cache:
  `make test-state-matrix-clear-all`
- Bypass matrix cache for one run:
  `make test-cdc-matrix FORCE=1`

## Optional CDC Suite Gate

Optional flows are tracked outside core parity and run with a separate gate.

Optional waves:
1. `optional-queues`: `kafka2ch`, `eventhub2ch`, `kinesis2ch`
2. `optional-connectors`: `airbyte2ch`, `oracle2ch`
3. `optional-clickhouse-source`: `ch2ch`

Primary commands:
- `make test-cdc-optional-list`
- `make test-cdc-optional-verify`
- `make test-cdc-optional-wave WAVE=optional-queues`
- `make test-cdc-optional`

Optional cache:
- Cache directory: `.teststate/waves-optional`
- List optional cached waves:
  `make test-state-optional-list`
- Clear one optional wave cache:
  `make test-state-optional-clear WAVE=optional-queues`
- Clear all optional cache:
  `make test-state-optional-clear-all`
- Bypass optional cache for one run:
  `make test-cdc-optional FORCE=1`

Blocked optional suites:
- `eventhub2ch`, `airbyte2ch`, `oracle2ch` currently provide smoke placeholders with explicit `t.Skip(...)`.
- See:
  - `tests/e2e/eventhub2ch/README.md`
  - `tests/e2e/airbyte2ch/README.md`
  - `tests/e2e/oracle2ch/README.md`

## Recent Behavior Change (MySQL -> ClickHouse)

- MySQL recipe init loader now resolves SQL init scripts by provider-specific subdirectory (`dump/mysql`) before fallback.
