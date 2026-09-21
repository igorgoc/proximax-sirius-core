# ProximaX Sirius Mainnet Validator for Home Assistant

Run a dedicated, low-power **ProximaX Sirius Mainnet POS+ Validator Node** directly on your Raspberry Pi 4 inside Home Assistant.

---

## Prerequisites

1. **Storage**: At least 35 GB free storage (recommended on SSD attached to your Raspberry Pi 4).
2. **Router Port Forwarding**: Forward external port **`7900` (TCP)** on your home router to your Home Assistant IP (`192.168.1.14`) to allow other Sirius mainnet nodes to discover and connect to your validator.

---

## Configuration

Navigate to the **Configuration** tab in Home Assistant before starting the add-on:

| Option | Required | Description |
| :--- | :--- | :--- |
| **`boot_key`** | Yes | 64-character hexadecimal private key for P2P transport network identity. |
| **`harvest_key`** | Yes | 64-character hexadecimal private key for POS+ block harvesting and earning block rewards. |
| **`friendly_name`** | No | Visible name for your node on the network (default: `HomeAssistant-Validator`). |
| **`fast_sync`** | No | Automatically streams and decompresses the official snapshot on first launch (default: `true`). |
| **`custom_snapshot_url`**| No | Optional custom zstandard snapshot tarball URL if not using official Hugging Face snapshot. |

---

## How it Works

1. On container startup, `run.sh` initializes `/data/chainconfig/` on your SSD.
2. If the blockchain data directory is empty and `fast_sync` is enabled, the node streams the official verified snapshot directly from Hugging Face into `/data/chainconfig/data/` without filling up temporary disk space.
3. The native C++ Catapult engine (`sirius.bc`) boots up, connects to seed peers on port `7900`, syncs latest blocks, and begins POS+ harvesting.
4. All real-time logs stream directly into the Home Assistant **Log** tab.
