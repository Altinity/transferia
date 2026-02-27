# AGENTS.md - AI Agent & Contributor Guidelines

This document provides essential context for AI agents (Claude, Copilot, etc.) and human contributors working on the Transferia codebase.

## Project Overview

**Transferia** is an open-source, cloud-native ELT (Extract, Load, Transform) ingestion engine built in Go. It enables seamless, high-performance data movement between diverse database systems at scale.

### Key Capabilities
- **Snapshot**: One-time bulk data transfer with table-level consistency
- **Replication**: Continuous CDC (Change Data Capture) streaming
- **Transformations**: Row-level transformations during transfer (rename, mask, filter, SQL)
- **Multi-source**: PostgreSQL, MySQL, MongoDB, Kafka, S3, and more
- **Multi-destination**: ClickHouse, PostgreSQL, Kafka, S3, and more

## Repository Structure

```
transferia/
├── cmd/trcli/          # CLI entry point (replicate, upload, check, validate)
├── pkg/
│   ├── abstract/       # Core interfaces (Source, Sink, Storage, Transfer)
│   ├── providers/      # Database adapters (postgres, mysql, mongo, clickhouse, kafka)
│   ├── dataplane/      # Runtime execution engine
│   ├── middlewares/    # Cross-cutting concerns (retry, filter, metrics)
│   ├── transformer/    # Pluggable transformers
│   ├── parsers/        # Data format parsers (JSON, Avro, Parquet)
│   ├── coordinator/    # Multi-node coordination (memory, S3)
│   └── connection/     # Connection management
├── internal/           # Internal packages (logger, config, metrics)
├── tests/
│   ├── e2e/            # End-to-end tests (pg2ch, mysql2ch, mongo2ch)
│   ├── helpers/        # Test utilities and helpers
│   ├── canon/          # Type/schema validation tests
│   └── storage/        # Provider storage tests
├── recipe/             # Test container recipes
├── examples/           # Configuration examples
└── docs/               # Documentation
```

## Core Abstractions

### Key Interfaces (pkg/abstract/)

1. **Storage** - One-time data reader for snapshots
2. **Source** - Streaming data reader for CDC replication
3. **Sink** - Data writer with async push semantics
4. **Transformer** - Row-level data transformation
5. **ChangeItem** - Core unit of data transfer (represents a row operation)

### Provider Pattern

All providers in `pkg/providers/` follow this structure:
```go
type Provider struct {
    logger   log.Logger
    registry metrics.Registry
    cp       coordinator.Coordinator
    transfer *model.Transfer
}
```

Providers register via `init()` with:
- `providers.Register(ProviderType, New)`
- `model.RegisterSource/RegisterDestination`
- `abstract.RegisterProviderName`

## Coding Guidelines

### Error Handling

- Use `xerrors.Errorf("context: %w", err)` for wrapping
- Domain-specific error types exist:
  - `FatalError` - Stops transfer, forbids restart
  - `RetriablePartUploadError` - Transient, eligible for retry
  - `TableUploadError` - Specific upload failures
- Never ignore errors silently; log if not returning

### Logging

- Use structured logging via the `logger` package (Zap-based)
- Prefer `logger.Log.Info("message", log.String("key", value))` over `Infof`
- Log levels: DEBUG, INFO, WARNING, ERROR, FATAL
- Never log credentials or sensitive data

### Testing

- Place tests in `tests/` directory, not alongside code
- Use testcontainers via `recipe/` package for integration tests
- Follow the pattern: `TestSnapshotAndIncrement`, `TestReplication`
- Use helpers: `helpers.Activate()`, `helpers.CompareStorages()`
- Wait helpers for async operations: `WaitEqualRowsCount()`, `WaitCond()`

### Concurrency

- Always use context for cancellation
- Use `sync.WaitGroup` for goroutine lifecycle
- Prefer buffered channels to avoid deadlocks
- Use `sync.Once` for one-time cleanup operations
- Avoid mutex in hot paths; consider atomics

## Security Considerations

### Critical Rules

1. **Never hardcode credentials** - Use environment variables
2. **Always validate TLS certificates** - Don't set `InsecureSkipVerify: true` in production
3. **Sanitize SQL inputs** - Use parameterized queries, never string concatenation
4. **Redact secrets in logs** - Never log passwords, tokens, or keys
5. **Validate database filters** - User-provided filters can be injection vectors

### Known Security Debt

- Some TLS configurations default to `InsecureSkipVerify` when no cert provided
- `SecretString` type alias provides no actual protection
- Test credentials exist in recipe files (acceptable for tests only)

## Provider-Specific Notes

### PostgreSQL
- Most complex provider with full replication support
- Uses pgx library with custom type mapping
- Has DBLog support for alternative loading
- System tables: `__consumer_keeper`, `__data_transfer_lsn`

### MySQL
- Supports both file-based and GTID position tracking
- Character set handling (UTF-8MB3 vs MB4)
- System tables: `__table_transfer_progress`, `__tm_keeper`

### MongoDB
- Simpler, document-oriented provider
- Uses cluster time instead of LSN for position
- System collection: `__dt_cluster_time`

### ClickHouse
- **Snapshot-only** - No replication source support
- Implements different interfaces (`Abstract2Provider`, `AsyncSinker`)
- HTTP and Native protocol support

## Common Tasks

### Adding a New Provider

1. Create package in `pkg/providers/newprovider/`
2. Implement required interfaces (Storage, Source, Sink as needed)
3. Register in `init()` function
4. Add test recipes in `recipe/`
5. Create e2e tests in `tests/e2e/`

### Adding a Transformer

1. Add implementation in `pkg/transformer/registry/`
2. Register with transformer registry
3. Update configuration model if needed
4. Add tests

### Running Tests

```bash
# Quick core tests
make test-core

# Full CDC test suite
make test-cdc-full

# Specific wave
make test-cdc-wave WAVE=providers

# Specific layer
make test-layer LAYER=e2e DB=pg2ch
```

## Build Commands

```bash
make build          # Build trcli binary
make docker         # Build Docker image
make clean          # Remove artifacts
make lint           # Run linters
```

## Important Files

- `pkg/abstract/model/transfer.go` - Transfer model definition
- `pkg/abstract/endpoint.go` - Provider interface definitions
- `pkg/providers/provider.go` - Provider registration
- `cmd/trcli/config/model.go` - CLI configuration model
- `.golangci.yml` - Linter configuration

## Code Style

- Follow Go idioms and effective Go guidelines
- Use `gofmt` and linters (`.golangci.yml` configured)
- Interface names: clear verbs (Source, Sink, Transformer)
- File names: lowercase with underscores
- Keep functions focused; extract when > 50 lines
- Comment non-obvious logic; skip obvious comments

## Architecture Decisions

1. **Compile-time plugins** - No runtime plugin loading; all providers compiled in
2. **Middleware pattern** - Cross-cutting concerns via composable middlewares
3. **Marker interfaces** - Capability detection via `Is*()` marker methods
4. **Coordinator abstraction** - Memory (single-node) or S3 (distributed) coordination
5. **Wave-based testing** - Tests organized in dependency waves for efficient CI

## Getting Help

- See `/docs/` for detailed documentation
- Check `/examples/` for configuration patterns
- Review existing provider implementations for patterns
- Test recipes in `/recipe/` show infrastructure setup
