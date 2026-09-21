# Research & Architecture Decisions: Headless Sirius Catapult Add-on for Home Assistant

## 1. Architecture: Pure Headless Catapult Engine vs Web Supervisor

- **Decision**: Deploy a **pure headless Home Assistant Add-on running `sirius.bc` directly**.
- **Rationale**:
  - **Zero Bloat**: Eliminates the Go supervisor HTTP daemon and embedded React frontend, saving ~300 MB RAM and background polling CPU cycles on the 4GB RPi4.
  - **Native HA Configuration UI**: Home Assistant natively renders schema fields (`boot_key`, `harvest_key`, `friendly_name`, `fast_sync`) with password masking and validation in the Add-on Configuration tab.
  - **Native Lifecycle**: Home Assistant's built-in watchdog, start/stop toggles, and streaming log viewer manage the node effortlessly.
- **Alternatives Considered**:
  - *Full Supervisor + Web Cockpit*: Unnecessary overhead when Home Assistant already provides UI controls, log viewing, and configuration management.

---

## 2. Dynamic Option Resolution via `bashio`

- **Decision**: Use Home Assistant's official `bashio` library in `run.sh` to read configuration options.
- **Rationale**:
  - `bashio::config 'boot_key'` securely reads user inputs without exposing them in shell history.
  - `bashio::log.info` formats output cleanly with Home Assistant log timestamps and colors.
  - Automatic fallback to standard `jq` or default values if run in standalone testing environments.

---

## 3. Storage Persistence & Auto Snapshot Sync on SSD

- **Decision**:
  - Root persistent directory: `/data/chainconfig/` (mapped to `/dev/sda8` 250GB SSD on HAOS).
  - Snapshot streaming command:
    ```bash
    curl -f -sSL "$SNAPSHOT_URL" | tar --zstd -x -C /data/chainconfig/data/
    ```
- **Rationale**: Direct streaming pipe from Hugging Face eliminates intermediate tar file disk consumption and completes in under 5 minutes on the RPi4 SSD.

---

## 4. Resource & Memory Limits for RPi 4

- **Decision**:
  - Limit RocksDB cache in `config-node.properties` (`maxCacheSize = 536870912` - 512 MB).
  - Log rotation: `rotationSize = 25MB`, `maxTotalSize = 250MB`.
  - Catapult disruptor queue sizes tuned for 4-core Cortex-A72 CPU.
