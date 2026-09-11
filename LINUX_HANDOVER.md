# ProximaX Sirius Core — Linux Handover & Testing Guide

> **Current Repository State**: `main` branch ([commit `c194e3c`](https://github.com/igorgoc/proximax-sirius-core/commit/c194e3c))  
> **Target Environment**: Linux (`x86_64` / `aarch64`)  
> **Repository**: `https://github.com/igorgoc/proximax-sirius-core.git`

---

## 1. Project Context & Architecture

- **Stack**: Go 1.22+ backend daemon + embedded React 18 / TypeScript / Vite UI.
- **Port**: Web Manager served on `http://127.0.0.1:8080` (API & Web UI).
- **Sirius P2P Ports**: `7900` (P2P), `7901` (API/REST internal), `7903` (DBRB).
- **Core Engine**: Sirius C++ native engine (`sirius.bc`) located in `./bin/sirius.bc` with dependencies in `./bin` or system library paths (`LD_LIBRARY_PATH`).
- **Configuration Root**: `./chainconfig` directory containing `resources/` (property files) and `data/` (blockchain state).

---

## 2. Invariants & Rules (from `GEMINI.md`)

1. **Key Privacy**:
   - Never show raw private keys in UI result cards or overview panels.
   - Display only public keys, addresses, and transaction hashes.
   - Use password masking with reveal/hide toggle for sensitive inputs.
   - Store private keys exclusively in `chainconfig/resources/config-harvesting.properties` and `config-user.properties`.
2. **Key Separation**:
   - `bootKey` (P2P mesh identity) and `harvestKey` (POS+ block harvesting) must remain strictly separated. Never mirror one to the other.
3. **Mandatory Harvest Key**:
   - The node **MUST NOT start** without a valid 64-character hexadecimal harvest key. Both supervisor pre-flight and API endpoints fail closed with HTTP 400 if missing or placeholder.
4. **Fast-Sync & Storage**:
   - Direct streaming decompression from Hugging Face Zstd snapshots without saving intermediate tarballs.
   - Auto-clear `data/*.lock` and state database locks on recovery/restart.
   - Atomic file updates and rollback protection on engine and config updates.

---

## 3. What Was Recently Completed & Verified

1. **Mandatory Harvest Key Enforcement**:
   - Backend supervisor preflight (`validateHarvestKeyPreflight()` in `supervisor.go`) checks `config-harvesting.properties` before starting `sirius.bc`.
   - API `/api/node/start` and `/api/node/restart` reject requests without a harvest key.
   - CLI script `start-node.sh` validates harvest key length and hex characters.
   - Setup Wizard removed "Skip" option; Step 3 requires valid key before proceeding to Step 4.
   - UI Start buttons in Header, Overview, and Storage tabs are disabled when no harvest key is set, showing clear guidance to configure one.

2. **Launcher & Multi-Platform Fixes**:
   - Working directory auto-resolution via `os.Executable()`. If launched outside the folder (e.g. file manager double-click), it switches `os.Chdir()` to the executable's directory.
   - Port 8080 conflict resolution: if `sirius-core` is already active on 8080, it detects the running instance, opens the browser to `http://localhost:8080`, and exits cleanly (exit code 0).
   - Removed duplicate scripts; standardized on `start.sh`, `stop.sh`, and `restart.sh`.

3. **UI Refinements**:
   - Simplified compression format selection to `.tar.zst` and `.tar.gz` with compact icons.
   - Simplified remote sync verification text to "Signature verified" and badge to "Signed".
   - Cleaned up Boot Key title to "Node boot private key" matching Harvester key styling.
   - Removed "on your fast external SSD" text from Settings / General data directory description.

4. **Engine Updater & CI Reliability**:
   - Fixed engine download timeout by introducing a dedicated 15-minute streaming HTTP client.
   - Handled GitHub API rate-limiting gracefully in CI integration tests.

---

## 4. Linux Verification Checklist

Run these verification steps on the Linux machine:

- [ ] **Git Sync**: Pull the latest code on `main`:
  ```bash
  git pull origin main
  ```
- [ ] **Test Suite**: Run backend unit and race detector tests:
  ```bash
  cd backend && go test -v -race ./pkg/...
  ```
- [ ] **Frontend Build**: Test TypeScript compilation and Vite bundling:
  ```bash
  cd frontend && npm run build
  ```
- [ ] **Daemon Binary Build**: Build standalone `sirius-core`:
  ```bash
  cd backend && go build -ldflags="-s -w" -o ../sirius-core .
  ```
- [ ] **Launcher Test**:
  - Test `./start.sh` (verifies port conflict handling, `xdg-open` browser launch, and node startup).
  - Test `./stop.sh` (verifies clean shutdown and lock file cleanup).
  - Test `./restart.sh` (verifies clean graceful restart).
- [ ] **Harvest Key Enforcement Test on Linux**:
  - Set `harvestKey = REMOTE_ACCOUNT_PRIVATE_KEY` in `chainconfig/resources/config-harvesting.properties`.
  - Attempt to run `./start-node.sh`: verify it refuses to start with an error message.
  - Attempt to call `POST http://127.0.0.1:8080/api/node/start`: verify HTTP 400 rejection.
  - Restore a valid 64-hex key and verify start succeeds.
- [ ] **Dynamic Libraries (`LD_LIBRARY_PATH`)**:
  - Confirm `sirius.bc` locates `libboost_*` and any system libraries on Linux without missing symbol errors (`ldd ./bin/sirius.bc`).
