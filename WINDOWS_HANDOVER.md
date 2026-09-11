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
- **Blockchain Engine**: Native Windows engine (`bin\sirius.exe` or `bin\sirius.bc.exe`) managed via supervisor.
- **Configuration Root**: `chainconfig\` containing `resources\` (Catapult properties) and `data\` (blockchain state).

---

## 2. Windows-Specific Optimizations & Invariants

1. **Pure-Go Build (Zero-CGO)**:
   - Replaced indirect CGO dependency from `supranational/blst` with a pure-Go shim (`backend/internal/blst_compat`).
   - Standard `go build` works out of the box on Windows without installing MSYS2, MinGW-w64, or GCC.
2. **NTFS ACL Security Hardening (POSIX 0600 Equivalent)**:
   - `start-node.ps1` runs `icacls` on `chainconfig\resources\config-harvesting.properties`, `config-user.properties`, and `.sirius-token`.
   - Strips inherited folder permissions and grants exclusive read/write access to the current Windows user and `SYSTEM`.
3. **Cross-Platform Path Sanitization**:
   - File paths written to `.properties` use forward slashes (`C:/proximax/...` via `filepath.ToSlash`), preventing Boost PropertyTree escape sequence parsing bugs (e.g. `\r` as carriage return).
4. **Frictionless Batch & PowerShell Launchers**:
   - `start.bat`, `stop.bat`, `restart.bat` allow double-click launching or CMD execution without encountering PowerShell execution policy restrictions.
   - `start-node.ps1`, `stop-node.ps1`, `restart-node.ps1` provide full PowerShell scripting support.
   - `run.bat` provides end-to-end dev build + run workflow.
5. **Lock File Auto-Cleanup**:
   - Stale `*.lock` and `statedb\*\LOCK` files are cleaned automatically on startup and shutdown.
6. **Dynamic Link Libraries (DLLs)**:
   - `start-node.ps1` prepends `$RootDir\bin` to `$env:PATH` so Windows automatically resolves required DLLs when launching the engine.

---

## 3. Windows Prerequisites

1. **Microsoft Visual C++ Redistributable (x64)**:
   - Required for the native C++ engine (`sirius.exe`).
   - Download from Microsoft: [vc_redist.x64.exe](https://aka.ms/vs/17/release/vc_redist.x64.exe) (Visual C++ 2015–2022).
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
  - Verify `sirius-core.exe` compiles cleanly.
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
- [ ] **Verify Web Cockpit UI**:
  - Open `http://localhost:8080`.
  - Check **Overview**: verify status cards (Node State, Peer Connectivity, Height, Storage Available).
  - Check **Validator** tab: ensure Setup Wizard or Key configurations display properly.
  - Check **Settings -> Snapshots**: confirm defaults (data directory, backup folder, compression formats).
  - Check **Maintenance**: check sub-tabs "Snapshots & Sync" and "System & Health", and persistent "Danger Zone".
- [ ] **Mandatory Harvest Key Enforcement**:
  - Ensure the engine does not start if `harvestKey` is not configured.
  - Configure a valid 64-hex harvest key in Validator Settings and verify the node engine starts successfully.
- [ ] **Stop & Restart**:
  - Run `stop.bat` (or `.\stop-node.ps1`): verify manager and engine stop cleanly and `.sirius-core.pid` is removed.
  - Run `restart.bat` (or `.\restart-node.ps1`): verify clean shutdown followed by fresh boot.
- [ ] **External Storage / Drive Mount**:
  - Set custom `data.path` in Settings (e.g. `D:/sirius-data` or `E:/sirius-data`).
  - Verify data directory is created and permissioned correctly.

---

## 5. What to Ask on the Windows Machine (Assistant Prompt)

Copy and paste the following prompt into your AI agent or assistant on the Windows machine:

```text
Please read WINDOWS_HANDOVER.md and GEMINI.md in the repository root. Follow the Windows Verification Checklist in WINDOWS_HANDOVER.md step-by-step:
1. Verify prerequisites (Visual C++ 2015-2022 x64 Redistributable, Go 1.22+, Node 18+ if building frontend).
2. Run backend tests using `cd backend; go test ./pkg/...` to ensure all pure-Go tests pass on Windows.
3. Build or launch the node manager using `run.bat` (or `start.bat` if testing pre-built package).
4. Verify the web cockpit loads on http://localhost:8080.
5. Verify harvest key validation, stop-node (stop.bat), and restart-node (restart.bat) operations.
6. Report findings and any Windows-specific edge cases encountered.
```
