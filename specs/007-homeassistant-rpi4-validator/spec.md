# Feature Specification: Headless ProximaX Sirius Validator Add-on for Home Assistant (RPi4 / ARM64)

**Feature Branch**: `007-homeassistant-rpi4-validator`

**Created**: 2026-09-18 (Updated: Pure Headless Catapult Architecture)

**Status**: Ready for Implementation

**Input**: Create an ultra-lightweight, pure headless Home Assistant Add-on for the ProximaX Sirius Mainnet Peer Validator (`sirius.bc`), specifically tailored for a Raspberry Pi 4 (4GB RAM) with an attached 250GB SSD, utilizing Home Assistant's native configuration schema for masked key entry, automated fast-sync initialization, and built-in supervisor log streaming.

---

## User Scenarios & Requirements

### User Story 1 - Native Key Entry via Home Assistant Add-on Configuration (Priority: P1)

As a node operator running Home Assistant OS on a Raspberry Pi 4, I want to enter my Sirius `boot_key` and `harvest_key` directly in the Home Assistant Add-on "Configuration" tab with password-masked input fields, so that I don't need a separate web manager, external server, or manual file editing.

**Acceptance Scenarios**:
1. **Given** the Add-on Configuration tab in Home Assistant, **When** the user inputs `boot_key` and `harvest_key`, **Then** the inputs are masked by Home Assistant with password reveal toggles.
2. **Given** the add-on starting up, **When** `run.sh` executes, **Then** it securely populates `/data/chainconfig/resources/config-harvesting.properties` and `config-user.properties` with strict `0600` permissions.

---

### User Story 2 - Automated First-Boot Snapshot Fast-Sync onto SSD (Priority: P1)

As a validator operator, I want the add-on to automatically stream and decompress the official fast-sync snapshot directly onto the SSD on first launch if the database is uninitialized, so that the node is ready to harvest blocks within minutes without manual tar downloads.

**Acceptance Scenarios**:
1. **Given** an empty `/data/chainconfig/data/` directory, **When** the add-on starts with `fast_sync: true`, **Then** `run.sh` streams the official Zstandard archive directly from Hugging Face into `/data/chainconfig/data/` with progress logged to Home Assistant.
2. **Given** the database already initialized (height > 1), **When** restarting the add-on, **Then** snapshot restore is skipped and `sirius.bc` resumes block synchronization immediately.

---

### User Story 3 - P2P Validation & POS+ Block Harvesting (Priority: P1)

As a Sirius Mainnet validator, I want `sirius.bc` to connect to the P2P network on port `7900` and actively harvest blocks on my Raspberry Pi 4 24/7.

**Acceptance Scenarios**:
1. **Given** the add-on running on RPi 4, **When** port 7900 is forwarded on the home router, **Then** `sirius.bc` accepts peer connections and participates in block consensus.
2. **Given** the Home Assistant Log viewer, **When** viewing add-on logs, **Then** real-time block synchronization and harvesting messages from `sirius.bc` stream in real time.

---

### User Story 4 - 4GB RAM & Zero Overhead Optimization (Priority: P2)

As a Raspberry Pi 4 owner running Home Assistant and Sirius on the same device, I want zero auxiliary memory overhead (no Go web server, no embedded React UI) so all available hardware resources are dedicated to Catapult engine performance and Home Assistant responsiveness.

**Acceptance Scenarios**:
1. Total add-on memory consumption stays between ~1.0 GB and 1.5 GB.
2. RocksDB block cache is tuned to 512 MB in `config-node.properties`.

---

## Technical Invariants

- **FR-001**: Add-on MUST run the native C++ Catapult engine (`sirius.bc`) directly inside a lightweight Debian 12 ARM64 container.
- **FR-002**: Private keys MUST NEVER appear in console logs; `run.sh` must write them directly to properties files with `chmod 0600`.
- **FR-003**: All blockchain data and configuration MUST reside in `/data/chainconfig` to persist on the 250GB SSD across container rebuilds.
- **FR-004**: P2P port `7900/tcp` MUST be exposed on the host network.
- **FR-005**: Add-on MUST automatically remove stale lock files (`server.lock`, `statedb/*/LOCK`) prior to launching `sirius.bc`.
