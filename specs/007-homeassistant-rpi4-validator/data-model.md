# Data Model & Storage Schema: Headless Sirius Catapult Add-on

## 1. Storage Layout in HAOS SSD (`/data`)

```
/data/chainconfig/
├── resources/
│   ├── config-user.properties        # [Generated] bootKey, dataDirectory, friendlyName
│   ├── config-harvesting.properties  # [Generated] harvestKey (chmod 0600)
│   ├── config-node.properties        # RocksDB cache size & network settings
│   ├── config-network.properties     # Sirius Mainnet network identifier & generation hash
│   ├── config-database.properties    # RocksDB column family configurations
│   ├── config-logging-file.properties# Log rotation (25MB/250MB limit)
│   └── config-task.properties        # Disruptor queue & sync task timers
├── data/
│   ├── 00000/                        # Flat block files (00001.dat, hashes.dat)
│   ├── index.dat                     # Height tracking flat file (8-byte binary)
│   └── statedb/                      # RocksDB column families (accounts, state tree)
└── logs/
    └── sirius.log                    # C++ Catapult engine log (streamed to stdout)
```

## 2. Configuration Schema Mapping

| Home Assistant Option (`config.yaml`) | Target Configuration File | Property Name |
| :--- | :--- | :--- |
| `boot_key` | `chainconfig/resources/config-user.properties` | `bootKey` |
| `harvest_key` | `chainconfig/resources/config-harvesting.properties` | `harvestKey` |
| `friendly_name` | `chainconfig/resources/config-user.properties` | `friendlyName` |
| `fast_sync` | *Evaluated at startup in `run.sh`* | Trigger streaming restore |
| `custom_snapshot_url` | *Evaluated at startup in `run.sh`* | Optional custom archive URL |
