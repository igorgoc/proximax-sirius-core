# ProximaX Sirius Core — Windows Handover & Testing Guide

> **Target Environment**: Windows 10 / 11 / Windows Server 2019/2022 (`amd64` / `x86_64`)  
> **Repository**: `https://github.com/igorgoc/proximax-sirius-core.git`  
> **Branch**: `main`

---

## 1. Project Context & Architecture

- **Backend**: Go 1.22+ native Windows daemon (`sirius-core.exe`) serving HTTP REST API and WebSocket events.
- **Frontend**: React 18 + TypeScript + Tailwind CSS UI embedded directly into the Go binary (`backend/dist`).
- **Cockpit Port**: `http://localhost:8080` (accessible in any modern browser; opens automatically on launch).
- **P2P & Blockchain Ports**:
  - `7900`: P2P transport network
  - `7901`: Internal node communication
  - `7903`: Dual-chain dynamic broker
- **Blockchain Engine**: C++ Sirius Catapult engine (`bin/sirius.bc` + RocksDB + plugins) executed inside WSL2 (Ubuntu-22.04) supervised by the Windows native binary.
- **Configuration Root**: `chainconfig\` containing `resources\` (Catapult properties) and `data\` (blockchain state).

---

## 2. Windows-Specific Optimizations & Invariants

1. **Pure-Go Build (Zero-CGO)**:
   - Replaced indirect CGO dependency from `supranational/blst` with a pure-Go shim (`backend/internal/blst_compat`).
   - Standard `go build` works out of the box on Windows without installing MSYS2, MinGW-w64, or GCC.
2. **WSL2 Execution & Runtime Dependencies**:
   - The C++ Catapult engine runs inside WSL2 (Ubuntu-22.04).
   - `libextension.fastfinality.so` requires `libatomic.so.1` (`libatomic1` package).
   - `bin/libatomic.so.1` is bundled in `bin/` and staged in `package-windows.ps1`.
   - `executeWSL` runs a silent pre-flight check inside WSL: `dpkg -s libatomic1 >/dev/null 2>&1 || (apt-get update -qq && apt-get install -y -qq libatomic1)`.
3. **NTFS ACL Security Hardening (POSIX 0600 Equivalent)**:
   - `start-node.ps1` runs `icacls` on `chainconfig\resources\config-harvesting.properties`, `config-user.properties`, and `.sirius-token`.
   - Strips inherited folder permissions and grants exclusive read/write access to the current Windows user and `SYSTEM`.
4. **Path & Lock Synchronization**:
   - `ToWSLPath` translates `C:\Sirius_data` -> `/mnt/c/Sirius_data`.
   - `FromWSLPath` and `normalizeHostPath` translate `/mnt/c/Sirius_data` -> `C:\Sirius_data` when reading properties.
   - Stale lock files (`server.lock`, `recovery.lock`, `statedb/*/LOCK`) are cleaned both via host Go and inside WSL (`rm -f <wslDataDir>/*.lock`).
   - `catapult.recovery` in WSL only executes when chain height > 1. At height ≤ 1, dirty partial `statedb` from previous aborted boots is cleared so `NemesisBlockLoader` computes the hash cleanly.
   - Process liveness in `GetStatus()` checks `dc.cmd.ProcessState == nil` on Windows (`syscall.Signal(0)` is unsupported on Windows).
5. **Cross-Platform Isolation**:
   - All Windows-specific logic is strictly isolated to `*_windows.go` or guarded by `if runtime.GOOS == "windows"`. Native macOS and Linux behaviors are preserved untouched.
6. **Frictionless Batch & PowerShell Launchers**:
   - `start.bat`, `stop.bat`, `restart.bat` allow double-click launching or CMD execution without encountering PowerShell execution policy restrictions.
   - `start-node.ps1`, `stop-node.ps1`, `restart-node.ps1` provide full PowerShell scripting support.
   - `run.bat` provides end-to-end dev build + run workflow.

---

## 3. Windows Prerequisites

1. **WSL2 (Windows Subsystem for Linux 2) & Virtualization**:
   - Required for the C++ Sirius Catapult engine (`bin/sirius.bc` running Ubuntu-22.04).
   - CPU Virtualization (Intel VT-x / AMD-V) enabled in BIOS/UEFI.
   - *Note*: If WSL2 is not yet enabled, the built-in Cockpit onboarding wizard (`WSLSetupModal`) will automatically detect it and offer a 1-click elevated install (`wsl --install --no-distribution`) with automatic reboot-resume tracking.
2. **PowerShell Execution Policy** (if running `.ps1` directly from terminal):
   ```powershell
   Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
   ```
   *(Note: Running `start.bat`, `stop.bat`, or `restart.bat` bypasses this automatically).*
3. **Windows Defender SmartScreen** (for standalone community zip releases):
   - If prompted with "Windows protected your PC", click **"More info"** -> **"Run anyway"**.

---

## 4. Windows Verification Checklist

Execute the following checklist on your Windows machine:

### A. If testing from Git Clone (Source Repository)
- [ ] **Git Clone / Pull**:
  ```powershell
  git clone https://github.com/igorgoc/proximax-sirius-core.git
  cd proximax-sirius-core
  git checkout main
  git pull origin main
  ```
- [ ] **Backend Tests**:
  ```powershell
  cd backend
  go test ./pkg/...
  cd ..
  ```
- [ ] **Build & Launch via `run.bat`**:
  ```cmd
  run.bat
  ```
  - Verify `sirius-core.exe` compiles cleanly (pure Go, zero-CGO).
  - Verify browser opens to `http://localhost:8080`.

---

### B. If testing from Release Archive (Standalone Package)
- [ ] **Extract ZIP Archive**:
  - Extract `proximax-sirius-windows-amd64-1.9.8.zip` to a folder (e.g. `C:\proximax-sirius-core`).
- [ ] **Launch Node**:
  - Double-click `start.bat` (or run `.\start.bat` in terminal).
  - Verify console outputs:
    ```
    =========================================================
      ProximaX Sirius Native Node is ONLINE (PID: xxxx)!

      Access GUI Cockpit Dashboard:
        >>> http://localhost:8080 <<<
    =========================================================
    ```
  - Verify default web browser opens `http://localhost:8080`.
- [ ] **Verify Web Cockpit UI & WSL2 Subsystem**:
  - Open `http://localhost:8080`.
  - Check **WSL Status**: If WSL is missing or requires distro setup, verify the **WSL Setup Modal** pops up with the correct status (`WSL_NOT_INSTALLED`, `WSL_V1_ONLY`, or `WSL2_NO_DISTRO`) and handles the elevation/retry flow gracefully.
  - Check **Overview**: verify status cards (Node State, Peer Connectivity, Height, Storage Available).
  - Check **Validator** tab: ensure Setup Wizard or Key configurations display properly.
  - Check **Settings -> Snapshots**: confirm defaults (data directory, backup folder, compression formats).
  - Check **Maintenance**: check sub-tabs "Snapshots & Sync" and "System & Health", and persistent "Danger Zone".
- [ ] **Mandatory Harvest Key Enforcement**:
  - Ensure the engine does not start if `harvestKey` is not configured.
  - Configure a valid 64-hex harvest key in Validator Settings and verify the node engine starts successfully in WSL2.
- [ ] **Stop & Restart**:
  - Run `stop.bat` (or `.\stop-node.ps1`): verify manager and WSL engine processes stop cleanly and `.sirius-core.pid` is removed.
  - Run `restart.bat` (or `.\restart-node.ps1`): verify clean shutdown followed by fresh boot.
- [ ] **External Storage / Drive Mount**:
  - Set custom `data.path` in Settings (e.g. `D:/sirius-data` or `E:/sirius-data`).
  - Verify data directory is created and mapped to WSL path (`/mnt/d/sirius-data`) correctly.

---

## 5. What to Ask on the Windows Machine (Assistant Prompt)

Copy and paste the following prompt into your AI agent or assistant on the Windows machine:

```text
Please read WINDOWS_HANDOVER.md and GEMINI.md in the repository root. Follow the Windows Verification Checklist in WINDOWS_HANDOVER.md step-by-step:
1. Verify prerequisites (WSL2 status with Ubuntu-22.04, Go 1.22+, Node 18+ if rebuilding frontend).
2. Run backend tests using `cd backend; go test ./pkg/...` to ensure all pure-Go tests pass on Windows.
3. Build or launch the node manager using `run.bat` (or `start.bat` if testing pre-built package).
4. Verify the web cockpit loads on http://localhost:8080 and verify WSL2 state detection / onboarding modal.
5. Verify mandatory harvest key enforcement, stop-node (stop.bat), and restart-node (restart.bat) operations.
6. Report findings and any Windows-specific edge cases encountered.
```
