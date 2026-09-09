# ProximaX Sirius Native Migration — Session Handoff (Ubuntu Transition)

**Date:** 2026-09-09  
**Branch:** `main` (`proximax-sirius-core-native`) / `feature/reducedDataSize` (`cpp-xpx-chain`)

---

## 1. Executive Summary & Product Decisions

1. **Two Independent Release Pipelines**:
   - **`cpp-xpx-chain`**: Produces native C++ Sirius engine (`sirius.bc`), dynamic plugins, and linked shared libraries. Signed with engine key, verified via `engine.compat.json`.
   - **`proximax-sirius-core`**: Produces Go Node Manager daemon + Web Cockpit + Desktop App Shell (Electron). Signed with manager key (`NODE_MANAGER_RELEASE_PRIVATE_KEY`), verified via `manager.compat.json`.

2. **Windows Engine Strategy Decision**:
   - Native MSVC Windows port of `cpp-xpx-chain` has been evaluated and **deprioritized** (estimated at 2–3 months of heavy systems C++ refactoring; unmaintained on Windows for 5+ years).
   - **WSL2-managed execution** is adopted as the official Windows path: the native Windows Node Manager (`proximax-sirius-core.exe`) supervises the Linux engine running inside WSL2 with transparent port forwarding and ext4 performance.
   - All Windows Electron GUI packaging work is paused until the Linux bare-metal engine is built and verified.

3. **Immediate Priority on Ubuntu**:
   - **Bare-metal Linux engine packaging & verification** in `cpp-xpx-chain`: Bundle `sirius.bc`, dynamic plugins (`libcatapult.*.so`), `librocksdb.so`, Boost 1.81.0, and OpenSSL 3 using `patchelf` / `DT_RUNPATH=$ORIGIN/../lib`.
   - Test directly on bare-metal Ubuntu (sync blocks, verify P2P on port 7900, POS+ harvesting).
   - Once verified, this engine bundle serves BOTH Linux bare-metal nodes AND the Windows WSL2 backend.

---

## 2. Status Matrix Across All 6 Targets

| Target | macOS (ARM64) | Linux (x86_64) | Windows (x86_64) |
| :--- | :---: | :---: | :---: |
| **1. C++ Sirius Engine** (`cpp-xpx-chain`) | **(c) Verified Runtime** | **(b) Docker only (bare-metal unverified)** | **(a) Deprioritized (uses WSL2)** |
| **2. Node Manager & Shell** (`proximax-sirius-core`) | **(c) Verified Runtime** | **(b) Compiles & CI Passed (ready for VM test)** | **(b) Compiles & CI Passed (GUI on hold)** |

---

## 3. Completed Security & Hygiene Audits (Latest Commits)

All 5 audit findings have been resolved with regression tests committed on `main`:
- **SEC-01**: Dedicated `chainconfig/manager.compat.json` created with its own Ed25519 public key (`1017f00b8e9c...`). Fixed workflow `.github/workflows/release.yml` using `NODE_MANAGER_RELEASE_PRIVATE_KEY`.
- **SEC-02 / SEC-03**: Cumulative mid-loop decompression limit enforced in `pkg/snapshot/manager.go` with zero-orphan cleanup (`cleanupCreated()`). Added 5MB stream caps to manifests.
- **SEC-04**: `cmd/sign-release` updated with `-key-env` flag to prevent private keys leaking into the `/proc` process table.
- **SEC-05**: Strict HTTP `POST` method check enforced on `/api/engine/reset` endpoint.
- **Cross-Platform Fixes**: Windows NTFS file permission checks, real `.zip` compression on Windows, and POSIX `rlimit` build tags (`rlimit_posix.go` vs `rlimit_windows.go`).

---

## 4. How to Resume Work on the Ubuntu Machine

### Step 1: Pull or Clone Repositories
```bash
# 1. Node Manager & Cockpit
git clone https://github.com/proximax-storage/proximax-sirius-core.git
# Or pull:
cd proximax-sirius-core && git checkout main && git pull

# 2. C++ Sirius Engine
git clone --recursive https://github.com/proximax-storage/cpp-xpx-chain.git
# Or pull:
cd cpp-xpx-chain && git checkout feature/reducedDataSize && git pull
```

### Step 2: System Build Dependencies for Ubuntu
```bash
sudo apt update
sudo apt install -y build-essential cmake ninja-build libssl-dev \
    libgtest-dev libbenchmark-dev patchelf golang-go
```

### Step 3: First Action Item on Ubuntu
Open Antigravity on the Ubuntu machine and prompt:
> "Resume work from SESSION_HANDOFF.md: Let's build and verify the standalone native Linux engine bundle for cpp-xpx-chain using patchelf and RPATH, then test block sync on this Ubuntu system."
