# oracle2ch optional suite

Status: blocked for local default runs.

Blocker:
- Oracle source test recipe is not yet wired for deterministic E2E execution.

Required environment/images:
- Oracle database container/image suitable for automated testing.
- Oracle initialization scripts and connection bootstrap in test recipes.

Enable command after fixture implementation:
- `make test-layer-optional DB=oracle2ch`
