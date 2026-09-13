# ProximaX Sirius Mainnet Peer Node (Native Cockpit & Node Manager)

A high-performance, **Bitcoin Core-inspired** standalone desktop and web application for running, configuring, and managing a **ProximaX Sirius Chain Mainnet Peer Node, POS+ Block Harvester, and DFMS Storage Replicator**.

Built as a lightweight native management engine (Go backend + React/TypeScript frontend) interfacing directly with the native C++ ProximaX Sirius Core blockchain binary (`cpp-xpx-chain`) on **macOS**, **Linux**, and **Windows** — with **zero Docker overhead and zero Electron bloat**.

---

## Architecture Overview

```
                      +-----------------------------------+
                      |     Operator Browser / Cockpit    |
                      |       http://localhost:8080       |
                      +-----------------+-----------------+
                                        | (HTTP / REST / SSE / WebSockets)
                      +-----------------v-----------------+
                      |    Native Go Node Manager Core    |
                      |   Process Supervisor & Telemetry  |
                      +---+-------------+---------------+--+
                          |             |               |
             (IPC/Signals)|             | (Disk I/O)    | (TLS 7900)
    +---------------------v-+     +-----v-------+  +----v--------------------+
    |  Sirius C++ Engine    |     | RocksDB /   |  | ProximaX Sirius Mainnet |
    |  (sirius.bc / WAL)    |     | Chunk Files |  | P2P Validator Network   |
    +-----------------------+     +-------------+  +-------------------------+
```

### Cross-Platform Architecture Strategy

| Operating System | Node Manager (UI & Supervisor) | Sirius Catapult Engine (`sirius.bc`) | Storage & Filesystem |
| :--- | :--- | :--- | :--- |
| **macOS** (Apple Silicon & Intel) | Native Darwin binary (`sirius-core`) | Native Darwin C++ binary (`bin/sirius.bc`) with dyld RocksDB | Native APFS / External NVMe SSD |
| **Linux** (Ubuntu / Debian x86_64) | Native Linux ELF binary (`sirius-core`) | Native Linux ELF binary (`bin/sirius.bc`) with glibc 2.35+ | Native ext4 / XFS filesystem |
| **Windows** (Windows 10 / 11 x64) | Native Windows x64 binary (`sirius-core.exe`) | Linux ELF binary (`sirius.bc`) orchestrated via **WSL2** | Native ext4 (`/var/lib/sirius`) or NTFS |

- **Why WSL2 on Windows?**  
  Direct Windows MSVC ports for Catapult are deprecated upstream. Running the C++ engine inside WSL2 provides **100% binary compatibility** with the verified Linux consensus engine, POSIX signal handling, and native ext4 write speeds for RocksDB multi-gigabyte state flushes without Docker overhead.

---

## Operating System Guides (Pre-Compiled Releases)

Pre-compiled standalone packages with the embedded React cockpit are available under [GitHub Releases](https://github.com/igorgoc/proximax-sirius-core/releases).

### 1. macOS (Apple Silicon ARM64 & Intel)

#### Start Node:
- **Option A (Finder)**: Extract the `.tar.gz` archive and double-click **`start.command`**.
- **Option B (Terminal)**:
  ```bash
  tar -xzf proximax-sirius-darwin-arm64-1.9.8.tar.gz
  cd proximax-sirius-core
  ./start.sh
  ```
- **Option C (One-Line cURL Install — Bypasses macOS Gatekeeper Quarantine)**:
  ```bash
  curl -sL https://github.com/igorgoc/proximax-sirius-core/releases/download/v1.9.8/proximax-sirius-darwin-arm64-1.9.8.tar.gz | tar -xz
  cd proximax-sirius-core && ./start.command
  ```

#### Stop & Restart Node:
- **Stop**: Run `./stop.sh` (or click "Stop Node" in the Web Cockpit).
- **Restart**: Run `./restart.sh`.

#### Access Dashboard:
- Browser opens automatically at **`http://localhost:8080`**.

> **macOS Gatekeeper Tip**: If downloaded via Safari/Chrome and blocked, right-click `start.command` → **Open** → **Open**, or run `xattr -cr .` inside the folder.

---

### 2. Linux (Ubuntu 22.04+ / Debian 12+)

#### Option A: Standalone Portable Tarball (No Root Required)
```bash
tar -xzf proximax-sirius-linux-amd64-1.9.8.tar.gz
cd proximax-sirius-core

# Start the node & manager
./start.sh

# Stop or restart gracefully
./stop.sh
./restart.sh
```

#### Option B: Debian / Ubuntu Package (`.deb` with systemd Service)
```bash
# Install the deb package
sudo dpkg -i proximax-sirius-core_1.9.8_amd64.deb

# Enable and start the systemd service
sudo systemctl enable --now proximax-sirius

# Check service status & logs
sudo systemctl status proximax-sirius
journalctl -u proximax-sirius -f
```

#### Access Dashboard:
- Open your browser at **`http://localhost:8080`** (or remote server IP `http://<server-ip>:8080`).

---

### 3. Windows (Windows 10 / 11 64-bit)

#### Start Node:
1. Extract `proximax-sirius-windows-amd64-1.9.8.zip` to a folder (e.g. `C:\proximax-sirius-core`).
2. Double-click **`start.bat`** (or in PowerShell run `.\start.bat`).
3. Your default web browser will open **`http://localhost:8080`**.

#### Stop & Restart Node:
- **Stop**: Double-click **`stop.bat`** (or in PowerShell run `.\stop.bat`).
- **Restart**: Double-click **`restart.bat`** (or in PowerShell run `.\restart.bat`).

#### Windows WSL2 Onboarding Wizard:
- On first launch, the Node Manager automatically inspects the Windows Subsystem for Linux (`wsl.exe --status`).
- If WSL2 or Ubuntu is not yet installed, the Web Cockpit presents a **1-Click Subsystem Setup Wizard** with automatic elevation (`Start-Process wsl -ArgumentList '--install --no-distribution' -Verb RunAs`) and reboot-resume state persistence.

> **Windows Defender SmartScreen**: If prompted with "Windows protected your PC", click **"More info"** → **"Run anyway"**.

---

## Developer Quick Start (Build from Source)

### macOS & Linux:
```bash
git clone https://github.com/igorgoc/proximax-sirius-core.git
cd proximax-sirius-core

# Builds React frontend, compiles Go supervisor, and launches on port 8080
./scripts/linux/run.sh
```

### Windows:
```cmd
git clone https://github.com/igorgoc/proximax-sirius-core.git
cd proximax-sirius-core

:: Builds React UI and compiles Windows native supervisor
scripts\windows\run.bat
```

---

## Key Capabilities & Invariants

### 1. Mandatory Harvest Key Enforcement
- The Sirius Catapult engine will **refuse to start** unless a valid 64-character hexadecimal `harvestKey` is configured.
- Configure your remote harvesting account key in the **Validator Settings** tab or via the initial **Setup Wizard**.

### 2. Hardware Vault & Key Privacy
- **Zero Raw Key Exposure**: Raw private keys are never displayed in UI overview cards, public API responses, or plain text logs.
- **Strict Key Separation**: Enforces strict isolation between P2P transport identity (`bootKey`) and POS+ consensus harvesting identity (`harvestKey`). Never mirror harvest key to boot key.
- **Encrypted Disaster Recovery**: Export and import password-protected recovery packages (`.drpkg`) using Argon2id key derivation and authenticated AES-256-GCM encryption.

### 3. High-Speed Snapshot Streaming Sync
- Direct streaming decompression from the official HuggingFace Mainnet snapshot (`.tar.zst`) without saving intermediate multi-gigabyte archive files to disk.
- Automatically re-initializes block storage (`data/`) and RocksDB state (`statedb/`) cleanly in under 2 minutes.

### 4. POS+ Harvester & Storage Replicator Cockpit
- Real-time harvester synchronization status, committee voting eligibility, and block generation metrics.
- Track total blocks harvested, fees earned in XPX, and average blocks per day.
- Monitor DFMS storage node operations, active shards, and allocated storage units.

---

## Configuration Reference

Runtime configuration files reside in `chainconfig/resources/`:

| File | Key | Description | Default |
| :--- | :--- | :--- | :--- |
| `config-harvesting.properties` | `harvestKey` | Delegated harvesting remote account private key (Mandatory) | *(Configured via UI)* |
| `config-harvesting.properties` | `isAutoHarvestingEnabled` | Automatic block creation on eligible rounds | `true` |
| `config-user.properties` | `bootKey` | Node P2P transport session identity | *(Auto-generated)* |
| `config-user.properties` | `data.path` | Local or external SSD storage path | `./chainconfig/data` |
| `config-node.properties` | `friendlyName` | Public validator node nickname | `sirius-mainnet-peer` |

---

## Network Ports

| Port | Protocol | Purpose |
| :---: | :---: | :--- |
| **8080** | TCP (HTTP) | Sirius Core Web Cockpit & Manager REST API |
| **7900** | TCP | Sirius Mainnet P2P Transport & Block Synchronization |
| **7901** | TCP | Sirius Peer API Gateway |
| **7902** | TCP | Sirius Broker / Messaging Queue |
| **7903** | TCP | Sirius Distributed Byzantine Agreement (DBRB / Fast Finality) |
| **7904** | TCP / UDP | Sirius DFMS Storage Replicator Data Stream |
| **3000** | TCP (HTTP) | Public Blockchain REST API Gateway |

---

## License

Licensed under the **Apache License, Version 2.0**. See the [LICENSE](LICENSE) file for details.
