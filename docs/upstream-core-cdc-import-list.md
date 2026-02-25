# Upstream Core CDC Import List

Source branch: `upstream/main`
Target branch: `codex/upstream-main-core-cdc-merge`
Base: `4865401f`

## Import Now
- 72d663f9 MySQL IPv6 connector fix
- 57c28280 MySQL replication key/schema handling improvements
- d9c85803 PG invalid replication slot fatal handling
- c7f81d64 PG WAL TOAST update fill fix
- a609bcd2 PG partition replication collapse fix
- e081b290 Kafka partition key builder fix (no cluster)
- 75ed9994 Kafka topic check fatal reduction
- 20846311 Kafka reader/per-partition architecture
- 51b23c57 Kafka per-partition + consumer-group improvements
- a86ec0f0 CH insert null-as-default by version
- 46414b0e CH type checks for unsupported composite types
- f5dc0fe8 CH column type alteration in schema migration
- 3432ce46 Remove AddNewColumns option usage
- b1a95070 Datatype normalization behavior updates
- 4385f970 In-memory coordinator worker index fix
- 4d2d0255 S3 operation-table parts matching fix
- 637857e1 Snapshot stage/part-id/shared-memory updates
- 34ba65b1 Transfer operation created-at/runtime wiring
- d0bcc1d1 Callback key API runID
- 41de8622 Snapshot retry model/config
- 53b35bc1 Endpoint defaulting strategy changes
- bc166125 TLS tri-state (*bool) model changes
- 60356947 Debezium secret sanitization

## Deferred
- fbd30580 PG sink inline bulk insert (postgres sink path)
- 427dbf6a pg_dump test ordering/filtering
- dc6a632e pg_dump test ordering/filtering follow-up
- 8cc66756 MySQL datetime fallback
- aee67ebe Revert MySQL datetime fallback

## Expected Touched Areas
- `pkg/providers/postgres/**`
- `pkg/providers/mysql/**`
- `pkg/providers/mongo/**` (callsite compat)
- `pkg/providers/kafka/**`
- `pkg/providers/clickhouse/**`
- `pkg/coordinator/**`
- `pkg/runtime/**`
- `pkg/abstract/**`
- `cmd/trcli/**` (config/model wiring)
