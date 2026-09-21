# Tasks: Pure Browser Cockpit & Complete Electron Removal

**Feature**: `005-remove-electron-pure-browser`
**Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md)

## Phase 1: Setup & Pre-flight Checks

- [X] T001 Verify baseline Go backend tests pass with `cd backend && go test ./...`
- [X] T002 Verify baseline frontend build with `cd frontend && npm run build`

---

## Phase 2: User Story 1 - Elimination of Electron Files & UI Hooks (Priority: P1) 🎯 MVP

**Goal**: Remove all Electron directories, scripts, and window hooks

- [X] T003 [P] [US1] Delete `electron/` directory completely
- [X] T004 [P] [US1] Delete `scripts/macos/sign_app.py`
- [X] T005 [P] [US1] Delete `frontend/src/components/QuitConfirmModal.tsx`
- [X] T006 [US1] Clean `frontend/src/components/Header.tsx` to remove `isElectron` padding and drag CSS classes
- [X] T007 [US1] Clean `frontend/src/App.tsx` to remove `electronAPI` IPC quit listeners and `QuitConfirmModal` state/import
- [X] T008 [US1] Clean `.gitignore` to remove `electron/` and `.app` exclusions

---

## Phase 3: User Story 2 - Packaging & Build Verification (Priority: P1)

**Goal**: Validate clean builds and cross-platform packaging with zero Electron dependencies

- [X] T009 [US2] Run `cd frontend && npm run build` and verify clean asset generation
- [X] T010 [US2] Run `cd backend && go test -race -count=1 ./pkg/...` to verify test suite
- [X] T011 [US2] Verify `package-macos.sh` packages standalone release without missing `sign_app.py` error

---

## Phase 4: Polish & Completion

- [X] T012 Update `specs/005-remove-electron-pure-browser/tasks.md` marking all tasks completed `[X]`
