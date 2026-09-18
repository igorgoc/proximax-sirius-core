# ProximaX Sirius Mainnet Peer Node (Linux)

A high-performance ProximaX Sirius Mainnet Peer Node with a built-in pure browser Web Cockpit dashboard.

---

## Quick Start

1. **Start Node**: Run `./start.sh`
   - Starts the supervisor in the background and launches the Sirius Catapult engine.
   - Automatically opens your web browser to the Cockpit at **http://localhost:8080** (or opens port 8080 on headless servers).
2. **Stop Node**: Run `./stop.sh`
   - Gracefully stops the supervisor, block disruptors, and engine, cleanly flushing RocksDB state to disk.

---

## Web Cockpit Dashboard

Once started, access **http://localhost:8080** in your web browser:

- **Overview & Status**: Live block height, sync status, network hash rate, peer connections, and harvester status.
- **Harvesting Configuration**: Configure POS+ harvesting keys with password-masked inputs and automated balance checks.
- **Fast-Sync Snapshot**: One-click streaming snapshot restore to quickly synchronize the entire blockchain history.
- **Live Logs**: Streaming real-time logs from both the Go supervisor and C++ Catapult engine with level filtering.
- **Node Controls**: In-browser Start, Stop, and Restart controls.

---

## Directory Layout

```
proximax-sirius-core-linux/
├── sirius-core              # Native Linux supervisor binary
├── start.sh                 # Start script (starts node in background & opens web browser)
├── stop.sh                  # Stop script (gracefully stops node)
├── README.md                # This documentation
├── bin/                     # Native engine binaries and shared libraries
├── chainconfig/             # Configuration properties and blockchain data
│   ├── resources/           # Network and harvesting configuration properties
│   ├── data/                # Blockchain database (RocksDB + flat files)
│   └── logs/                # Node and engine log files
└── scripts/
    └── linux/               # Helper and advanced management scripts
```

---

## Helper Scripts Reference (`scripts/linux/`)

For advanced operations, service management, or terminal usage, the following scripts are located in `scripts/linux/`:

| Script / Directory | Description | Usage |
| :--- | :--- | :--- |
| **`start.sh`** | Background launcher that checks port availability, architecture, permissions, and starts the node. | `./scripts/linux/start.sh` |
| **`stop.sh`** | Graceful shutdown script that communicates with the supervisor API to cleanly flush database state. | `./scripts/linux/stop.sh` |
| **`restart.sh`** | Convenience helper that executes a clean stop followed by a start. | `./scripts/linux/restart.sh` |
| **`start-node.sh`** | Foreground runner for terminal debugging or container entry points. | `./scripts/linux/start-node.sh` |
| **`run.sh`** | Developer build and run script. Compiles the Go backend and React UI when working from the source git repository. | `./scripts/linux/run.sh` |
| **`reset_to_genesis.sh`** | Database maintenance utility to reset local chain data back to height 1 / genesis block. | `./scripts/linux/reset_to_genesis.sh` |
| **`systemd/`** | Systemd service unit configuration file (`proximax-sirius.service`) for running the node as a Linux system service. | See `scripts/linux/systemd/` |
