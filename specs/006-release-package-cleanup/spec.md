# Feature Specification: Release Packages Standardization & Single Start/Stop Layout

**Feature Branch**: `006-release-package-cleanup`

**Created**: 2026-09-18

**Status**: Ready for Implementation

**Input**: Standardize the downloaded release packages (`proximax-sirius-core-mac`, `proximax-sirius-core-linux`, `proximax-sirius-core-windows` in `/Users/igorgoc/Downloads`) and the repository packaging scripts (`package-macos.sh`, `package-linux.sh`, `package-windows.ps1`) so that:
1. The root of each release package contains ONLY one Start script (which launches the supervisor and automatically opens the browser at `http://localhost:8080`), one Stop script (which gracefully terminates the node), the binary (`sirius-core` / `sirius-core.exe`), and a comprehensive `README.md`.
2. All secondary / advanced helper scripts (`restart.*`, `run.*`, `setup-wsl.*`, `start-node.*`, `stop-node.*`, `reset_to_genesis.sh`, `WINDOWS_DEFENDER_NOTES.md`, etc.) are organized exclusively inside `scripts/<os>/` to avoid cluttering the root and confusing users.
3. Unneeded handover documents (e.g. `WINDOWS_HANDOVER.md`, `LINUX_HANDOVER.md`) are completely removed.
4. Each package includes an intuitive, well-documented `README.md` explaining how to start, stop, access the web cockpit, and how/when to use the scripts in `scripts/<os>/`.

## User Scenarios & Testing

### User Story 1 - Simplified Node Operation for End Users (Priority: P1)

As a node operator unpacking a release package on Windows, macOS, or Linux, I want to see a clean, uncluttered root directory with only `start`, `stop`, the node binary, and a `README.md`, so I can immediately start my node and open the web cockpit without guessing which script to run.

**Acceptance Scenarios**:
1. **Windows package**:
   - Root contains ONLY: `sirius-core.exe`, `start.bat`, `stop.bat`, `README.md`, `bin/`, `chainconfig/`, `scripts/windows/`.
   - Running `start.bat` launches the supervisor and opens `http://localhost:8080` in the default web browser.
   - Running `stop.bat` cleanly terminates the node supervisor and WSL engine.
   - `WINDOWS_HANDOVER.md` is deleted.
2. **Linux package**:
   - Root contains ONLY: `sirius-core`, `start.sh`, `stop.sh`, `README.md`, `bin/`, `chainconfig/`, `scripts/linux/`.
   - Running `start.sh` launches the supervisor in the background and opens the web browser.
   - Running `stop.sh` cleanly terminates the node.
3. **macOS package**:
   - Root contains ONLY: `sirius-core`, `start.sh`, `start.command` (Finder launcher), `stop.sh`, `README.md`, `bin/`, `chainconfig/`, `scripts/macos/`.
   - Running `start.sh` or double-clicking `start.command` launches the supervisor in the background and opens Safari/default browser.
   - Running `stop.sh` cleanly terminates the node.

---

### User Story 2 - Automated Clean Packaging from Repo (Priority: P2)

As a maintainer generating new releases, I want `package-windows.ps1`, `package-linux.sh`, and `package-macos.sh` to automatically build archives with this exact clean directory layout.

**Acceptance Scenarios**:
1. Packaging scripts copy ONLY `start.*`, `stop.*`, `README.md`, and the binary to the release root.
2. All helper scripts are placed exclusively in `scripts/<os>/`.
3. Build artifacts pass verification tests.

## Requirements

### Functional Requirements

- **FR-001**: Clean `/Users/igorgoc/Downloads/proximax-sirius-core-windows` root to contain only `sirius-core.exe`, `start.bat`, `stop.bat`, `README.md`, `bin/`, `chainconfig/`, `scripts/windows/`.
- **FR-002**: Clean `/Users/igorgoc/Downloads/proximax-sirius-core-linux` root to contain only `sirius-core`, `start.sh`, `stop.sh`, `README.md`, `bin/`, `chainconfig/`, `scripts/linux/`.
- **FR-003**: Clean `/Users/igorgoc/Downloads/proximax-sirius-core-mac` root to contain only `sirius-core`, `start.sh`, `start.command`, `stop.sh`, `README.md`, `bin/`, `chainconfig/`, `scripts/macos/`.
- **FR-004**: Create comprehensive `README.md` for each package detailing root scripts and `scripts/<os>/` helpers.
- **FR-005**: Delete `WINDOWS_HANDOVER.md` and `LINUX_HANDOVER.md`.
- **FR-006**: Update `scripts/windows/package-windows.ps1`, `scripts/linux/package-linux.sh`, and `scripts/macos/package-macos.sh` to enforce the clean root and `README.md` generation.
