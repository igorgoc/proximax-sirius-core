# Tasks: Dynamic Code-Splitting for Cockpit UI

**Feature**: `004-dynamic-code-splitting`
**Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md)

## Phase 1: Setup & Pre-flight Checks

- [X] T001 Verify baseline frontend build with `export PATH=/Users/igorgoc/.nvm/versions/node/v20.20.2/bin:$PATH && cd frontend && npm run build`

---

## Phase 2: User Story 1 - Dynamic Code-Splitting Implementation (Priority: P1) 🎯 MVP

**Goal**: Convert secondary subtabs and modals in `frontend/src/App.tsx` into `React.lazy` imports with `Suspense` fallback

- [X] T002 [US1] Convert `ValidatorTab`, `NetworkTab`, `StorageTab`, `LogsTab`, `ConfigTab`, `MaintenanceTab` to `React.lazy()` in `frontend/src/App.tsx`
- [X] T003 [US1] Convert `SetupWizard`, `AboutModal`, `QuitConfirmModal`, `EngineUpdateModal`, `WSLSetupModal` to `React.lazy()` in `frontend/src/App.tsx`
- [X] T004 [US1] Add a lightweight animated fallback spinner within `<Suspense>` wrapping the main content and modal outlets in `frontend/src/App.tsx`

---

## Phase 3: User Story 2 - Build Validation & Performance Verification (Priority: P1)

**Goal**: Verify that discrete chunk assets are emitted and initial bundle is significantly lighter

- [X] T005 [US2] Run `cd frontend && npm run build` and verify emitted chunk sizes
- [X] T006 [US2] Run `cd backend && go test ./...` to verify full integration test suite passes

---

## Phase 4: Polish & Documentation

- [X] T007 Update `specs/004-dynamic-code-splitting/tasks.md` marking all tasks completed `[X]`
