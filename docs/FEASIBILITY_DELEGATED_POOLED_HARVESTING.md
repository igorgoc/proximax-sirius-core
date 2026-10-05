# Feasibility Investigation Report: Delegated & Pooled Harvesting on ProximaX Sirius (`cpp-xpx-chain`)

---

## 1. Verification of Prior Beliefs (CONFIRMED & REVISED)

### Belief 1: `AccountLinkTransaction` Links Remote Harvesting Key to Main Account
* **Status**: **CONFIRMED**
* **Citations**:
  * [`plugins/txes/account_link/src/model/AccountLinkTransaction.h`](file:///Users/igorgoc/Projects/cpp-xpx-chain/plugins/txes/account_link/src/model/AccountLinkTransaction.h#L31-L54): Declares `AccountLinkTransactionBody` with `Key RemoteAccountKey` (line 41) and `AccountLinkAction LinkAction` (line 44).
  * [`plugins/txes/account_link/src/model/AccountLinkAction.h`](file:///Users/igorgoc/Projects/cpp-xpx-chain/plugins/txes/account_link/src/model/AccountLinkAction.h#L27-L33): Defines `enum class AccountLinkAction : uint8_t { Link, Unlink }`.
  * [`plugins/txes/account_link/src/observers/AccountLinkObserver.cpp`](file:///Users/igorgoc/Projects/cpp-xpx-chain/plugins/txes/account_link/src/observers/AccountLinkObserver.cpp#L34-L50): When linking, establishes a bidirectional link in `AccountStateCache`:
    * `mainAccountState.LinkedAccountKey = notification.RemoteAccountKey; mainAccountState.AccountType = AccountType::Main;`
    * `remoteAccountState.LinkedAccountKey = notification.MainAccountKey; remoteAccountState.AccountType = AccountType::Remote;`
* **Additional Note**: ProximaX Sirius requires a **second on-chain transaction** beyond `AccountLink`: the `AddHarvesterTransaction` ([`plugins/txes/committee/src/observers/AddHarvesterObserver.cpp#L15-L26`](file:///Users/igorgoc/Projects/cpp-xpx-chain/plugins/txes/committee/src/observers/AddHarvesterObserver.cpp#L15-L26)), which registers the key into the consensus `CommitteeCache`.

---

### Belief 2: Remote/Linked Key Can ONLY Sign Blocks, Never Spend Funds
* **Status**: **CONFIRMED (Strict Consensus Invariant)**
* **Citations across the entire source tree**:
  * **Cannot Sign Transactions**: [`plugins/txes/account_link/src/validators/RemoteSenderValidator.cpp`](file:///Users/igorgoc/Projects/cpp-xpx-chain/plugins/txes/account_link/src/validators/RemoteSenderValidator.cpp#L29-L35):
    ```cpp
    DEFINE_STATEFUL_VALIDATOR(RemoteSender, [](const auto& notification, const auto& context) {
        ...
        return accountStateIter.tryGet() && state::IsRemote(accountStateIter.get().AccountType)
            ? Failure_AccountLink_Remote_Account_Signer_Not_Allowed
            : ValidationResult::Success;
    });
    ```
    Any transaction signed by a remote key is rejected by consensus.
  * **Cannot Participate in Any Transaction**: [`plugins/txes/account_link/src/validators/RemoteInteractionValidator.cpp`](file:///Users/igorgoc/Projects/cpp-xpx-chain/plugins/txes/account_link/src/validators/RemoteInteractionValidator.cpp#L48-L61): Rejects any transaction where a remote key is a sender, recipient, or cosignatory (`Failure_AccountLink_Remote_Account_Participant_Not_Allowed`).
  * **Cannot Unlink Itself**: [`plugins/txes/account_link/src/validators/AccountLinkAvailabilityValidator.cpp`](file:///Users/igorgoc/Projects/cpp-xpx-chain/plugins/txes/account_link/src/validators/AccountLinkAvailabilityValidator.cpp#L38-L44): Explicitly enforces `if (state::AccountType::Main != accountState.AccountType) return Failure_AccountLink_Link_Does_Not_Exist;` — only the main account can unlink the key.
  * **Automatic Fee Redirection**: [`plugins/coresystem/src/observers/HarvestFeeObserver.cpp`](file:///Users/igorgoc/Projects/cpp-xpx-chain/plugins/coresystem/src/observers/HarvestFeeObserver.cpp#L54-L66): When a block signed by a remote key is processed, `ApplyFee` detects `AccountType::Remote`, loads `accountState.LinkedAccountKey`, and credits the main account (`linkedAccountState.Balances.credit(...)`). The remote key holds zero funds.
  * **Importance Resolution**: [`src/catapult/cache_core/ImportanceView.cpp`](file:///Users/igorgoc/Projects/cpp-xpx-chain/src/catapult/cache_core/ImportanceView.cpp#L34-L42): PoS+ importance checks resolve through `accountState.LinkedAccountKey` to the delegator's main balance.

---

### Belief 3: `UnlockedAccounts.h` Supports Hosting Multiple Key Pairs
* **Status**: **CONFIRMED (In Data Structure, but with a Vital Caveat)**
* **Citations**:
  * [`src/catapult/harvesting_core/UnlockedAccounts.h`](file:///Users/igorgoc/Projects/cpp-xpx-chain/src/catapult/harvesting_core/UnlockedAccounts.h#L118-L135): `UnlockedAccounts` encapsulates `std::vector<crypto::KeyPair> m_keyPairs` guarded by `utils::SpinReaderWriterLock`.
  * [`src/catapult/harvesting_core/UnlockedAccounts.cpp`](file:///Users/igorgoc/Projects/cpp-xpx-chain/src/catapult/harvesting_core/UnlockedAccounts.cpp#L53-L60):
    ```cpp
    if (m_maxUnlockedAccounts == m_keyPairs.size())
        return UnlockedAccountsAddResult::Failure_Server_Limit;
    m_keyPairs.push_back(std::move(keyPair));
    ```
* **Vital Caveat (Discovered in Code)**: While the container supports multiple keypairs, the C++ Catapult daemon [`HarvestingService.cpp#L34-L48`](file:///Users/igorgoc/Projects/cpp-xpx-chain/extensions/harvesting/src/HarvestingService.cpp#L34-L48) **only reads a single `config.HarvestKey`** from `config-harvesting.properties` on startup, and [`HarvestingConfiguration.cpp#L38-L46`](file:///Users/igorgoc/Projects/cpp-xpx-chain/src/catapult/harvesting_core/HarvestingConfiguration.cpp#L38-L46) enforces `VerifyBagSizeLte(bag, 4)`. There is currently **no runtime HTTP or P2P endpoint in the C++ engine to unlock additional keys** into `UnlockedAccounts` (see Point 1 & Point 4).

---

### Belief 4: `Harvester.cpp` Sets Block `Signer` to the Specific Delegator Key That Won
* **Status**: **CONFIRMED**
* **Citations**:
  * [`extensions/harvesting/src/Harvester.cpp`](file:///Users/igorgoc/Projects/cpp-xpx-chain/extensions/harvesting/src/Harvester.cpp#L104-L128):
    ```cpp
    for (const auto& keyPair : unlockedAccountsView) {
        hitContext.Signer = keyPair.publicKey();
        hitContext.GenerationHash = model::CalculateGenerationHash(context.ParentContext.GenerationHash, hitContext.Signer);
        if (hitPredicate(hitContext)) {
            pHarvesterKeyPair = &keyPair;
            break;
        }
    }
    ...
    auto pBlockHeader = CreateUnsignedBlockHeader(context, config.Immutable.NetworkIdentifier, pHarvesterKeyPair->publicKey(), m_beneficiary);
    ...
    SignBlockHeader(*pHarvesterKeyPair, *pBlock);
    ```
  * Blocks are 100% attributable on-chain per delegator by their remote public key in `block.signer`.

---

### Belief 5: `harvestBeneficiaryPercentage` Is a Fixed Network-Wide Consensus Parameter
* **Status**: **CONFIRMED (with Minor Clarification on Type)**
* **Citations**:
  * [`plugins/coresystem/src/observers/HarvestFeeObserver.cpp`](file:///Users/igorgoc/Projects/cpp-xpx-chain/plugins/coresystem/src/observers/HarvestFeeObserver.cpp#L80-L93): Reads `config.Network.HarvestBeneficiaryPercentage` (line 80) and calculates `beneficiaryAmount = Amount(totalAmount.unwrap() * percentage / 100)`.
  * [`resources/config-network.properties`](file:///Users/igorgoc/Projects/cpp-xpx-chain/resources/config-network.properties#L25): `harvestBeneficiaryPercentage = 10`.
  * [`src/catapult/model/NetworkConfiguration.h`](file:///Users/igorgoc/Projects/cpp-xpx-chain/src/catapult/model/NetworkConfiguration.h#L92): Declared as `uint8_t HarvestBeneficiaryPercentage;`.
* **Clarification**: In Catapult, `Beneficiary` is a 32-byte public key (`Key`), not an Address ([`Harvester.cpp#L63`](file:///Users/igorgoc/Projects/cpp-xpx-chain/extensions/harvesting/src/Harvester.cpp#L63), [`HarvestFeeObserver.cpp#L68`](file:///Users/igorgoc/Projects/cpp-xpx-chain/plugins/coresystem/src/observers/HarvestFeeObserver.cpp#L68)). `ApplyFee` resolves the account from the public key.

---

## 2. In-Depth Investigation

### 1. Committee / Round Selection Mechanics & Scaling
* **How Selection Works**:
  Every 1 second ([`config-task.properties#L27`](file:///Users/igorgoc/Projects/cpp-xpx-chain/resources/config-task.properties#L27)), `ScheduledHarvesterTask` executes `Harvester::harvest`. It iterates through `m_unlockedAccounts.view()` in vector order ([`Harvester.cpp#L106-L114`](file:///Users/igorgoc/Projects/cpp-xpx-chain/extensions/harvesting/src/Harvester.cpp#L106-L114)):
  1. Computes `hitContext.GenerationHash = SHA3_256(ParentGenerationHash || KeyPair.publicKey)`.
  2. Evaluates `hitPredicate`:
     * Computes `hit = CalculateHit(GenerationHash)`.
     * Looks up delegator's main account importance in `ImportanceView`.
     * Computes `target = CalculateTarget(ElapsedTime, Difficulty, Importance, ...)`.
     * Tests `hit < target` ([`BlockScorer.cpp#L100-L115`](file:///Users/igorgoc/Projects/cpp-xpx-chain/src/catapult/chain/BlockScorer.cpp#L100-L115)).
  3. **First-hit wins**: The loop executes `pHarvesterKeyPair = &keyPair; break;`.
* **Scalability (CPU / Memory)**:
  * **Memory**: Each hosted account is a 64-byte `crypto::KeyPair`. Hosting 1,000 delegators consumes **~64 KB of RAM** — virtually zero memory footprint.
  * **CPU**: For $N$ delegators, the node computes at most $N$ SHA3-256 hashes and memory-table importance lookups per second. For $N = 100$, this takes $< 0.1\text{ ms}$ of CPU time per second; for $N = 1,000$, it takes $\approx 1\text{ ms}$ per second. It scales linearly and cleanly.
* **Selection Bias / Tie-Breaking**:
  Because `target` grows with `ElapsedTime` (block delay), the delegator whose mathematical target threshold is crossed earliest in time wins the round. However, if two delegators on the same node cross the target in the *exact same 1-second tick*, the one positioned earlier in `m_keyPairs` will always be selected due to `break;`.
* **Hard Caps**:
  * `maxUnlockedAccounts` in `config-harvesting.properties` (default: 5) is defined as `uint32_t` in [`HarvestingConfiguration.h#L39`](file:///Users/igorgoc/Projects/cpp-xpx-chain/src/catapult/harvesting_core/HarvestingConfiguration.h#L39). It can be raised arbitrarily (e.g. to 10,000) without any code-level hard ceiling.

---

### 2. Is `harvestBeneficiaryPercentage` Truly Fixed? Levers & Limitations
* **Is there a per-node override?**: **NO.**
  `HarvestFeeObserver` strictly reads `context.Config.Network.HarvestBeneficiaryPercentage`. If a node operator altered this value locally, the receipts hash (`block.blockReceiptsHash`) and state hash would differ from the rest of the network, causing other peers to reject the block immediately with `Failure_Core_Block_Receipts_Hash_Mismatch`.
* **Can it be upgraded on-chain?**: **YES.**
  ProximaX Sirius includes the `BlockchainConfigPlugin` ([`plugins/txes/config/src/validators/NetworkConfigValidator.cpp#L49-L50`](file:///Users/igorgoc/Projects/cpp-xpx-chain/plugins/txes/config/src/validators/NetworkConfigValidator.cpp#L49-L50)), which allows network-wide dynamic parameter updates via an on-chain `NetworkConfigTransaction`. Changing it requires network governance multisig approval.
* **Existing Levers for Pool Fees**:
  * **Lever A (Zero Beneficiary)**: Setting `beneficiary = 000...000` (or leaving it blank) triggers `ShouldShareFees == false` ([`HarvestFeeObserver.cpp#L68-L70`](file:///Users/igorgoc/Projects/cpp-xpx-chain/plugins/coresystem/src/observers/HarvestFeeObserver.cpp#L68-L70)), giving **100% directly to the winning delegator's main account**.
  * **Lever B (Fixed 10% Protocol Cut)**: Setting `beneficiary = <node_operator_public_key>` automatically delivers **exactly 10% to the operator** and **90% to the winning delegator** directly at the consensus level.
  * **Crucial Implication for a "Pool"**: The consensus engine **CANNOT** enforce an arbitrary pool fee (e.g. 3% or 15%) on-chain, nor can it hold the 90% in escrow. To offer custom rates or pool-wide reward smoothing, the operator must rely on off-chain accounting and periodic redistribution transactions.

---

### 3. Security Model & Blast Radius of Hosted Remote Private Keys
* **Storage at Rest**:
  * In the C++ engine: stored in plaintext in `chainconfig/resources/config-harvesting.properties` (`harvestKey = <hex>`).
  * In the Go supervisor & Home Assistant add-on: stored in `config-harvesting.properties` with UNIX file permissions `0600`.
  * **There is currently NO encryption-at-rest or HSM / PKCS#11 support in Catapult C++ or the Go supervisor.** Keys are parsed via `crypto::KeyPair::FromString` from raw hex.
* **Blast Radius Analysis (If the Server Is Fully Compromised)**:
  * **Can an attacker steal funds from delegator accounts?** **NO.**
    Even with the remote private key in hand, the attacker cannot transfer tokens, sign transactions, or alter account states. All transactions signed by remote accounts are permanently blocked by consensus (`RemoteSenderValidator`).
  * **Can an attacker unlink or redirect the delegation?** **NO.**
    An `AccountLinkTransaction` with `LinkAction::Unlink` must be signed by the **Main Account private key** ([`AccountLinkAvailabilityValidator.cpp#L38-L44`](file:///Users/igorgoc/Projects/cpp-xpx-chain/plugins/txes/account_link/src/validators/AccountLinkAvailabilityValidator.cpp#L38-L44)). The remote key lacks the authority to unlink itself or link elsewhere.
  * **What CAN an attacker do?**
    1. **Harvest on a rogue node**: The attacker can copy the remote keys to their own server and harvest blocks. However, on-chain, **90% of all block rewards will still be deposited directly into the delegators' main accounts**! The attacker only steals the 10% beneficiary portion if they set their own beneficiary public key.
    2. **Equivocation / Double-Signing**: If the attacker and the legitimate node both harvest a block at the exact same height using the same key, it could cause temporary fork competition. However, current mainnet has `enableWeightedVoting = false` and `enableDbrbFastFinality = false` ([`resources/config-network.properties#L111`](file:///Users/igorgoc/Projects/cpp-xpx-chain/resources/config-network.properties#L111)), so slashing rules are inactive.

---

### 4. Existing Support in `proximax-sirius-core` (Go Backend + React Frontend)
An audit of `backend/pkg` in `proximax-sirius-core-native` reveals:

| Feature | Current State | File Reference |
| :--- | :--- | :--- |
| **AccountLink Construction & Signing** | **Fully Implemented** | [`backend/pkg/crypto/crypto.go#L111-L374`](file:///Users/igorgoc/Projects/proximax-sirius-core-native/backend/pkg/crypto/crypto.go#L111-L374) (`PerformDelegatedHarvestingLink`) |
| **AccountLink HTTP API** | **Fully Implemented** | [`backend/pkg/api/server.go#L808-L848`](file:///Users/igorgoc/Projects/proximax-sirius-core-native/backend/pkg/api/server.go#L808-L848) (`/api/harvesting/link`) |
| **Committee Harvester Registration** | **Implemented as Utility** | [`backend/bin/add-harvester-util`](file:///Users/igorgoc/Projects/proximax-sirius-core-native/backend/bin/add-harvester-util) (`go-xpx-harvester-util`) |
| **Multiple Unlocked Accounts Management** | **NET-NEW WORK** | Currently only persists one single `HarvestKey` in `config-harvesting.properties` ([`server.go#L837`](file:///Users/igorgoc/Projects/proximax-sirius-core-native/backend/pkg/api/server.go#L837), [`config.go#L377`](file:///Users/igorgoc/Projects/proximax-sirius-core-native/backend/pkg/config/config.go#L377)). |
| **Delegator Registry & State Tracking** | **NET-NEW WORK** | No database/store exists for tracking delegators, their main accounts, or active remote keys. |
| **Payout Accounting & Batch Distribution** | **NET-NEW WORK** | No accounting ledger or reward calculation engine exists. |

---

### 5. Historical Block Querying by `Signer` in MongoDB / REST
* **MongoDB Index Status**: **ALREADY INDEXED & OPTIMIZED**
  * In [`scripts/mongo/mongoDbPrepare.js#L26-L29`](file:///Users/igorgoc/Projects/cpp-xpx-chain/scripts/mongo/mongoDbPrepare.js#L26-L29):
    ```javascript
    db.blocks.createIndex({ 'block.signer': 1 });
    db.blocks.createIndex({ 'block.signer': 1, 'block.height': -1 }, { unique: true });
    ```
  * MongoDB has a compound index on `block.signer` and `block.height` descending. Querying historical blocks for a given delegator's remote public key is an indexed $O(\log M)$ B-tree lookup, offering excellent performance at multi-million block scales.
* **REST API Status**:
  * In Catapult REST (port 3000), `/blocks` endpoints and `/diagnostic/blocks` support querying by height.
  * In `sirius-rest` / `catapult-rest`, the `/blocks` collection can be queried directly via MongoDB or through the block search route using `signerPublicKey`. If querying via standard REST, the route `/blocks/from/{height}/limit/{limit}` queries the indexed MongoDB collection.

---

### 6. Codebase Family Precedents: The "Pool" Dilemma in NEM / Symbol / Sirius
* **The Fundamental Architectural Difference**:
  * In Bitcoin / Ethereum mining pools, block rewards are paid **to a single pool address controlled by the pool operator**, who then redistributes them to miners based on submitted shares (PPLNS / PPS).
  * In Catapult / Symbol / ProximaX Sirius:
    **Consensus bypasses the node operator.** 90% of the block reward (fees + inflation) is credited **directly into the delegator's main account on-chain** ([`HarvestFeeObserver.cpp#L65`](file:///Users/igorgoc/Projects/cpp-xpx-chain/plugins/coresystem/src/observers/HarvestFeeObserver.cpp#L65)).
* **How "Pools" Worked Historically in NEM / Symbol**:
  1. **"Delegated Harvesting Hosting Service" (Non-Custodial Solo Model)**:
     Most services branded as "harvesting pools" in NEM/Symbol were actually **multi-tenant node hosting services**:
     * Delegators linked their key to the node.
     * When a delegator won a block, they kept 100% of their 90% reward.
     * The node operator collected the 10% `beneficiary` network fee as their hosting commission.
     * *There was no variance smoothing between delegators.*
  2. **True Reward-Smoothing Pools (The "Free-Rider" Hazard)**:
     If an operator wants to pool and distribute rewards smoothly to all delegators proportional to their staked balance (regardless of who hit a block):
     * A winning delegator receives the 90% on-chain directly in their personal wallet.
     * The pool operator cannot claw this money back.
     * If the pool operator pays out smoothed rewards out-of-pocket, any delegator who wins a block can keep their 90% jackpot AND collect their smoothed pool share, then unlink and walk away.
     * The only historical ways to avoid this were:
       a) Require delegators to transfer their funds into a pool-managed account or multi-sig (custodial / semi-custodial), OR
       b) Operate strictly as a **non-custodial solo-harvesting hosting provider** where each delegator receives their own blocks and the pool collects the 10% beneficiary reward.

---

## 3. Riskiest Unknowns

1. **How to Feed Multiple Keys into `sirius.bc` without Daemon Modifications**:
   * `HarvestingConfiguration.cpp` only loads 1 key (`HarvestKey`). `utils::VerifyBagSizeLte(bag, 4)` will crash the node if extra keys are added to `config-harvesting.properties`.
   * *Risk*: Unless `cpp-xpx-chain` is patched (e.g. to load keys from a file or directory) OR a dynamic unlock extension is added, the C++ node cannot physically host multiple unlocked keys simultaneously on startup.
2. **On-Chain Committee Registration Requirement**:
   * ProximaX Sirius PoS+ requires each harvester key to be registered in `CommitteeCache` via `AddHarvesterTransaction`.
   * *Risk*: Can a third-party delegator announce `AddHarvesterTransaction` specifying a remote key generated by your service, or does the transaction fee / signature requirement create friction that aborts the onboarding funnel?
3. **The Delegator Reward Recovery Problem (True Pool vs. Hosting Service)**:
   * Because 90% of block rewards land directly in the delegator's main wallet, a "pooled" model with daily redistribution requires trusting delegators to remit excess earnings, or running a reserve that covers non-remitted jackpots.

---

## 4. Open Questions Before Scoping Work

1. **Service Model Definition**: Do you intend to build:
   * **Model A (Non-Custodial Solo-Harvesting Host)**: Delegators delegate to your node; when their key hits a block, they keep the 90% on-chain reward, and your node receives the 10% consensus beneficiary fee as your commission (zero custodial risk, zero payout accounting, zero default risk)?
   * **Model B (True Pooled / Smoothed Yield)**: All rewards generated across your node are pooled and divided pro-rata among all participants based on daily stake? (If Model B, how do you plan to handle delegators receiving 90% directly to their wallets?)
2. **C++ Daemon Modification Tolerance**:
   * Are you willing to recompile `sirius.bc` with a patched `HarvestingService` / `HarvestingConfiguration` (or custom extension) to load a list of keys, or must the solution work against vanilla upstream binaries?
3. **Delegator Key Generation & Custody**:
   * Will delegators generate the remote key in their own client and send the remote private key to your server over an API, or will your server generate the remote key pair and return the public key for the delegator to link?
