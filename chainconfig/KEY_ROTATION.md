# ProximaX Sirius Core — Cryptographic Release Key Management & Rotation Procedure

## 1. Architecture & Dual-Keypair Threat Model
To maintain strict separation of concerns and prevent cross-repository credential compromise, the system maintains two completely separate cryptographic keypairs:

### A. Sirius C++ Engine Keypair (`cpp-xpx-chain`)
- **Private Key (`ENGINE_RELEASE_PRIVATE_KEY`)**: Stored in GitHub Actions secrets for `igorgoc/cpp-xpx-chain`.
- **Public Key**: Published in [`chainconfig/engine.compat.json`](engine.compat.json) (`releasePublicKeyHex`).
- **Function**: Verified by the node manager's autonomous engine updater before replacing native engine binaries (`sirius-core`, `libcatapult.*`).

### B. Sirius Cockpit & Node Manager Keypair (`proximax-sirius-core`)
- **Private Key (`NODE_MANAGER_RELEASE_PRIVATE_KEY`)**: Stored in GitHub Actions secrets for `igorgoc/proximax-sirius-core`.
- **Public Key**: Published in [`chainconfig/manager.compat.json`](manager.compat.json) (`releasePublicKeyHex`).
- **Function**: Used by GitHub Actions to sign cross-platform distributions (Debian `.deb`, Linux tarballs, macOS bundles, Windows packages) and verified during CI release publication.

### Fail-Closed Invariant:
- The GitHub Actions release workflow refuses to publish any release if `NODE_MANAGER_RELEASE_PRIVATE_KEY` is missing.
- The node updater refuses to apply any update if the cryptographic signature fails or the checksum does not match.

---

## 2. Generating a Fresh Production Keypair
To generate a new keypair on an air-gapped machine or secure developer workstation:

```bash
# Build the signing utility
cd backend && go build -o ../bin/sign-release ./cmd/sign-release

# Generate a new Ed25519 release keypair
../bin/sign-release keygen -out-dir ~/.sirius-keys
```

Output:
- `~/.sirius-keys/release-ed25519.key`: Private key file (chmod `0600`).
- `~/.sirius-keys/release-ed25519.pub`: Public key hex string.

### Installing into GitHub Secrets:
1. Navigate to **GitHub Repository** -> **Settings** -> **Secrets and variables** -> **Actions**.
2. Create/Update Secret:
   - For Node Manager repo (`proximax-sirius-core`): **`NODE_MANAGER_RELEASE_PRIVATE_KEY`**
   - For Engine repo (`cpp-xpx-chain`): **`ENGINE_RELEASE_PRIVATE_KEY`**
   - **Value**: The raw 64-byte hex string from `release-ed25519.key`.
3. Securely wipe the local unencrypted private key file or store it in an encrypted hardware vault (e.g. YubiKey / HSM):
   ```bash
   shred -u ~/.sirius-keys/release-ed25519.key
   ```

---

## 3. Key Rotation Procedure

If the release private key is ever suspected compromised or scheduled for routine rotation:

### Step 1: Generate New Keypair
Generate `NEW_PRIVATE_KEY_HEX` and `NEW_PUBLIC_KEY_HEX` as shown in Section 2.

### Step 2: Update Repository Public Key
Update [`chainconfig/engine.compat.json`](engine.compat.json):
```json
{
  "engineRepository": "igorgoc/cpp-xpx-chain",
  "engineMinCompatible": "v1.9.0",
  "engineMaxCompatible": "v1.9.99",
  "recommendedVersion": "v1.9.8",
  "releasePublicKeyHex": "<NEW_PUBLIC_KEY_HEX>"
}
```

### Step 3: Migration Paths for Existing Installs

Existing installs verify updates using the `releasePublicKeyHex` stored in their local `chainconfig/engine.compat.json`.

#### Scenario A: Scheduled / Graceful Rotation (Recommended)
1. Prepare a bridge update (e.g. `v1.9.8`).
2. Sign the `v1.9.8` release with the **OLD** private key.
3. The `v1.9.8` package contains the updated `chainconfig/engine.compat.json` pointing to the **NEW** public key.
4. Once all nodes update to `v1.9.8`, update the GitHub Secret `ENGINE_RELEASE_PRIVATE_KEY` to the **NEW** private key.
5. All subsequent releases (`v1.9.9+`) are signed with the new key.

#### Scenario B: Emergency Compromise Revocation
If the old private key is actively compromised:
1. Immediately delete or overwrite `ENGINE_RELEASE_PRIVATE_KEY` in GitHub Repository Secrets to stop automated release signing.
2. Commit the new `releasePublicKeyHex` into `main`.
3. Publish an emergency security advisory and notify node operators to pull the updated `engine.compat.json` via git or apply the patch package:
   ```bash
   git pull origin main
   ./restart.sh
   ```
4. Because the node supervisor fails closed, an attacker possessing the compromised key *cannot* deploy malicious binaries to nodes once the public key in `engine.compat.json` has been updated.
