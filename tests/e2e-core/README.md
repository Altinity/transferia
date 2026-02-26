# e2e-core layer

Core flow validation for supported paths:
- `pg2ch`
- `mysql2ch`
- `mongo2ch`
- `kafka2ch`

## Source Variant Matrix

Source compatibility matrix is defined in:
- `tests/e2e-core/matrix/sources.yaml`
- `tests/e2e-core/matrix/core2ch.yaml`

Manual run entrypoint for a specific source variant:
- `make test-source-variant SOURCE_VARIANT=postgres/18`

Core parity commands:
- `make test-matrix-gap-report`
- `make test-matrix-core`
