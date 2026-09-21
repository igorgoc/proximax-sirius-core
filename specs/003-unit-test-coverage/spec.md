# Feature Specification: Complete Go Backend Test Coverage

**Feature Branch**: `003-unit-test-coverage`

**Created**: 2026-09-17

**Status**: Ready for Planning

**Input**: Add comprehensive unit test suites for pkg/migrator, pkg/network, and pkg/storage to achieve 100% package test coverage across the Go backend.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Storage Migration Robustness & Error Handling (Priority: P1)

As a node operator upgrading legacy database directories, I want automated unit tests verifying the legacy flat-file to chunk-binary migrator so that block data conversions and cancellation states execute reliably without corrupting chain history.

**Why this priority**: Block data integrity is essential for consensus synchronization.

**Independent Test**: Run `go test -v ./pkg/migrator/...` and verify conversion, cancellation, index binary offsets, and empty directory handling.

**Acceptance Scenarios**:

1. **Given** a directory with mock loose `.dat` and `.stmt` files, **When** `StartMigration` runs, **Then** `blocks.dat`, `statements.dat`, and `blocks.idx` are written with exact little-endian binary offsets and loose files are deleted.
2. **Given** an active migration, **When** `Cancel()` is called, **Then** the migrator terminates gracefully with status `cancelled`.

---

### User Story 2 - Replicator Storage Configuration & Metrics Testing (Priority: P1)

As a Sirius storage node provider, I want unit test coverage over `pkg/storage` configuration parsing, metrics calculation, and mock account derivation so that storage paths, sandboxes, and key masking invariants behave deterministically.

**Why this priority**: Guarantees configuration persistence and protects private key masking invariants in API responses.

**Independent Test**: Run `go test -v ./pkg/storage/...` to verify properties loading, path resolution, sandboxes cleanup, and key masking.

**Acceptance Scenarios**:

1. **Given** a mock `config-storage.properties` with a 64-character private key, **When** `GetStatus()` is called, **Then** `PublicKey` and `Address` are derived, and `Key` is stripped/masked in the response.
2. **Given** temporary sandbox files in storage, **When** `CleanSandboxes()` is executed, **Then** uncommitted temporary folders are removed and freed byte counts are accurate.

---

### User Story 3 - Network & Port Reachability Testing (Priority: P2)

As an operator running in NAT or firewall environments, I want unit tests for `pkg/network` checking IP detection fallbacks and port reachability helpers.

**Why this priority**: Ensures network diagnostics do not crash or block supervisor startup when offline or behind firewalls.

**Independent Test**: Run `go test -v ./pkg/network/...`.

**Acceptance Scenarios**:

1. **Given** a mock local environment, **When** port checks and outbound IP helpers are invoked, **Then** results are returned within timeouts without blocking.

---

### Edge Cases

- What happens if the data directory has invalid file names during migration?
  - The migrator skips non-integer `.dat` files and does not crash.
- What happens if `config-storage.properties` is missing?
  - `LoadStorageConfig` initializes sensible defaults and creates the file from template if available.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide unit tests for `backend/pkg/migrator/migrator.go` covering `GetStatus`, `Cancel`, `processDirectory`, binary index writing, and error cases.
- **FR-002**: System MUST provide unit tests for `backend/pkg/storage/storage.go` covering `LoadStorageConfig`, `SaveStorageConfig`, `GetStorageMetrics`, `CleanSandboxes`, and `GetStatus` (verifying key masking).
- **FR-003**: System MUST provide unit tests for `backend/pkg/network/upnp.go` covering `PortCheckResult`, reachability queries, and thread safety.
- **FR-004**: System MUST ensure `go test ./...` in `backend/` passes with 0 failures and 0 untested packages.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of active backend Go packages report `ok` in `go test ./...` (zero `[no test files]`).
- **SC-002**: All new test suites run in under 3 seconds in aggregate.
- **SC-003**: Zero race conditions detected under `go test -race ./...`.
