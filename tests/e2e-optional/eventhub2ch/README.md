# eventhub2ch optional suite

Status: blocked for local default runs.

Blocker:
- End-to-end EventHub -> ClickHouse replication fixture is not yet wired in `tests/e2e-optional/eventhub2ch/replication`.

Required environment/images:
- EventHub emulator image (for example: `mcr.microsoft.com/azure-messaging/eventhubs-emulator:latest`).
- Working transport wiring from EventHub source recipe to transfer runtime.

Enable command after fixture implementation:
- `make test-layer-optional DB=eventhub2ch`
