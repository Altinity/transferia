# airbyte2ch optional suite

Status: blocked for local default runs.

Blocker:
- Deterministic Airbyte source fixture (connector image + state/config bootstrap) is not yet wired for E2E.

Required environment/images:
- Airbyte source connector image used by the test case.
- Fixture bootstrap for Airbyte config/state handshake.

Enable command after fixture implementation:
- `make test-layer-optional DB=airbyte2ch`
