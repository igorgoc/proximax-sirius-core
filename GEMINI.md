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
  - **State Cache & Storage Height Reconciliation (`reconcileChainStateIntegrity`)**:
    - Abrupt process terminations can cause `state/supplemental.dat` and `state/BlockDifficultyCache.dat` (cache height) to advance to `N` while `index.dat` (storage height) remains at `N-1`.
    - Both `LocalNode` and `catapult.recovery` abort with exit status 134 if `cache height > storage height`.
    - Enforce automated pre-flight reconciliation in `wsl_windows.go`: aligns `supplemental.dat` and `BlockDifficultyCache.dat` back to `storage height`, and resets `commit_step.dat` to 0.
  - **DrvFS Graceful Shutdown Timeout**:
    - Catapult RocksDB state flushes on WSL DrvFS (`/mnt/c/...`) require up to 30s. `stopWSL()` enforces a 45s SIGINT grace period before issuing SIGKILL to prevent mid-commit disk corruption.
  - In WSL2, `catapult.recovery` only runs when block height > 1. At height ≤ 1, dirty partial `statedb` from aborted boots is cleared so `NemesisBlockLoader` boots cleanly.
  - Process liveness check in `GetStatus()` checks `dc.cmd.ProcessState == nil` on Windows (`proc.Signal(syscall.Signal(0))` is unsupported on Windows).
