# ProximaX Sirius Delegated Staking Architecture & Operational Guide

## 1. Architectural Overview

ProximaX Sirius Delegated Staking enables delegators to pool or delegate their PoS+ harvesting power to validator nodes without transferring custody of their funds or exposing their main account private keys.

The architecture comprises three coordinated layers:

```
┌────────────────────────────────────────────────────────┐
│           1. Web Wallet (Client Tier)                  │
│  - Ephemeral Remote Harvester Key Generation           │
│  - On-Chain AccountLink (Link / Unlink)                │
│  - Harvester Committee Registration                   │
│  - Diffie-Hellman (ECDH) Encrypted Transfer to Node    │
└───────────────────────────┬────────────────────────────┘
                            │ On-Chain P2P Broadcast
                            ▼
┌────────────────────────────────────────────────────────┐
│     2. Node Supervisor & Listener (Go Daemon)          │
│  - Background Outbound Polling (/transactions/confirmed)│
│  - ECDH Decryption using Node's Private Harvest Key    │
│  - On-Chain Link & XPX Mosaic Balance (>=100k) Verify   │
│  - File Persistence: resources/delegated_keys/*.key    │
│  - Periodic Stale Key Sweeper (Pruning broken links)   │
└───────────────────────────┬────────────────────────────┘
                            │ Local Filesystem Watcher
                            ▼
┌────────────────────────────────────────────────────────┐
│        3. Catapult Blockchain Engine (C++ Daemon)       │
│  - Dynamic Multi-Key Directory Watcher                 │
│  - UnlockedAccounts In-Memory Key Management           │
│  - 1-Second PoS+ Block Harvesting Evaluation Task      │
│  - Automatic In-Memory Pruning for Removed Files       │
└────────────────────────────────────────────────────────┘
```

---

## 2. Security & Cryptographic Invariants

### Zero Custody Risk
* The **Remote Harvester Key** can **ONLY** sign consensus block headers.
* It is permanently blocked by Catapult's `RemoteSenderValidator` from signing token transfers, multisig modifications, or balance deductions.
* Even if a remote harvester key is intercepted or exposed, the delegator's underlying funds cannot be stolen.

### End-to-End Encryption (ECDH)
* When transmitting the remote private key to the node, the wallet encrypts the payload using:
  $$\text{SharedSecret} = \text{ECDH}(\text{DelegatorPrivateKey}, \text{NodePublicKey})$$
* On the blockchain explorer, only ciphertext bytes are visible. Only the delegator and the targeted node operator possess the private keys required to decrypt it.

### XPX Mosaic Validation
* The supervisor strictly validates that the account holds at least **100,000 XPX** (`0x402B2F579FAEBC59`) before creating a key file. Non-XPX spam tokens are rejected.

---

## 3. Dynamic Key Lifecycle

### Ingestion (Link)
1. Delegator broadcasts an on-chain transfer to the node address containing the encrypted payload:
   ```json
   {
     "type": "sirius.delegated_staking",
     "version": 1,
     "action": "link",
     "remotePrivateKey": "<64_HEX_CHARACTERS>"
   }
   ```
2. The Go `DelegationListener` detects the transaction, decrypts the payload, verifies the on-chain link and balance, and writes:
   `chainconfig/resources/delegated_keys/<owner_address>.key` (mode `0600`).
3. `sirius.bc` automatically hot-loads the new keypair into `UnlockedAccounts` within 30 seconds without restarting the node.

### Deactivation (Unlink)
1. **Wallet-Initiated**: Delegator unlinks on-chain or broadcasts an encrypted transfer with `"action": "unlink"`.
2. **Node Operator Removal**: Node operator clicks the trash icon in Cockpit, deleting the `.key` file.
3. **Automated Pruning**:
   * The C++ engine detects file deletion and evicts the key from memory.
   * The Go supervisor's periodic sweeper checks active `.key` files every 5 minutes and deletes any files whose on-chain account link was broken or balance fell below 100,000 XPX.

---

## 4. Configuration Parameters

In `chainconfig/resources/config-harvesting.properties`:

```properties
[harvesting]
# Node's primary POS+ harvesting key
harvestKey = <64_HEX_PRIVATE_KEY>

# Beneficiary public key (optional; 000...000 for 100% to delegator, or operator key for 10% fee)
beneficiary = 0000000000000000000000000000000000000000000000000000000000000000

# Automatically harvest blocks
isAutoHarvestingEnabled = true

# Maximum simultaneous delegated staking slots (default: 5; can be raised to 100, 1000, etc.)
maxUnlockedAccounts = 5

# Directory containing delegated harvesting key files
delegatedKeysDirectory = ../resources/delegated_keys
```

---

## 5. Home Assistant Add-On Integration

Inside the Home Assistant container:
1. `sirius-core` runs as a supervisor process, listening on port 8080 (accessible via HA Ingress).
2. The `DelegationListener` uses outbound HTTPS requests, eliminating the need to forward ports on home routers.
3. Persistent keys are preserved in Home Assistant's `/data/chainconfig/resources/delegated_keys/` directory across container restarts and system reboots.
