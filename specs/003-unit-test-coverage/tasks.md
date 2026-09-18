# Tasks: Complete Go Backend Test Coverage

**Feature**: `003-unit-test-coverage`
**Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md)

## Phase 1: Setup & Pre-flight Checks

- [X] T001 Verify baseline Go backend tests pass with `cd backend && go test ./...`

---

## Phase 2: User Story 1 - Migrator Package Unit Test Suite (Priority: P1) 🎯 MVP

**Goal**: Implement comprehensive test suite for `backend/pkg/migrator`

- [X] T002 [P] [US1] Create unit tests in `backend/pkg/migrator/migrator_test.go` covering directory processing, binary index serialization, cancellation, and empty folder handling
- [X] T003 [US1] Run `cd backend && go test -v ./pkg/migrator/...` and verify 100% pass

---

## Phase 3: User Story 2 - Storage Package Unit Test Suite (Priority: P1)

**Goal**: Implement unit tests for `backend/pkg/storage`

- [X] T004 [P] [US2] Create unit tests in `backend/pkg/storage/storage_test.go` covering config load/save, key masking in `GetStatus`, sandbox cleanup, and metrics calculations
- [X] T005 [US2] Run `cd backend && go test -v ./pkg/storage/...` and verify 100% pass

---

## Phase 4: User Story 3 - Network Package Unit Test Suite (Priority: P2)

**Goal**: Implement unit tests for `backend/pkg/network`

- [X] T006 [P] [US3] Create unit tests in `backend/pkg/network/upnp_test.go` covering struct defaults, thread safety, and port listening checks
- [X] T007 [US3] Run `cd backend && go test -v ./pkg/network/...` and verify 100% pass

---

## Phase 5: Polish & Full Suite Verification

- [X] T008 Run `cd backend && go test -v ./...` to verify ALL packages report `ok` with 0 untested packages
- [X] T009 Update `specs/003-unit-test-coverage/tasks.md` marking all tasks completed `[X]`
