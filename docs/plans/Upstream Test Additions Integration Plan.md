# Upstream Test Additions Integration Plan (Manual-First, Layered)

## Summary
Use upstream `transferia/transferia` test work as input to strengthen your new layered system, with priority on:
1. coordinator resume coverage for `pg2ch/mysql2ch/mongo2ch`,
2. anti-flake stability improvements in storage comparison,
3. S3 coordinator regression unit tests.

Key upstream references to integrate:
- [46c67a7](https://github.com/transferia/transferia/commit/46c67a75d893d5fef9d5ec7346d412dadf6d3f07) (`mysql2ch` resume tests + coordinator backend helper)
- [6e9e7dd](https://github.com/transferia/transferia/commit/6e9e7dd231c352cf1943a2f9be634dd06b7f5950) (`mongo2ch` resume tests)
- [2bcc98e](https://github.com/transferia/transferia/commit/2bcc98e6f7e7e5e7227fb76ea622538afaf7fc67) (sorted compare / anti-flake in `pg2ch`)
- [c3811c6](https://github.com/transferia/transferia/commit/c3811c6ec08509ff4f1a6da8f2426c95f5c89b99) (better replication test diagnostics)
- [881a8ac](https://github.com/transferia/transferia/commit/881a8ac12b3f39f8e97f38911f6a4a2be1abe7f5) (S3 coordinator `oldKeys` regression coverage)
- Optional parser hardening: [c0f6f39](https://github.com/transferia/transferia/commit/c0f6f3947a54e38c9849f8f6a4877e8c8773766a)

## Important Changes to Interfaces / Types / Test Contracts
- Standardize coordinator backend contract in tests:
  - `COORDINATOR_BACKEND=fake|s3`
  - shared helper entrypoint for transfer-scoped coordinator creation/reset.
- Standardize resume scenario contract metadata:
  - `layer`, `db`, `scenario_id`, `requires_s3_coordinator`, `expected_delta_only`.
- Standardize stable data assertions:
  - prefer sorted storage compare path where order is non-deterministic.
- Keep Makefile as the only orchestration API for now (no CI workflow change).

## Implementation Plan

### Phase 1: Upstream parity intake (targeted)
1. Compare current local test files with upstream commit deltas above.
2. Build a “parity checklist” per commit and mark each hunk as:
   - already present,
   - missing and required,
   - intentionally not adopted.
3. Apply missing parity only for supported scope (`pg2ch/mysql2ch/mongo2ch -> clickhouse`).

### Phase 2: Layer placement and normalization
1. Place/adapt upstream resume tests into `tests/resume/{pg2ch,mysql2ch,mongo2ch}`.
2. Keep `tests/e2e` compatibility until imports are decoupled; use alias/symlink strategy during migration.
3. Ensure `tests/storage/{postgres,mysql,mongo}` and `tests/canon/{postgres,mysql,mongo}` remain source-oriented.

### Phase 3: Stability hardening
1. Integrate sorted compare and row-count wait patterns from upstream `pg2ch` anti-flake changes.
2. Add explicit logging patterns from upstream replication diagnostics where flaky behavior was observed.
3. Define “stable assertion rules” in `tests/README.md` (when to use sorted compare vs strict compare).

### Phase 4: S3 coordinator regression protection
1. Add/port unit tests for S3 coordinator key/old-keys behavior from upstream.
2. Map these tests to resume-layer acceptance so coordinator regressions fail early even before e2e.
3. Require at least one S3-backed resume smoke per supported DB in manual core gate.

### Phase 5: Optional parser/canon expansion
1. Add pg_dump parser edge tests (empty-schema and similar) if parser remains in supported path.
2. Keep parser additions non-blocking to `pg/mysql/mongo -> ch` core gate initially.

## Test Cases and Scenarios

### Mandatory core (all 3 DBs)
- `snapshot_basic`
- `replication_basic`
- `snapshot_plus_replication`
- `resume_second_run_no_duplicates`
- `resume_delta_only`

### Mandatory S3 coordinator checks
- checkpoint restore after restart
- no replay from stale old keys
- state reset behavior correctness on fresh transfer ID

### Stability checks
- sorted compare for unordered sinks
- row-count convergence before deep compare
- improved failure diagnostics for fast triage

## Bug Handling During Test Implementation
- If a new case fails, classify as test bug vs product bug using minimal repro.
- For product bug:
  - keep regression test (or mark known-failing with bug ID),
  - open GitHub issue with reproducible script/data + expected/actual + logs/checkpoint evidence.
- Do not relax assertions to hide product defects.

## Assumptions and Defaults
- CI remains unchanged in this phase (manual/Makefile-driven).
- Active scope is only `Postgres/MySQL/Mongo -> ClickHouse`.
- Upstream parity is selective: only commits affecting supported flows/coordinator correctness are mandatory.
- S3 coordinator behavior is treated as production-critical and therefore mandatory in resume validation.
