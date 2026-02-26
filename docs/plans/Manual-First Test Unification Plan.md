# Manual-First Test Unification Plan (Integrated Next Steps)

## Summary
Integrate the 3 immediate actions into the active plan:
1. Install `gotestsum` and run `make test-core` as the baseline gate.
2. Start real migration of `tests/large/docker-compose` into `tests/large/{pg2ch,mysql2ch,mongo2ch}`.
3. Add first dedicated `evolution` scenarios for `mysql2ch` and `mongo2ch`.

This remains **manual/Makefile-driven**; CI changes stay in the separate deferred plan.

## Public Interfaces / Contracts
- Keep Makefile test API as the stable interface:
  - `make test-list`
  - `make test-layer LAYER=<storage|canon|e2e-core|evolution|resume|large> DB=<pg2ch|mysql2ch|mongo2ch>`
  - `make test-layer-all LAYER=<...>`
  - `make test-db DB=<...>`
  - `make test-core`
  - `make test-all-supported`
  - `make test-resume-s3 DB=<...>`
- Scenario metadata contract (documented and enforced in naming/docs):
  - `layer`, `db`, `scenario_id`, `requires_s3_coordinator`, `expected_delta_only`

## Phase Plan

### Phase 1: Baseline Tooling + Core Gate
1. Install tool locally:
   - `go install gotest.tools/gotestsum@latest`
2. Validate command availability:
   - `gotestsum --version`
3. Run baseline supported gate:
   - `make test-core`
4. Capture results by layer/DB and classify failures:
   - infra/setup
   - test bug
   - product bug (open GitHub issue with repro)

### Phase 2: Large Layer Real Migration
1. Inventory current `tests/large/docker-compose` tests and map to DB ownership:
   - `pg2ch`: postgres-origin heavy cases
   - `mysql2ch`: mysql-origin heavy cases
   - `mongo2ch`: mongo-origin heavy cases
   - non-target flows -> `tests/legacy` mapping list
2. Move first batch (not placeholders) into:
   - `tests/large/pg2ch/<scenario>`
   - `tests/large/mysql2ch/<scenario>`
   - `tests/large/mongo2ch/<scenario>`
3. Ensure each moved suite runs via:
   - `make test-layer LAYER=large DB=<...>`
4. Keep old path compatibility only until all references are updated; then remove transitional links.

### Phase 3: First Dedicated Evolution Scenarios (MySQL + Mongo)
1. Add `mysql2ch` evolution scenario set:
   - `add_column_nullable`
   - `add_column_with_default`
   - `type_widening_safe` (where supported)
2. Add `mongo2ch` evolution scenario set:
   - `new_field_appears_in_documents`
   - `nested_field_shape_change`
   - `flatten_mode_schema_change`
3. Place under:
   - `tests/evolution/mysql2ch/<scenario>`
   - `tests/evolution/mongo2ch/<scenario>`
4. Add deterministic fixtures and assertions:
   - source mutation script
   - sink schema/value checks
   - replay stability check (no duplicate side effects on restart)

### Phase 4: Resume/S3 Hardening During Rollout
1. For each new evolution/large scenario touching restart behavior, run:
   - `make test-resume-s3 DB=<...>`
2. Enforce resume assertions:
   - checkpoint restored
   - second run consumes only delta
   - no duplicates in ClickHouse

### Phase 5: Documentation + Tracking
1. Update `/Users/bvt/work/transferia/tests/README.md` with:
   - moved large scenarios table
   - new evolution scenarios matrix
   - exact run commands per layer/DB
2. Maintain bug tracker section in `docs/plans/test-unification-manual.md`:
   - scenario ID
   - status (pass/fail/known-bug)
   - issue link if product defect

## Test Cases and Acceptance Criteria

### Acceptance for Phase 1
- `gotestsum` installed and executable.
- `make test-core` runs end-to-end (pass or produces classified failures).

### Acceptance for Phase 2
- At least one real large scenario migrated and runnable for each DB:
  - `pg2ch`, `mysql2ch`, `mongo2ch`
- `make test-layer LAYER=large DB=<...>` executes real tests (not empty dir only).

### Acceptance for Phase 3
- At least 3 dedicated evolution scenarios implemented for `mysql2ch`.
- At least 3 dedicated evolution scenarios implemented for `mongo2ch`.
- All new scenarios runnable by `make test-layer LAYER=evolution DB=<...>`.

### Acceptance for Phase 4
- Resume S3 checks pass for all 3 DBs on at least one scenario each.

## Assumptions and Defaults
- CI workflow files are out of scope in this phase.
- Active product scope is only `Postgres/MySQL/Mongo -> ClickHouse`.
- Product bugs discovered during test rollout are tracked via GitHub issues with minimal repro; tests remain as regression coverage.
- Transitional links are allowed short-term, but end-state is real per-layer directories with no hidden dependency on old layout.
