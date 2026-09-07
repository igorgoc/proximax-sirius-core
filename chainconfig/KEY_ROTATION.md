# ProximaX Sirius Core Engine — Cryptographic Release Key Management & Rotation Procedure

## 1. Architecture & Threat Model
The ProximaX Sirius Core node manager features an autonomous engine updater (`cpp-xpx-chain`). To prevent arbitrary remote code execution (RCE) and man-in-the-middle (MITM) package tampering, every released engine binary and platform distribution archive must be cryptographically signed with an Ed25519 release private key.

- **Private Key (`ENGINE_RELEASE_PRIVATE_KEY`)**: 64-byte Ed25519 signing key. Highly confidential. Stored exclusively in GitHub Actions Encrypted Secrets (`secrets.ENGINE_RELEASE_PRIVATE_KEY`).
- **Public Key (`releasePublicKeyHex`)**: 32-byte Ed25519 verification key published in [`chainconfig/engine.compat.json`](engine.compat.json). Distributed publicly with each node installation.
- **Fail-Closed Guarantee**:
  - The GitHub Actions release workflow refuses to publish any release if `ENGINE_RELEASE_PRIVATE_KEY` is missing or invalid.
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
   - **Name**: `ENGINE_RELEASE_PRIVATE_KEY`
   - **Value**: The raw hex string from `release-ed25519.key`.
3. Wipe the local unencrypted private key file or store it in an encrypted hardware vault (e.g. YubiKey / HSM):
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
