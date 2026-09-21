# Feature Specification: Pure Browser Cockpit & Complete Electron Removal

**Feature Branch**: `005-remove-electron-pure-browser`

**Created**: 2026-09-18

**Status**: Ready for Planning

**Input**: Fully transition to a 100% native Go supervisor + pure browser dashboard architecture across all platforms (macOS, Windows, Linux), eliminating Electron dependencies, wrapper scripts, window quit interceptors, and desktop app bundle packaging.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Zero-Overhead Pure Browser Node Operation (Priority: P1)

As a Sirius peer node operator on any platform (macOS, Windows, Linux), I want to run the lightweight native supervisor and interact with the node entirely through my preferred web browser at `http://localhost:8080`, without installing or running heavyweight Electron desktop wrappers.

**Why this priority**: Eliminates hundreds of megabytes of Electron runtime bloat, eliminates Chromium window lifecycle issues, and provides a uniform web experience across all desktop operating systems and headless servers.

**Independent Test**: Start the supervisor on any platform, open `http://localhost:8080` in Chrome/Safari/Firefox/Edge, and control all node functions (Start/Stop/Restart/Config/Logs) without any Electron dependency.

**Acceptance Scenarios**:

1. **Given** the repository, **When** all Electron artifacts (`electron/`, `sign_app.py`, `QuitConfirmModal.tsx`) are removed, **Then** the Go supervisor compiles, embeds the React UI, and serves it on port 8080.
2. **Given** the web dashboard opened in a browser, **When** viewing the header and controls, **Then** all Electron-specific drag regions and window traffic-light paddings are gone, providing a clean responsive web layout.

---

### User Story 2 - Clean Release Packaging & Maintenance (Priority: P2)

As a maintainer and release engineer, I want all packaging scripts (`package-macos.sh`, `package-linux.sh`, `package-windows.ps1`) to strictly produce standalone tarballs and zip archives with zero Electron dependencies.

**Why this priority**: Streamlines CI/CD builds, reduces archive sizes, and prevents code signing failures on macOS/Windows desktop bundles.

**Independent Test**: Run packaging scripts on each platform and verify release bundles contain only `sirius-core`, `chainconfig/`, and launch scripts.

**Acceptance Scenarios**:

1. **Given** the macOS packager, **When** `package-macos.sh` is executed, **Then** it packages the standalone native bundle without attempting to build or sign `.app` Electron wrappers.

---

### Edge Cases

- What happens when a user closes their browser tab?
  - In pure browser mode, the Go supervisor and C++ engine continue running as a background daemon uninterrupted until explicitly stopped via the Cockpit UI or `stop.sh` / `stop.bat`.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST delete the entire `electron/` directory (all scripts, assets, and dependencies).
- **FR-002**: System MUST remove `QuitConfirmModal.tsx` and all Electron IPC hooks (`electronAPI`) from `frontend/src/App.tsx`.
- **FR-003**: System MUST clean `frontend/src/components/Header.tsx` to remove `isElectron` padding and Electron window drag regions.
- **FR-004**: System MUST delete `scripts/macos/sign_app.py` (which was dedicated to Electron `.app` bundles).
- **FR-005**: System MUST clean `.gitignore` to remove all Electron and `.app` bundle exclusions.
- **FR-006**: System MUST verify that `npm run build` and `go test -race ./...` pass with 100% success.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Zero references to Electron in frontend source code and scripts.
- **SC-002**: Complete removal of ~100MB+ `electron/` directory.
- **SC-003**: 100% clean build on `npm run build` and `go test -race ./...`.
- **SC-004**: Standalone browser dashboard operates with full functionality on port 8080 across all 3 platforms.
