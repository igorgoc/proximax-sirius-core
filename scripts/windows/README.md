# ProximaX Sirius Mainnet Peer Node (Windows)

A high-performance ProximaX Sirius Mainnet Peer Node with a built-in pure browser Web Cockpit dashboard.

---

## Quick Start

1. **Start Node**: Double-click `start.bat` (or run `.\start.bat` in PowerShell/CMD).
   - Starts the supervisor and launches the Sirius Catapult engine.
   - Automatically opens your web browser to the Cockpit at **http://localhost:8080**.
2. **Stop Node**: Double-click `stop.bat` (or run `.\stop.bat`).
   - Gracefully stops the supervisor, block disruptors, and engine, cleanly flushing RocksDB state to disk.

---

## Web Cockpit Dashboard

Once started, access **http://localhost:8080** in any modern web browser (Chrome, Edge, Firefox, Brave):

- **Overview & Status**: Live block height, sync status, network hash rate, peer connections, and harvester status.
- **Harvesting Configuration**: Configure POS+ harvesting keys with password-masked inputs and automated balance checks.
- **Fast-Sync Snapshot**: One-click streaming snapshot restore to quickly synchronize the entire blockchain history.
- **Live Logs**: Streaming real-time logs from both the Go supervisor and C++ Catapult engine with level filtering.
- **Node Controls**: In-browser Start, Stop, and Restart controls.

---

## Directory Layout

```
proximax-sirius-core-windows/
├── sirius-core.exe          # Native Windows supervisor binary
├── start.bat                # Start script (starts node & opens web browser)
├── stop.bat                 # Stop script (gracefully stops node)
├── README.md                # This documentation
├── bin/                     # Native engine binaries and DLL dependencies
├── chainconfig/             # Configuration properties and blockchain data
│   ├── resources/           # Network and harvesting configuration properties
│   ├── data/                # Blockchain database (RocksDB + flat files)
│   └── logs/                # Node and engine log files
└── scripts/
    └── windows/             # Helper and advanced management scripts
```

---

## Helper Scripts Reference (`scripts/windows/`)

For advanced operations, management, or manual terminal usage, the following scripts are located in `scripts/windows/`:

| Script / Document | Description | Usage |
| :--- | :--- | :--- |
| **`start.bat`** / **`start-node.ps1`** | Background launcher that applies NTFS permission hardening (0600 equivalent) on key files and starts the node. | `.\scripts\windows\start.bat` |
| **`stop.bat`** / **`stop-node.ps1`** | Graceful shutdown script that communicates with the supervisor API to cleanly flush database state. | `.\scripts\windows\stop.bat` |
| **`restart.bat`** / **`restart-node.ps1`** | Convenience helper that executes a clean stop followed by a start. | `.\scripts\windows\restart.bat` |
| **`setup-wsl.bat`** / **`setup-wsl.ps1`** | Automated WSL2 environment verification. Checks that WSL2 Ubuntu is installed and verifies `libatomic1` runtime requirements. | `.\scripts\windows\setup-wsl.bat` |
| **`pick-directory.ps1`** | Interactive GUI folder picker dialog used when selecting a custom external SSD data path. | `powershell -File .\scripts\windows\pick-directory.ps1` |
| **`run.bat`** | Developer build and run script. Compiles the Go backend and React UI when working from the source git repository. | `.\scripts\windows\run.bat` |
| **`WINDOWS_DEFENDER_NOTES.md`** | Optimization guide for configuring Windows Defender Antivirus exclusions to prevent I/O latency on RocksDB database folders. | Read markdown guide |
