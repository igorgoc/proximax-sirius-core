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

---

## 5. Windows WSL2 Onboarding Architecture Specification

When Windows development resumes, the Windows Node Manager will feature a **first-class onboarding wizard** instead of cryptic runtime errors:

1. **Detection States (`wsl.exe --status` / `wsl.exe -l -v`)**:
   - `WSL_NOT_INSTALLED`: Neither WSL nor VirtualMachinePlatform enabled.
   - `WSL_V1_ONLY`: WSL installed but default version is 1 (needs upgrade to v2 for RocksDB performance).
   - `WSL2_NO_DISTRO`: WSL2 kernel installed, but no Ubuntu/Sirius distribution imported.
   - `WSL2_READY`: Ready to execute `sirius.bc`.

2. **User Experience Flow (First-Run Modal)**:
   - **Title**: *"High-Performance Blockchain Subsystem Setup"*
   - **Explanation**: Plainly explains: *"To achieve high-speed block validation and RocksDB storage without Docker overhead, ProximaX Sirius runs its engine inside Windows Subsystem for Linux (WSL2)."*
   - **One-Click Action**:
     - If missing: A prominent button *"Enable Blockchain Subsystem"* that launches an elevated prompt (`powershell -Command "Start-Process wsl -ArgumentList '--install --no-distribution' -Verb RunAs"`).
     - Writes local state marker `{ "wsl_install_initiated": true }` to `chainconfig/cache.json`.
     - Followed by a modal dialog: *"Windows requires a restart to finalize this subsystem. When you restart your PC and re-open ProximaX Sirius, setup will resume automatically."*
   - **Manual Mode Toggle**: Direct copyable PowerShell commands for advanced users who prefer configuring WSL manually.

3. **Reboot-Resume Architecture**:
   - **Dynamic Startup Check**: The Go backend executes the WSL status probe (`wsl.exe --status`) on *every single application startup*, not just on manual button clicks.
   - **Resume Flow**: If `wsl_install_initiated == true` and `wsl.exe --status` reports WSL2 is active, the app automatically clears the marker, shows *"Blockchain subsystem detected successfully! Finalizing setup..."*, and advances directly to the next stage (distro provisioning or Cockpit launch) without showing the initial "Enable Subsystem" screen again.

4. **Failure & Denial Paths (Explicit Handlers, Never Silent Crashes)**:
   - **UAC Denial (Exit code 1223 / Access Denied)**:
     - *UI Message*: *"Administrator permissions were declined. ProximaX Sirius requires permission once to enable the Windows virtualization feature."*
     - *Action*: Displays a **"Try Again"** button alongside an expandable **"Manual PowerShell Instructions"** panel.
   - **BIOS/UEFI Hardware Virtualization Disabled (`0x80370102` / `Wsl/Service/CreateVm`)**:
     - *UI Message*: *"Hardware Virtualization is disabled in your computer's BIOS/UEFI. WSL2 requires CPU virtualization (Intel VT-x or AMD-V) to run."*
     - *Action*: Shows a dedicated remediation card with a clickable guide link: *"How to enable virtualization in BIOS for your motherboard"* and a **"Re-check Status"** button.
   - **Network Timeout / Kernel Download Failure**:
     - *UI Message*: *"Windows failed to download the Linux kernel update automatically."*
     - *Action*: Provides a direct download button for Microsoft's official offline MSI installer (`wsl_update_x64.msi`).
   - **Corporate Group Policy / MDM Restriction**:
     - *UI Message*: *"WSL2 installation is restricted by your system administrator or organization security policy."*
     - *Action*: Displays IT policy guidelines and diagnostic details.


