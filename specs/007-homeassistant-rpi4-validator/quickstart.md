# Quickstart Validation Guide: Headless Sirius Catapult Add-on

## Prerequisites
- Raspberry Pi 4 running Home Assistant OS with 250GB SSD.
- SSH access to Home Assistant (`ssh igorgoc@192.168.1.14`).
- Router port forward for Port `7900` (TCP) to `192.168.1.14`.

---

## Validation Scenario 1: Deploying Add-on to Home Assistant
1. Copy the `addon/` directory to `/addons/proximax-sirius` on your Home Assistant host:
   ```bash
   scp -r addon/* igorgoc@192.168.1.14:/addons/proximax-sirius/
   ```
2. In Home Assistant: **Settings** → **Add-ons** → **Add-on Store** → Top right **⋮** → **Check for updates**.
3. Under **Local add-ons**, click **ProximaX Sirius Validator** → **Install**.

---

## Validation Scenario 2: Configuring Keys & Fast-Sync
1. Navigate to the **Configuration** tab of the add-on.
2. Enter your:
   - **Boot Key**: 64-character hex private key for P2P identity.
   - **Harvest Key**: 64-character hex private key for POS+ block harvesting.
   - **Friendly Name**: e.g., `My-HomeAssistant-Node`.
   - **Fast Sync**: `true` (ON).
3. Click **Save**.

---

## Validation Scenario 3: Starting Validator & Monitoring Logs
1. Click the **Info** tab → toggle **Start on boot** and **Watchdog** to ON → Click **Start**.
2. Switch to the **Log** tab in Home Assistant.
3. **Expected Outcome**:
   - `run.sh` securely creates `/data/chainconfig/resources/`.
   - Streams and extracts fast-sync snapshot onto the SSD in ~3-5 minutes.
   - Starts `sirius.bc` engine.
   - Logs show peer connection on port `7900`, block height syncing, and active harvesting.
