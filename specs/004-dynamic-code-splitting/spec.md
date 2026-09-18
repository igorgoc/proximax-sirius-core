# Feature Specification: Dynamic Code-Splitting for Cockpit UI

**Feature Branch**: `004-dynamic-code-splitting`

**Created**: 2026-09-17

**Status**: Ready for Planning

**Input**: Split large subtabs (ConfigTab, MaintenanceTab, LogsTab, StorageTab, ValidatorTab, NetworkTab) and dialog modals using React.lazy() to reduce initial cockpit bundle size and optimize load performance.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Instant Initial Page Load for Node Operators (Priority: P1)

As a Sirius node operator opening the Cockpit dashboard (`http://localhost:8080`), I want the initial page and Overview tab to load almost instantaneously without downloading all heavy configuration forms and maintenance managers upfront.

**Why this priority**: Enhances perceived UI performance, reduces memory overhead on lower-end node host machines, and allows browser caching of individual tab chunks.

**Independent Test**: Load the root page and inspect network requests in browser DevTools or bundle analyzer; verify `index.js` is reduced and secondary tabs load on demand.

**Acceptance Scenarios**:

1. **Given** the application shell with `OverviewTab`, **When** the app renders, **Then** only the primary bundle and Overview tab are loaded initially.
2. **Given** switching to `ConfigTab`, `MaintenanceTab`, or `LogsTab`, **When** the tab is clicked, **Then** the tab chunk is dynamically loaded with a smooth `Suspense` loading transition.

---

### User Story 2 - Resilient Chunk Loading & Error Boundary Integration (Priority: P2)

As an operator running in fluctuating network conditions, I want dynamic chunk loading failures to be safely caught by the `ErrorBoundary` so that the cockpit never crashes into a blank white screen.

**Why this priority**: Prevents unhandled chunk load errors from crashing the operator interface.

**Independent Test**: Simulate offline/chunk load error and verify `ErrorBoundary` provides a clean retry mechanism.

**Acceptance Scenarios**:

1. **Given** a chunk load event, **When** network drops or file is refreshed, **Then** `ErrorBoundary` catches the error and offers a "Reload Dashboard" option.

---

### Edge Cases

- What happens if the user visits a direct URL hash (e.g. `http://localhost:8080/#logs`)?
  - `React.lazy` and `Suspense` seamlessly stream the `LogsTab` chunk immediately on initial mount with zero routing delays.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST convert secondary tabs (`ConfigTab`, `MaintenanceTab`, `LogsTab`, `StorageTab`, `ValidatorTab`, `NetworkTab`) to dynamic `React.lazy()` imports in `frontend/src/App.tsx`.
- **FR-002**: System MUST convert heavy modal dialogs (`SetupWizard`, `AboutModal`, `EngineUpdateModal`, `WSLSetupModal`, `QuitConfirmModal`) to `React.lazy()` imports.
- **FR-003**: System MUST wrap tab and modal outlets in `React.Suspense` with an operator-themed spinner fallback.
- **FR-004**: System MUST verify `npm run build` generates discrete chunks for each tab and `go test ./...` in `backend/` passes 100%.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Initial application entry chunk size reduced by at least 40% compared to monolithic bundle.
- **SC-002**: Zero layout shift or flashing during tab transitions.
- **SC-003**: 100% clean build on `npm run build` with discrete chunk artifacts in `dist/assets/`.
