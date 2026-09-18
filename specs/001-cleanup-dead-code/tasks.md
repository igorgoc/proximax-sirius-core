# Tasks: Codebase Cleanup and Dead Artifact Removal

**Feature**: `001-cleanup-dead-code`
**Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md)

## Phase 1: Setup & Pre-flight Checks

**Purpose**: Verify pre-clean state and test baselines

- [X] T001 Verify existing Go backend tests pass with `cd backend && go test ./...`
- [X] T002 Verify existing frontend builds cleanly with `cd frontend && npm run build`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Confirm safety invariants before artifact deletion

- [X] T003 Verify `backend/pkg/snapshot` contains complete streaming decompression and `backend/cmd/sign-release` is preserved
- [X] T004 Verify packaging scripts (`scripts/macos/package-macos.sh`, `scripts/linux/package-linux.sh`, `scripts/windows/package-windows.ps1`) have no dependencies on obsolete tools

---

## Phase 3: User Story 1 - Dead Code and Obsolete Script Removal (Priority: P1) 🎯 MVP

**Goal**: Remove all 6 dead/obsolete files and directories across backend and scripts

**Independent Test**: All dead files are removed from disk; `git status` shows clean removal of dead targets.

### Implementation for User Story 1

- [X] T005 [P] [US1] Remove obsolete Go CLI tool directory `backend/cmd/fast-extract`
- [X] T006 [P] [US1] Remove obsolete Go CLI tool directory `backend/cmd/stream-chunk-restore`
- [X] T007 [P] [US1] Remove unreferenced Python validator `scripts/common/audit_chunks.py`
- [X] T008 [P] [US1] Remove Docker-era shell script `scripts/linux/fast-snapshot.sh`
- [X] T009 [P] [US1] Remove superseded test script `scripts/linux/test_config_persistence.sh`
- [X] T010 [P] [US1] Remove superseded test script `scripts/linux/test_transition_persistence.sh`
- [X] T011 [US1] Clean up empty parent directory `scripts/common` if no files remain

**Checkpoint**: Dead code and scripts completely eliminated.

---

## Phase 4: User Story 2 - Automated Verification & Test Completeness (Priority: P2)

**Goal**: Validate full compilation and test suite passes without any regression

**Independent Test**: Run `go test ./...` in `backend` and `npm run build` in `frontend`; all pass.

### Implementation for User Story 2

- [X] T012 [US2] Run `cd backend && go test -v ./...` to verify all Go packages pass tests
- [X] T013 [US2] Run `cd frontend && npm run build` to verify frontend assets build
- [X] T014 [US2] Verify documentation references in `CROSS_PLATFORM_KNOWLEDGE.md` are clean

---

## Phase 5: Polish & Spec-Kit Infrastructure Tracking

**Purpose**: Track Spec-Kit artifacts and ensure repository cleanliness

- [X] T015 Verify `git status` reflects clean removal of dead code and inclusion of `.specify/` and `.agents/`
- [X] T016 Mark all completed tasks in `specs/001-cleanup-dead-code/tasks.md`

---

## Dependencies & Execution Order

1. **Setup (Phase 1)** $\rightarrow$ **Foundational (Phase 2)** $\rightarrow$ **US1 Removal (Phase 3)** $\rightarrow$ **US2 Verification (Phase 4)** $\rightarrow$ **Polish (Phase 5)**.
2. T005, T006, T007, T008, T009, T010 can execute in parallel.
