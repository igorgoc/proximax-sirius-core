# Feature Specification: Codebase Cleanup and Dead Artifact Removal

**Feature Branch**: `001-cleanup-dead-code`

**Created**: 2026-09-17

**Status**: Ready for Planning

**Input**: Clean up dead code, obsolete scripts, and unused files across proximax-sirius-core-native.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Developer/Operator Repository Hygiene & Build Reliability (Priority: P1)

As a Sirius peer node developer or node operator, I want all obsolete prototype scripts and superseded CLI commands removed from the codebase so that the repository contains only production-active, verified code and packaging scripts.

**Why this priority**: Eliminates developer confusion, reduces binary maintenance surface, and prevents inadvertent execution of outdated Docker-era scripts on native installations.

**Independent Test**: Build all packages and run tests (`go test ./...` and `npm run build`); verify zero build errors, zero broken references, and zero orphan binaries.

**Acceptance Scenarios**:

1. **Given** the repository with legacy CLI tools (`fast-extract`, `stream-chunk-restore`), **When** codebase cleanup is performed, **Then** all legacy CLI tools are removed and `backend/pkg/snapshot` continues to handle native streaming restores with 100% test pass.
2. **Given** the repository with obsolete scripts (`scripts/common/audit_chunks.py`, `scripts/linux/fast-snapshot.sh`, `scripts/linux/test_*_persistence.sh`), **When** cleanup is performed, **Then** all obsolete scripts are deleted without impacting production launchers (`start.sh`, `run.sh`, `package-*.sh`, `setup-wsl.*`).

---

### User Story 2 - Automated Verification & Test Completeness (Priority: P2)

As a maintainer, I want the codebase to have active automated test passes for remaining packages so that future refactorings do not cause regressions.

**Why this priority**: Ensures that removing dead files causes no regressions in active supervisor, config, or storage routines.

**Independent Test**: Run `go test ./...` and verify all tests pass across all packages.

**Acceptance Scenarios**:

1. **Given** active Go backend packages, **When** tests are executed, **Then** all active packages pass with zero test failures.

---

### Edge Cases

- What happens when a user or CI script looks for an obsolete CLI tool?
  - All standard packaging scripts (`package-macos.sh`, `package-linux.sh`, `package-windows.ps1`) must be verified to ensure they only reference production binaries (`sirius-core` and native engine binaries).
- What happens if an external documentation file references a legacy tool?
  - Internal markdown docs should be checked and updated if they cite deprecated prototype commands.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST remove obsolete standalone Go CLI commands:
  - `backend/cmd/fast-extract/`
  - `backend/cmd/stream-chunk-restore/`
- **FR-002**: System MUST remove unreferenced prototype scripts:
  - `scripts/common/audit_chunks.py`
  - `scripts/linux/fast-snapshot.sh`
  - `scripts/linux/test_config_persistence.sh`
  - `scripts/linux/test_transition_persistence.sh`
- **FR-003**: System MUST preserve all active production operational and packaging scripts across macOS, Linux, and Windows.
- **FR-004**: System MUST verify that removing dead code does not break any build targets (`go test ./...`, `npm run build`).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of identified dead CLI tools and obsolete prototype scripts (6 targets) are removed from the repository.
- **SC-002**: 100% of remaining backend Go packages pass unit tests without errors (`go test ./...`).
- **SC-003**: Frontend builds cleanly with zero compilation errors (`npm run build`).
- **SC-004**: Packaging scripts for macOS, Linux, and Windows complete verification checks without missing dependency errors.

## Assumptions

- Production streaming snapshot restore is fully integrated and tested in `backend/pkg/snapshot/`.
- Native supervisor and config persistence are fully covered by Go unit tests in `backend/pkg/config/` and `backend/pkg/supervisor/`.
