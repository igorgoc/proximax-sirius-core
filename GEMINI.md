---
name: proximax-sirius-mainnet-peer
description: Architectural guidelines, privacy invariants, and storage rules for the ProximaX Sirius Mainnet Peer Node manager.
always_on: true
---

# ProximaX Sirius Mainnet Peer Node Guidelines

## 1. Key Privacy
- Never show raw private keys in UI result cards or overview panels.
- Display only public keys, addresses, and transaction hashes.
- Use password masking with reveal/hide toggle for all sensitive key inputs.
- Ensure private keys are only stored securely in `chainconfig/resources/config-harvesting.properties` and `config-user.properties`.

## 2. Fast-Sync & Storage Optimization
- Use direct streaming decompression from `https://huggingface.co/datasets/igorgoc/sirius-snapshot/resolve/main/sirius-data-backup-2026-09-10-131735.tar.zst` without intermediate tar file saving to eliminate disk overhead.
- Enforce Docker log rotation (`--log-opt max-size=250m --log-opt max-file=3`) and Sirius log caps (`rotationSize=25MB`, `maxTotalSize=250MB`).
- Maintain `.dockerignore` ignoring `chainconfig/data/` and `chainconfig/logs/` so Docker builds remain sub-10 seconds.
- Automatically clear `data/server.lock` on restart/recovery.

## 3. Configuration & External Storage
- Enforce strict separation between `bootKey` (P2P transport identity) and `harvestKey` (POS+ block harvesting identity). Never mirror harvest key to boot key.
- Support `data.path` in `config-user.properties` (default: `./chainconfig/data`) for storing blockchain data on external drives/SSDs.
- Mount `/Volumes:/Volumes` in `docker-compose.yml` so host-mounted SSD drives are accessible for directory browsing and container mounts.

## 4. Architecture & Frontend Standards
- Fully containerized in Docker to ensure zero pollution of the host OS.
- Always use `docker compose up -d --build` in `run.sh` so container images stay synchronized with code edits.
- Isolate background polling state (`/api/status` interval) from user form state so uncommitted form inputs are never overwritten.
- Go backend + embedded React/TypeScript UI served on port 8080.
- Connects to Sirius P2P on port 7900 and public REST APIs on port 3000.

## 5. Windows Native & WSL2 Architecture Guidelines
- **Windows Host Supervisor**: Compiled as native Windows x64 binary (`sirius-core.exe`), serving web UI on port 8080 (cockpit).
- **Engine Execution via WSL2**: C++ Sirius Catapult engine (`sirius.bc` + RocksDB + plugins) runs as ELF Linux x86_64 inside WSL2 (Ubuntu-22.04) for native ext4 performance and POSIX signal compliance.
- **Cross-Platform Isolation Rule**: All Windows-specific logic MUST be strictly isolated to `*_windows.go` (via `//go:build windows`) or guarded by `if runtime.GOOS == "windows"`. Native macOS and Linux paths must remain completely untouched.
- **Runtime Dependency Invariant (`libatomic1`)**:
  - `libextension.fastfinality.so` requires `libatomic.so.1` for GCC 64-bit/128-bit atomic intrinsics.
  - Default Ubuntu 22.04 WSL2 images omit `libatomic1`.
  - Enforce `bin/libatomic.so.1` in the distribution and automated self-healing pre-flight check in `executeWSL`:
    `dpkg -s libatomic1 >/dev/null 2>&1 || (apt-get update -qq && apt-get install -y -qq libatomic1)`
- **Windows Path & Lock Synchronization**:
  - `ToWSLPath` converts `C:\Path` to `/mnt/c/Path` for WSL execution.
  - `FromWSLPath` and `normalizeHostPath` convert `/mnt/c/Path` to `C:\Path` when loading configuration properties.
  - Stale locks (`server.lock`, `recovery.lock`, `broker.lock`, `statedb/*/LOCK`) must be cleared both via host Go and inside WSL (`rm -f '<wslDataDir>'/*.lock`).
  - **No Isolated Flat-File Byte Surgery**:
    - Sirius Catapult stores chain state across flat files (`supplemental.dat`, `BlockDifficultyCache.dat`, `index.dat`) and RocksDB column families (`statedb/`).
    - Flat files MUST NEVER be manually modified or rolled back independently of RocksDB. Desynchronizing them corrupts the Merkle state tree and causes `FastFinalityActions.cpp: rejecting block, signer ... invalid` and peer disconnections (`Verify_Error`).
  - **Automatic Recovery via catapult.recovery**:
    - Do not artificially alter `commit_step.dat` or `index.dat`. Let `catapult.recovery` automatically reconcile WAL and state commits.
  - **DrvFS Graceful Shutdown Timeout & Native ext4 Storage**:
    - Catapult RocksDB state flushes on WSL DrvFS (`/mnt/c/...`) have higher write latency than ext4. `stopWSL()` enforces a 45s SIGINT grace period before issuing SIGKILL to prevent mid-commit disk corruption.
    - Storing blockchain data in native WSL2 ext4 (`/var/lib/sirius/data`) eliminates DrvFS 9P overhead.
  - **State Desynchronization Recovery Rule**:
    - When statedb and flat files diverge (`signer invalid`), wipe `data/` and perform a fast-sync restore from the official snapshot (`https://huggingface.co/datasets/igorgoc/sirius-snapshot/resolve/main/sirius-data-backup-2026-09-10-131735.tar.zst`).
  - In WSL2, `catapult.recovery` only runs when block height > 1. At height ≤ 1, dirty partial `statedb` from aborted boots is cleared so `NemesisBlockLoader` boots cleanly.
  - Process liveness check in `GetStatus()` checks `dc.cmd.ProcessState == nil` on Windows (`proc.Signal(syscall.Signal(0))` is unsupported on Windows).

## 6. Autonomous Local LLM Delegation (LAN 190k Context Model)
- **Endpoint & Capability**: A local 27B model with a 196,608-token context window is hosted on LAN (`http://192.168.1.111:8080`) and accessible via `ask_remote`.
- **Proactive Context-Saving Invariant**:
  - Whenever reviewing or summarizing large files (>300 lines), multi-file modules, or bulk directories, DO NOT read the entire contents directly into the main Antigravity conversation context.
  - Proactively execute `ask_remote <files...> "<instruction>"` or `ask_remote --dir <dir> "<instruction>"` via `run_command` to let the local LAN model perform the heavy lifting and first-pass analysis.
  - For long logs (>100 lines) or raw terminal output, pipe them directly to `ask_remote`: e.g. `docker logs <container> | ask_remote "<instruction>"` or `ask_remote <log_file> "<instruction>"`.
  - For drafting boilerplate code, test suites, or repetitive conversions, delegate the draft generation to `ask_remote` first, then review and refine the output before committing.
  - Always verify and review the local model's output before applying critical logic or final architecture decisions. Only bring synthesized results or refined diffs into the AGY context to conserve Google Gemini quota and token bandwidth.
- **Unavailability Fallback (Session Forget Rule)**:
  - If `ask_remote` is unreachable or fails to connect (e.g., endpoint offline, connection refused, or network timeout), **immediately forget `ask_remote` for the remainder of the session**.
  - Do NOT retry `ask_remote` or repeatedly attempt connection.
  - Seamlessly fall back to processing all file inspections, code reviews, and instructions directly within Antigravity.
- **Zero Token-Waste Waiting Invariant**:
  - When `ask_remote` or any long-running command is pushed to a background task, **NEVER schedule timers or poll task status**.
  - Call zero tools and end the turn immediately; let the system's reactive wakeup resume execution at zero token cost.

