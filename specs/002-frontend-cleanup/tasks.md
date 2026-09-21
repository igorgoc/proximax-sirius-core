# Tasks: Frontend Cleanup and Bundle Optimization

**Feature**: `002-frontend-cleanup`
**Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md)

## Phase 1: Setup & Pre-flight Checks

- [X] T001 Verify baseline frontend build with `export PATH=/Users/igorgoc/.nvm/versions/node/v20.20.2/bin:$PATH && cd frontend && npm run build`

---

## Phase 2: Foundational (Blocking Prerequisites)

- [X] T002 Verify `SnapshotSubTab.tsx` in `frontend/src/components/` is fully functional and referenced by `MaintenanceTab.tsx`

---

## Phase 3: User Story 1 - Dead Component Deletion (Priority: P1) 🎯 MVP

**Goal**: Remove orphaned `SnapshotModal.tsx` from component tree

- [X] T003 [US1] Delete obsolete modal file `frontend/src/components/SnapshotModal.tsx`
- [X] T004 [US1] Verify zero import references remain for `SnapshotModal` across `frontend/src/`

---

## Phase 4: User Story 2 - Vite Bundle Optimization (Priority: P2)

**Goal**: Configure Rollup manual chunks for icon and vendor libraries

- [X] T005 [US2] Update `frontend/vite.config.ts` to add `build.rollupOptions.output.manualChunks` for `lucide-icons` and `react-vendor`
- [X] T006 [US2] Run `npm run build` in `frontend` and verify chunk sizes and elimination of the 500 kB warning

---

## Phase 5: Polish & Regression Verification

- [X] T007 Run `cd backend && go test ./...` to verify Go backend continues to serve the embedded dist
- [X] T008 Update `specs/002-frontend-cleanup/tasks.md` marking all tasks completed `[X]`
