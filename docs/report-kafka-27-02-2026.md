# Kafka Provider Implementation Analysis

**Date**: 2026-02-27
**Branch**: `codex/ch-only-bloat-cleanup`

## 1. Offset Management

The Kafka provider uses **manual offset commits** with sophisticated sequencing:

### Key Design
- **Auto-commit disabled**: `kgo.DisableAutoCommit()` at `source.go:565`
- **Storage**: Kafka's internal `__consumer_offsets` topic (standard Kafka behavior)
- **Consumer Group ID**: Uses `transferID` as the group ID

### Offset Commit Flow
```
Message Fetched → Parse Queue → Sink Push → Ack Callback → Sequencer → Commit
```

1. Messages fetched via `PollRecords()` (`reader.go:27`)
2. Passed through parse queue for parallel processing
3. On successful push, `ack()` callback triggered (`source.go:262-287`)
4. **Sequencer** tracks in-flight offsets per partition (`sequencer.go:111-185`)
5. Only commits **contiguous ranges** - prevents gaps/partial commits
6. `CommitRecords()` called with safe offset (`reader.go:16-23`)

### Offset Policies
Configured via `OffsetPolicy` in `model_source.go:34-44`:
- `AtStartOffsetPolicy` - consume from beginning
- `AtEndOffsetPolicy` - consume from end (new messages only)
- Empty - resume from last committed offset

---

## 2. Multi-Threading & Multi-Instance Support

### Threading Model

| Layer | Parallelism | Configuration |
|-------|-------------|---------------|
| **Consumer Fetch** | Single thread | 1 franz-go client per Source |
| **Parse Queue** | Configurable | `ParseQueueParallelism` (default: 10, min: 2) |
| **Sink Write** | Configurable | `ParralelWriterCount` (default: 10) |

**Parse Queue** (`parsequeue.go:114-164`):
- Channel-based work distribution
- Semaphore-controlled parallelism
- Separate goroutines for push and ack loops

### Multi-Instance (Horizontal Scaling)

**Yes, fully supported via Kafka consumer groups:**

```go
kgo.ConsumerGroup(transferID)  // source.go:561
```

- Multiple instances with **same transferID** form a consumer group
- Kafka broker automatically assigns partitions across instances
- Rebalancing handled via `OnPartitionsRevoked` callback (`source.go:512-517`)
- `partitionReleased` flag triggers synchronization events

### Concurrency Controls
- `inflightMutex` - protects in-flight byte counter
- `pmx` - protects partition rebalance state
- `sync.Once` - ensures graceful shutdown
- Sequencer mutex - protects offset state machine

---

## 3. Library Versions

### Current vs Latest

| Library | Current | Latest | Gap |
|---------|---------|--------|-----|
| **twmb/franz-go** | v1.17.0 | **v1.20.7** | 3 minor versions behind |
| **segmentio/kafka-go** | v0.4.48 (patched) | **v0.4.50** | 2 patches behind |
| **confluent-kafka-go** | v2.1.1 | - | Schema Registry only |

**Note**: No librdkafka/CGO - all pure Go implementations.

### franz-go v1.20.7 Improvements Since v1.17.0
- Bug fixes and performance improvements
- Better client metrics
- kadm enhancements for internal topics

### kafka-go v0.4.50 Changes
- `v0.4.50` (Jan 2025): DescribeGroups v5 support
- `v0.4.49` (Aug 2024): Go 1.23, OffsetCommit improvements

---

## 4. Improvement Recommendations

### High Priority

1. **Upgrade franz-go to v1.20.7**
   - 3 minor versions behind
   - Bug fixes for client metrics
   - Better error handling

2. **Upgrade kafka-go to v0.4.50**
   - Remove vendor patch if possible (check what was patched)
   - DescribeGroups v5 support

### Medium Priority

3. **Configurable Fetch Parallelism**
   - Current: Single `PollRecords()` call
   - Could benefit from concurrent partition fetching for high-throughput scenarios

4. **Batch Size Tuning**
   - `FetchMaxBytes` hardcoded to 10MB (`source.go:562`)
   - Consider making configurable per use case

5. **Offset Commit Batching**
   - Currently commits after each ack
   - Could batch commits on timer for higher throughput (with at-least-once tradeoff)

### Low Priority

6. **Consumer Metrics Enhancement**
   - Add lag metrics per partition
   - Expose sequencer queue depth

7. **Cooperative Rebalancing**
   - Current: Uses default eager rebalancing
   - franz-go supports cooperative-sticky for smoother rebalances

8. **Connection Pool Tuning**
   - `ConnIdleTimeout` hardcoded to 30s
   - May need tuning for cloud environments

---

## 5. Architecture Summary

```
┌─────────────────────────────────────────────────────────────────┐
│                         Kafka Source                            │
├─────────────────────────────────────────────────────────────────┤
│  franz-go Client (v1.17.0)                                      │
│  ├── PollRecords() [single thread]                              │
│  ├── Consumer Group: transferID                                 │
│  └── Manual Commits via CommitRecords()                         │
├─────────────────────────────────────────────────────────────────┤
│  Parse Queue [parallel: ParseQueueParallelism]                  │
│  ├── pushCh → Parse goroutines → ackCh                          │
│  └── Semaphore-controlled parallelism                           │
├─────────────────────────────────────────────────────────────────┤
│  Sequencer [mutex-protected]                                    │
│  ├── Tracks in-flight offsets per partition                     │
│  ├── Ensures contiguous commit ranges                           │
│  └── Returns committable offset on Pushed()                     │
├─────────────────────────────────────────────────────────────────┤
│  Sink [parallel: ParralelWriterCount]                           │
│  └── kafka-go Writer (v0.4.48 patched)                          │
└─────────────────────────────────────────────────────────────────┘
```

---

## 6. Key Files Reference

| File | Purpose |
|------|---------|
| `pkg/providers/kafka/source.go` | Main consumer implementation |
| `pkg/providers/kafka/reader.go` | franz-go client wrapper |
| `pkg/providers/kafka/model_source.go` | Source configuration model |
| `pkg/providers/kafka/sink.go` | Producer/sink implementation |
| `pkg/providers/kafka/writer/writer_impl.go` | kafka-go writer wrapper |
| `pkg/util/queues/sequencer/sequencer.go` | Offset tracking state machine |
| `pkg/parsequeue/parsequeue.go` | Parallel parse queue |

---

## Sources
- [franz-go releases](https://github.com/twmb/franz-go/tags)
- [kafka-go releases](https://github.com/segmentio/kafka-go/releases)
