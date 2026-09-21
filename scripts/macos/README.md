# ProximaX Sirius Mainnet Peer Node (macOS)

A high-performance ProximaX Sirius Mainnet Peer Node with a built-in pure browser Web Cockpit dashboard.

---

## Quick Start

1. **Start Node**: Double-click `start.command` in Finder (or run `./start.sh` in Terminal).
   - Starts the supervisor in the background and launches the Sirius Catapult engine.
   - Automatically opens Safari / default web browser to the Cockpit at **http://localhost:8080**.
2. **Stop Node**: Run `./stop.sh` in Terminal.
   - Gracefully stops the supervisor, block disruptors, and engine, cleanly flushing RocksDB state to disk.

---

## Web Cockpit Dashboard

Once started, access **http://localhost:8080** in Safari, Chrome, Edge, or Firefox:

- **Overview & Status**: Live block height, sync status, network hash rate, peer connections, and harvester status.
- **Harvesting Configuration**: Configure POS+ harvesting keys with password-masked inputs and automated balance checks.
- **Fast-Sync Snapshot**: One-click streaming snapshot restore to quickly synchronize the entire blockchain history.
- **Live Logs**: Streaming real-time logs from both the Go supervisor and C++ Catapult engine with level filtering.
- **Node Controls**: In-browser Start, Stop, and Restart controls.

---

## Directory Layout

```
proximax-sirius-core-mac/
├── sirius-core              # Native macOS supervisor binary
├── start.command            # Double-clickable macOS Finder launcher
├── start.sh                 # Start script (starts node in background & opens web browser)
├── stop.sh                  # Stop script (gracefully stops node)
├── README.md                # This documentation
├── bin/                     # Native engine binaries and macOS dynamic libraries
├── chainconfig/             # Configuration properties and blockchain data
│   ├── resources/           # Network and harvesting configuration properties
│   ├── data/                # Blockchain database (RocksDB + flat files)
│   └── logs/                # Node and engine log files
└── scripts/
    └── macos/               # Helper and advanced management scripts
```

---

## Helper Scripts Reference (`scripts/macos/`)

For advanced operations, management, or terminal usage, the following scripts are located in `scripts/macos/`:

| Script / Tool | Description | Usage |
| :--- | :--- | :--- |
| **`start.sh`** | Background launcher that checks port availability, architecture, Gatekeeper attributes, and starts the node. | `./scripts/macos/start.sh` |
| **`start.command`** | Double-clickable macOS script wrapper that launches `start.sh`. | Double-click in Finder |
| **`stop.sh`** | Graceful shutdown script that communicates with the supervisor API to cleanly flush database state. | `./scripts/macos/stop.sh` |
| **`restart.sh`** | Convenience helper that executes a clean stop followed by a start. | `./scripts/macos/restart.sh` |
