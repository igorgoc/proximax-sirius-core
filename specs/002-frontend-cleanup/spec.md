# Feature Specification: Frontend Dead Code Cleanup and Bundle Optimization

**Feature Branch**: `002-frontend-cleanup`

**Created**: 2026-09-17

**Status**: Ready for Planning

**Input**: Remove orphaned frontend SnapshotModal component and optimize UI bundle size with dynamic code-splitting.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Elimination of Orphaned UI Components (Priority: P1)

As a frontend maintainer, I want unused legacy modal components removed from `frontend/src/components/` so that the component tree is clean, readable, and contains no dead code.

**Why this priority**: Eliminates developer confusion between the active `SnapshotSubTab` and the obsolete `SnapshotModal`.

**Independent Test**: Remove `frontend/src/components/SnapshotModal.tsx` and run `npm run build` to verify zero missing import errors.

**Acceptance Scenarios**:

1. **Given** `SnapshotModal.tsx` in `frontend/src/components/`, **When** the file is deleted, **Then** all active tabs and modals compile and function without errors.

---

### User Story 2 - UI Bundle Optimization & Code Splitting (Priority: P2)

As a node operator using the Sirius Cockpit dashboard, I want fast initial page load times and optimized chunk sizes so that the dashboard loads efficiently over local and remote connections.

**Why this priority**: Resolves the Vite 500 kB chunk warning by code-splitting large tabs and lazy-loading heavy modal dialogues.

**Independent Test**: Run `npm run build` and verify that output bundle chunks are neatly split and initial bundle size is significantly reduced.

**Acceptance Scenarios**:

1. **Given** large tabs (`ConfigTab`, `MaintenanceTab`, `LogsTab`, `StorageTab`), **When** dynamic code-splitting is applied, **Then** Vite produces split chunks and the UI renders all tabs smoothly without flickering or layout breaks.

---

### Edge Cases

- What happens if a lazy-loaded tab takes a moment to load?
  - A clean React `Suspense` fallback with a subtle loading spinner ensures no layout shifting.
- What happens if an error occurs while dynamically loading a chunk?
  - `ErrorBoundary` gracefully catches any chunk loading failure and allows the user to reload.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST delete `frontend/src/components/SnapshotModal.tsx`.
- **FR-002**: System MUST optimize `frontend/vite.config.ts` or component imports to code-split large vendor dependencies (`lucide-react`, large tabs) into separate chunks.
- **FR-003**: System MUST verify that `npm run build` finishes with 100% success and reduced main bundle size.
- **FR-004**: System MUST verify that Go backend tests `go test ./...` continue to pass.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Zero orphaned modal components in `frontend/src/components/`.
- **SC-002**: Elimination or significant reduction of monolithic JS chunk size in `dist/assets/`.
- **SC-003**: 100% clean build on `npm run build` with zero TypeScript or Vite compilation errors.
- **SC-004**: All active dashboard features (Status, Overview, Config, Maintenance, Storage, Logs, Validator) operate with 100% functionality.

## Assumptions

- Snapshot restoration is fully handled by `SnapshotSubTab.tsx` inside `MaintenanceTab.tsx`.
- React 18 `Suspense` and `React.lazy` are fully supported by Vite and standard ES modules.
