# Implementation Plan - Headless ProximaX Sirius Validator Add-on (RPi4 / ARM64)

## Architecture Summary
Deploying a pure headless Home Assistant Add-on running the native `sirius.bc` engine directly on a Raspberry Pi 4 (aarch64) with 250GB SSD storage.

---

## Constitution Check

- **I. Key Privacy**: `run.sh` writes keys from Home Assistant options to `config-harvesting.properties` with `0600` permissions. Keys are never printed to logs or console.
- **II. State Atomicity**: All RocksDB and flat files reside in `/data/chainconfig/data/`. `catapult.recovery` handles crash reconciliation.
- **III. Cross-Platform Parity**: Uses official Linux ARM64 Catapult engine binaries and configuration templates.
- **IV. Fast-Sync & Resource Limits**: Direct streaming decompression (`curl | tar --zstd -x`) from Hugging Face snapshot; RocksDB cache capped at 512MB for 4GB RPi4.

---

## Phased Tasks

### Phase 1 - Add-on Manifest & Container Definition
- Create `addon/config.yaml` (schema for masked keys, port 7900 mapping, aarch64 arch).
- Create `addon/Dockerfile` (Debian 12 Bookworm base with RocksDB, Boost, Zstandard, and Catapult dependencies).
- Create `addon/DOCS.md` (Home Assistant add-on documentation).

### Phase 2 - Startup & Configuration Generator Script (`addon/run.sh`)
- Implement `bashio` options parsing for `boot_key`, `harvest_key`, `friendly_name`, `fast_sync`.
- Implement dynamic configuration properties templating in `/data/chainconfig/resources/`.
- Implement automated fast-sync snapshot streaming into `/data/chainconfig/data/`.
- Implement stale lock cleaning (`server.lock`, `statedb/*/LOCK`).
- Launch `sirius.bc` with stdout directed to Home Assistant supervisor log stream.

### Phase 3 - Engine & Dependency Staging
- Stage ARM64 `sirius.bc`, `catapult.recovery`, and shared libraries into `addon/bin/`.
- Stage baseline configuration templates into `addon/chainconfig/`.

### Phase 4 - Live Staging & Verification on RPi4 (192.168.1.14)
- Copy `addon/` to `/addons/proximax-sirius` on Home Assistant via SSH.
- Test install and launch from Home Assistant Add-on store.
- Verify block synchronization and log output.
