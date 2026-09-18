# ProximaX Sirius Mainnet Peer Node Constitution

## Core Principles

### I. Key Privacy & Cryptographic Isolation (NON-NEGOTIABLE)
- Raw private keys MUST NEVER be displayed in unmasked UI state, result cards, overview panels, or console logs.
- Display only public keys, addresses, and transaction hashes.
- Use password masking with reveal/hide toggle for all sensitive key inputs in frontend.
- Ensure private keys are only persisted securely in `chainconfig/resources/config-harvesting.properties` and `config-user.properties` with restricted permissions (0600 on POSIX, tightened NTFS ACL on Windows).
- Enforce strict separation between `bootKey` (P2P transport identity) and `harvestKey` (POS+ block harvesting identity). Never mirror harvest key to boot key.

### II. State & Storage Atomicity (No Flat-File Byte Surgery)
- Catapult stores chain state across flat files (`supplemental.dat`, `BlockDifficultyCache.dat`, `index.dat`, `00000/*.dat`) and RocksDB column families (`statedb/`).
- Flat files MUST NEVER be manually modified or rolled back independently of RocksDB. Desynchronizing them corrupts the Merkle state tree and causes fatal validation failures.
- `catapult.recovery` is the sole authority for reconciling WAL commits and state when block height > 1.
- At block height ≤ 1, dirty partial `statedb` from aborted boots must be cleared so `NemesisBlockLoader` boots cleanly.

### III. Cross-Platform Parity & Isolation
- **Native Host Supervisor**: Compiled as native Go binary on each platform (macOS ARM64, Linux x86_64, Windows x64 `sirius-core.exe`), serving Cockpit web UI on port 8080.
- **Engine Execution**: Runs bare-metal on macOS/Linux and as Linux ELF x86_64 inside WSL2 (Ubuntu-22.04) on Windows.
- **Pure Release Bundles**: Release packages for a given OS must NEVER contain foreign OS binaries, shell scripts, or internal test files.

### IV. Fast-Sync & Resource Limits
- Use direct streaming decompression from official snapshot archives without intermediate tar file saving to eliminate disk overhead.
- Automatically clear `*.lock` files on supervisor startup.
- Enforce Docker log rotation (`max-size=250m`, `max-file=3`) and Sirius log caps (`rotationSize=25MB`, `maxTotalSize=250MB`).

### V. Fail-Closed Security & Release Integrity
- Release manifests (`SHA256SUMS`) must be cryptographically signed using Ed25519 node manager release keys.
- CI workflows must fail-closed and refuse to publish unsigned release archives if release secrets are missing.

## Development Workflow & Quality Gates
1. **Spec-Driven**: Every feature, bug fix, or refactor must have a clear problem specification and task checklist before editing code.
2. **Automated Verification**: All Go changes must pass `go test ./...` and `npm run build`. C++ changes must compile and pass engine boot gate tests.
3. **Regression Guard**: Any fix for engine state recovery must include a regression verification test.

## Governance
This Constitution defines the foundational engineering invariants of the ProximaX Sirius Peer Node and Cockpit. No AI coding agent or developer commit may violate these core principles.

**Version**: 1.0.0 | **Ratified**: 2026-09-17 | **Status**: Active
