# ProximaX Sirius Mainnet Peer Node (Native GUI & Node Manager)

A high-performance, **Bitcoin Core-inspired** standalone desktop and web application for running, configuring, and managing a **ProximaX Sirius Chain Mainnet Peer Node, POS+ Block Harvester, and DFMS Storage Replicator**.

Built as a lightweight native management engine (Go backend + React/TypeScript frontend) interfacing directly with the native C++ ProximaX Sirius Core blockchain binary (`cpp-xpx-chain`) on macOS (Apple Silicon ARM64 & Intel) and Linux — with **zero Docker overhead**.

---

## Architecture Overview

```
                      +-----------------------------------+
                      |      Operator Browser / GUI       |
                      |       http://localhost:3080       |
                      +-----------------+-----------------+
                                        | (HTTP / REST / SSE)
                      +-----------------v-----------------+
                      |    Native Go Node Manager Core    |
                      |   Process Supervisor & Telemetry  |
                      +---+-------------+---------------+--+
                          |             |               |
             (IPC / Unix) |             | (Disk I/O)    | (TLS 7900)
    +---------------------v-+     +-----v-------+  +----v--------------------+
    |  Sirius C++ Engine    |     | RocksDB /   |  | ProximaX Sirius Mainnet |
    |  (sirius.bc / WAL)    |     | Chunk Files |  | P2P Validator Network   |
    +-----------------------+     +-------------+  +-------------------------+
```

- **Engine**: Native compiled Sirius C++ binary (`sirius.bc`) with optimized async WAL and RocksDB state cache.
- **Manager Daemon**: Lightweight Go daemon managing process lifecycles, configuration persistence, disaster recovery, and network health.
- **Cockpit GUI**: Modern dark-themed operator interface built with React, Vite, Tailwind CSS, and Lucide icons.
- **Storage Subsystem**: Supports Bitcoin-style 65,536-blocks chunked storage with seamless external SSD relocation.

---

## Prerequisites

- **macOS**: Apple Silicon (M1/M2/M3/M4) or Intel (macOS 12.0+)
- **Linux**: Ubuntu 22.04+ / Debian 12+ (x86_64 or aarch64)
- **Windows**: Windows 10 (Version 2004 / Build 19041+) or Windows 11 with **WSL2** enabled. *(Note: The Windows Node Manager GUI runs natively on Windows and automatically detects or guides the one-time setup of the high-performance Linux Sirius engine subsystem inside WSL2).*
- **Node.js**: v20+ with `npm`
- **Go**: v1.22+
- **Disk Space**: At least 50 GB free disk space (external high-speed SSD recommended)

## Pre-Compiled Standalone Releases

Pre-compiled standalone packages with the React web cockpit embedded inside are available under [GitHub Releases](https://github.com/igorgoc/proximax-sirius-core/releases).

### macOS (Apple Silicon ARM64 & Intel)

#### Option 1: One-Line Terminal Setup (Bypasses Browser Quarantine)
```bash
curl -sL https://github.com/igorgoc/proximax-sirius-core/releases/download/v1.9.8/proximax-sirius-darwin-arm64-1.9.8.tar.gz | tar -xz
cd proximax-sirius-core && ./start.command
```
*(Files downloaded via `curl` do not receive browser quarantine attributes and launch immediately with zero Gatekeeper prompts).*

#### Option 2: If Downloaded Via Web Browser (Safari / Chrome)
1. Extract the downloaded `proximax-sirius-darwin-arm64-1.9.8.tar.gz`.
2. In the extracted folder, double-click **`start.command`** (or in Terminal run `xattr -cr . && ./start.command`).
3. If macOS Gatekeeper flags the open-source binary as unverified, right-click `start.command` → **Open** → **Open**, or navigate to **System Settings > Privacy & Security** and click **Open Anyway**.

---

## Quick Start (Build from Source)

### 1. Launch the Node & GUI
In your terminal, navigate to the repository directory and run:

```bash
./run.sh
```

`run.sh` will:
1. Automatically set optimal file descriptor limits (`ulimit -n 65536`) for RocksDB.
2. Build the React frontend into static assets embedded within the Go binary.
3. Compile and launch the native manager daemon on port **3080**.
4. Perform readiness healthchecks and open the dashboard.

### 2. Access the Dashboard & Automatic Engine Setup
Open your browser at:
👉 **[http://localhost:3080](http://localhost:3080)**

> **Automatic Initial Setup:** Download the Node Manager for your platform; it will automatically fetch and verify the matching Sirius Engine on first launch (requires internet access once). The application cryptographically validates the release manifest via Ed25519 signatures and SHA-256 integrity checks before extracting the native engine binary.

On first launch, the **Setup Wizard** will also guide you through:
- Node network identity and friendly name.
- Storage path configuration (internal or external SSD).
- High-speed snapshot synchronization.
- Harvesting and storage replicator credentials.

### 3. Stopping the Node
To safely shut down the daemon, flush RocksDB caches, and stop background processes:

```bash
./stop.sh
```

---

## Key Capabilities

### 1. POS+ Harvester Cockpit
- Real-time harvester synchronization status, committee voting eligibility, and block generation metrics.
- Track total blocks harvested, fees earned in XPX, and average blocks per day.
- Delegated account linkage diagnostics against public Sirius REST nodes.

### 2. DFMS Storage Replicator
- Monitor distributed file management (DFMS) storage node operations.
- Metrics for active shards, allocated storage space, streaming units (SI), and storage units (SO).
- Drive path management with native OS file dialog support.

### 3. Fast Snapshot Synchronization & Live Streaming Restorer
- High-speed direct-streaming snapshot downloader and unpacker.
- Memory-streamed decompression directly into chunked block files (`blocks.dat`, `hashes.dat`, `statements.dat`) without intermediate archive extraction overhead.
- Real-time ETA, download bandwidth, and extraction progress meters.

### 4. Hardware Vault & Key Security Invariants
- **Zero Raw Key Exposure**: Raw private keys are never exposed in UI overview cards or API responses.
- **Strict Key Separation**: Enforces complete architectural separation between P2P transport identity (`bootKey`) and consensus block harvesting identity (`harvestKey`).
- **Encrypted Disaster Recovery**: Export and import password-protected, encrypted recovery packages (`.drpkg`) using Argon2id key derivation and authenticated AES-256-GCM encryption.

---

## Configuration Reference

Node configuration is maintained in `chainconfig/resources/`:

| File | Key | Description | Default |
| :--- | :--- | :--- | :--- |
| `config-manager.properties` | `bootkey.source` | Sourcing method for P2P transport key (`generated` or `custom`) | `generated` |
| `config-manager.properties` | `data.path` | Local filesystem path where chain data is stored | `./chainconfig/data` |
| `config-harvesting.properties` | `harvestKey` | Delegated harvesting remote account private key | *(Unset / Template)* |
| `config-harvesting.properties` | `isAutoHarvestingEnabled` | Automatic block creation on eligible rounds | `true` |
| `config-user.properties` | `bootKey` | Node P2P transport session identity | *(Auto-generated)* |
| `config-node.properties` | `friendlyName` | Public validator node nickname | `sirius-mainnet-peer` |

---

## Network Ports

| Port | Protocol | Purpose |
| :---: | :---: | :--- |
| **3080** | TCP (HTTP) | Sirius Core Web Cockpit & Manager REST API |
| **7900** | TCP | Sirius Mainnet P2P Transport & Block Synchronization |
| **7901** | TCP | Sirius Peer API Gateway |
| **7902** | TCP | Sirius Broker / Messaging Queue |
| **7903** | TCP | Sirius Distributed Byzantine Agreement (DBRB) |
| **7904** | TCP / UDP | Sirius DFMS Storage Replicator Data Stream |
| **3000** | TCP (HTTP) | Public Blockchain REST API Gateway |

---

## Upstream Relationship

This project is the native desktop and operator control software for the **ProximaX Sirius Chain**, derived from and compatible with the official [ProximaX Sirius Core C++ engine](https://github.com/proximax-sirius/cpp-xpx-chain) (Catapult architecture).

---

## License

Licensed under the **Apache License, Version 2.0**. See the [LICENSE](LICENSE) file for details.
