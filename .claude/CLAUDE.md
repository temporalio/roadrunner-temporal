# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

RoadRunner Temporal Plugin - enables workflow and activity processing for PHP processes using Temporal. The plugin acts as a bridge between RoadRunner (Go) and Temporal SDK for PHP, handling communication via protobuf codec over goridge protocol.

## Build & Test Commands

### Go Tests
```bash
# Run all tests with race detection and coverage
go test -timeout 20m -v -race -cover -tags=debug -failfast ./...

# Run specific test suites
go test -timeout 20m -v -race -cover -tags=debug -failfast ./tests/general
go test -timeout 20m -v -race -cover -tags=debug -failfast ./canceller
go test -timeout 20m -v -race -cover -tags=debug -failfast ./dataconverter
go test -timeout 20m -v -race -cover -tags=debug -failfast ./queue

# Run with coverage profile
go test -timeout 20m -v -race -cover -tags=debug -failfast -coverpkg=$(cat pkgs.txt) -coverprofile=coverage.out -covermode=atomic ./general
```

### PHP Tests Setup
```bash
cd tests/php_test_files
composer install
```

### Linting
```bash
# Run golangci-lint with project config
golangci-lint run --timeout=10m --build-tags=safe
```

## Architecture

### Core Components

**Plugin Structure** (`plugin.go`):
- `Plugin` - main plugin struct, manages lifecycle and pools
- Implements RoadRunner plugin interface: `Init()`, `Serve()`, `Stop()`, `RPC()`
- Manages two worker pools: workflow pool (single worker) and activity pool (multiple workers)
- Uses mutex-protected `temporal` struct to hold client, workers, and definitions

**Worker Pools**:
- **Workflow Pool** (`wfP`): Single PHP worker dedicated to workflow execution
- **Activity Pool** (`actP`): Multiple PHP workers for concurrent activity execution
- Configured via `pool.Config` in config.go
- Uses `static_pool.Pool` from roadrunner-server/pool

**Communication Flow**:
1. Temporal SDK (Go) ↔ Plugin (`aggregatedpool/`) ↔ PHP Workers via codec
2. Protocol messages defined in `internal/protocol.go`
3. Codec implementation in `internal/codec/proto/`
4. Uses goridge for Go↔PHP communication

### Key Abstractions

**Workflow Definition** (`aggregatedpool/workflow.go`):
- Implements Temporal's `WorkflowDefinition` interface
- Handles workflow execution, local activities, queries, signals, updates
- Uses message queue for command/response exchange with PHP worker
- Maintains ID registry, cancellation context, and callback management

**Activity Definition** (`aggregatedpool/activity.go`):
- Implements Temporal's activity execution interface
- Routes activity invocations to PHP activity pool
- Handles activity context, headers, and heartbeats

**Protocol** (`internal/protocol.go`):
- Defines command constants (e.g., `invokeActivityCommand`, `startWorkflowCommand`)
- Message structure for Go↔PHP communication
- Context includes task queue, replay flag, history info

### Worker Lifecycle

1. **Initialization** (`internal.go:initPool()`):
   - Creates activity and workflow pools
   - Initializes codec and definitions
   - Retrieves worker info from PHP via protobuf
   - Creates Temporal client with interceptors
   - Starts Temporal workers

2. **Reset Flow** (`plugin.go:Reset()`, `ResetAP()`):
   - Triggered by worker stop events
   - Stops Temporal workers, resets pools
   - Purges sticky workflow cache
   - Re-initializes workers with fresh PHP processes
   - Workflow worker PID tracked for targeted resets

3. **Event Handling**:
   - Subscribes to `EventWorkerStopped` events
   - Checks PID in event message to determine WF vs Activity worker
   - Executes full reset for WF worker, activity-only reset for activity workers

### Configuration

**Structure** (`config.go`):
- `Address`: Temporal server address
- `Namespace`: Temporal namespace
- `CacheSize`: Sticky workflow cache size
- `Activities`: Pool configuration for activity workers
- `Metrics`: Optional Prometheus or StatsD metrics
- `TLS`: Optional TLS configuration
- `DisableActivityWorkers`: Flag to disable activity pool

**Environment Variables**:
- `RR_MODE=temporal` - set for PHP workers
- `RR_CODEC=protobuf` - codec identifier
- `NO_PROXY` - respected for gRPC connections

### Interceptors & Extensions

The plugin supports interceptors via `api.Interceptor` interface:
- Collected via Endure's dependency injection (`Collects()`)
- Applied to Temporal workers during initialization
- Stored in `temporal.interceptors` map

### Metrics

Two driver options:
- **Prometheus**: Exposed on configured address
- **StatsD**: Sent to statsd server with configurable prefix/tags

Metrics integrated via Temporal's `MetricsHandler` interface using uber-go/tally.

## Important Patterns

### Codec Usage
- All PHP communication uses protobuf codec (`internal/codec/proto/`)
- Wraps Temporal's data converter for payload serialization
- PHP SDK version extracted from worker info to set gRPC headers

### Worker Restart Strategy
- Workflow worker PID stored in `p.wwPID` for tracking
- Worker stop events include PID in message
- Targeted restarts based on PID matching prevent unnecessary full resets

### Client Headers
- gRPC interceptor rewrites client-name to "temporal-php-2"
- client-version set to PHP SDK version from worker info
- API key dynamically loaded from atomic pointer

### Pool Allocation
- Activity pool: configurable workers via `Activities.NumWorkers`
- Workflow pool: always 1 worker with 240h allocate timeout
- Both use same command/env but different pool configs

## File Organization

- `plugin.go`, `internal.go` - plugin lifecycle and initialization
- `config.go`, `tls.go`, `metrics.go` - configuration and setup
- `rpc.go` - RPC methods for external control
- `info.go`, `status.go` - worker and status information
- `aggregatedpool/` - workflow/activity definitions bridging Go↔Temporal
- `internal/` - protocol definitions and codec implementation
- `api/` - interfaces and context utilities
- `canceller/`, `queue/`, `registry/` - workflow execution utilities
- `dataconverter/` - Temporal data converter wrapper
- `tests/` - integration tests with PHP workers
