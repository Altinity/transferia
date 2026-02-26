# Core2CH Coverage Report

- Generated: `2026-02-25 13:20:38Z`
- Matrix: `tests/e2e-core/matrix/core2ch.yaml`
- Wave: `1`
- Coverage model: `Tiered Core+Extensions`

## YDB/YT Inventory

- Total ydb/yt related tests: `125`
- `*2yt`: `82`
- `*2ydb`: `20`
- `ydb2*`: `28`
- `yt2*`: `15`

## Coverage by Source

| Source | Required | Covered | Missing |
|---|---:|---:|---:|
| `kafka2ch` | 12 | 12 | 0 |
| `mongo2ch` | 12 | 12 | 0 |
| `mysql2ch` | 12 | 12 | 0 |
| `pg2ch` | 12 | 12 | 0 |
| **Total** | **48** | **48** | **0** |

## Missing Mandatory Coverage

- None

## Extensions (Excluded from Core Parity)

- `ch_async`
  - `tests/e2e/yt2ch_async/**`
  - `pkg/providers/clickhouse/async/**`
  - `pkg/providers/clickhouse/tests/async/**`
- `s3_sink`
  - `tests/e2e/*2s3/**`
- `source_specific`
  - `tests/e2e/pg2yt/**`
  - `tests/e2e/mysql2yt/**`
  - `tests/e2e/mongo2yt/**`
- `ydb_sink`
  - `tests/e2e/*2ydb/**`
  - `tests/e2e/ydb2*/**`
- `yt_sink`
  - `tests/e2e/*2yt/**`
  - `tests/e2e/yt2*/**`
