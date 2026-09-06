# Standalone Native Sirius Node CPU Usage Investigation & Debugging Context

## 1. Problem Statement
The standalone native build of the ProximaX Sirius Core node engine (`sirius.bc`) running on macOS (Apple Silicon ARM64, Darwin kernel) exhibits persistent high CPU usage (~200%–500% reported across multiple cores in Activity Monitor / `ps aux`), even when the node is idle and fully synchronized with the network.

---

## 2. Investigated Files, Components & Functions

### C++ Engine & Configuration (`catapult` / `cpp-xpx-chain`)
- `chainconfig/resources/config-node.properties`: `shouldUseSingleThreadPool`, `blockDisruptorSize`, `transactionDisruptorSize`, connection parameters.
- `chainconfig/resources/config-dbrb.properties`: `isDbrbProcess`, `resendMessagesInterval`.
- `chainconfig/resources/config-extensions-server.properties`: Consensus extensions (`extension.harvesting`, `extension.fastfinality`, `extension.partialtransaction`, `extension.sync`, `extension.timesync`).
- `chainconfig/resources/config-task.properties`: Background task schedules (`synchronizer task`, `time synchronization task`, `connect peers task for service Pt/Sync`, `pull partial transactions task`).
- `chainconfig/resources/peers-p2p.json` & `peers-api.json`: P2P bootstrap peer lists and API endpoint definitions.
- `src/catapult/disruptor/ConsumerDispatcher.cpp` & `libextension.sync.dylib`: Catapult LMAX Disruptor dispatcher consumer loops.
- `src/catapult/thread/IoThreadPool.cpp` & `libextension.fastfinality.dylib`: Boost ASIO `io_context` worker threads and mutex contention.

### Go Backend Supervisor & Frontend (`backend/`, `electron/`)
- `backend/pkg/supervisor/supervisor.go`: `ProcessSupervisor` process manager, socket peer inspection (`GetConnectedPeersCount`), property auto-synchronization (`syncProperties`).
- `backend/pkg/config/config.go`: `ConfigManager`, `SaveNodeConfig`, `LoadNodeConfig`.
- `backend/pkg/storage/storage.go`: Storage metrics calculations (`GetStorageMetrics`, `CheckOnChainAccount`).
- `backend/pkg/network/upnp.go`: UPnP reachability and public IP polling.
- `build-electron.sh`: Desktop bundling, staging sanitization, and release packaging.

---

## 3. Hypotheses Tested & Diagnostic Results

| Hypothesis | Test Performed | Result / Finding | Conclusion |
| :--- | :--- | :--- | :--- |
| **A. DBRB Committee Spin-Loop** | Set `isDbrbProcess = false` and `resendMessagesInterval = 5s`. | Reduced unnecessary `dbrb::MessageSender` network retries on non-committee peer nodes. | **Keep `isDbrbProcess = false`** on standard peer nodes. |
| **B. Disruptor Dedicated Dispatcher Spin-Wait** | Set `shouldUseSingleThreadPool = false` in `config-node.properties`. | Spawns 11 dedicated disruptor stage workers (7 block + 4 transaction) that execute `sleep_for(1ns)` / `nanosleep(0)` in `ConsumerDispatcher::run()`. On macOS, this causes thousands of Mach context switches (`mach_msg2_trap`, `__semwait_signal`), pegging cores at 200%–400%. | Cannot be fixed by property flags alone without C++ disruptor wait strategy modification. |
| **C. Mutual Exclusivity of Consensus Extensions** | Set `extension.fastfinality = false` and `extension.harvesting = true`. | Node successfully unlocked harvester account, but **stopped syncing new blocks** past height 13810473 because Sirius Mainnet blocks require FastFinality committee verification. | **Both `extension.fastfinality = true` and `extension.harvesting = true` are mandatory** on Sirius Mainnet. |
| **D. Missing Partial Transaction Handlers** | Removed `Pt` tasks while `extension.partialtransaction = false`. | Remote API peers (e.g. `aldebaran`) sent `Pull_Partial_Transaction_Infos` packets, causing `Malformed_Data` socket resets and peer drops. | `extension.partialtransaction = true` and its corresponding task definitions must remain enabled to prevent socket thrashing. |
| **E. Go Backend Inspection Overhead** | Replaced `lsof` with `netstat`, cached storage walks (15s), cached account lookups (30s), cached UPnP public IP (5m). | Dropped Go backend CPU from ~5% to <0.5%. | **Successfully resolved supervisor overhead.** |

---

## 4. Diagnostic Call Stacks & Output Samples

### Disruptor Worker Thread Busy-Spin Stack (`sample` output on `sirius.bc`)
```
Thread_144395: 0 block dispatcher
+ 1617 thread_start (in libsystem_pthread.dylib)
+   1617 _pthread_start (in libsystem_pthread.dylib)
+     1617 boost::(anonymous namespace)::thread_proxy(void*) (in libboost_thread.dylib)
+       1617 boost::detail::thread_data<catapult::disruptor::ConsumerDispatcher::...>::run() (in libextension.sync.dylib)
+         1617 std::this_thread::sleep_for(std::chrono::nanoseconds(1)) (in libc++.1.dylib)
+           1565 nanosleep (in libsystem_c.dylib)
+           ! 1565 __semwait_signal (in libsystem_kernel.dylib)
+           52 nanosleep (in libsystem_c.dylib)
+             52 clock_get_time (in libsystem_kernel.dylib)
+               52 mach_msg2_internal (in libsystem_kernel.dylib)
+                 52 mach_msg2_trap (in libsystem_kernel.dylib)
```

### Mutex Lock Contention in Boost ASIO Scheduler
```
Thread_115751: 0 server IoThreadPool worker
+ 1205 boost::asio::detail::scheduler::run(boost::system::error_code&)
+   ! 704 boost::asio::detail::scheduler::do_run_one(...)
+   ! : 694 boost::asio::detail::scheduler::work_cleanup::~work_cleanup()
+   ! : | 686 _pthread_mutex_firstfit_lock_slow
+   ! : | + 684 _pthread_mutex_firstfit_lock_wait
+   ! : | + ! 684 __psynch_mutexwait (in libsystem_kernel.dylib)
```

---

## 5. Next Steps & Recommended Code Fix

To bring `sirius.bc` CPU down from ~200% to <2% permanently on macOS **without breaking chain synchronization or block harvesting**:

1. **Patch C++ Disruptor Wait Strategy in `cpp-xpx-chain`**:
   - Location: `src/catapult/disruptor/ConsumerDispatcher.cpp` (or corresponding disruptor consumer loop).
   - Change: Replace the `std::this_thread::sleep_for(std::chrono::nanoseconds(1))` busy-wait poll with a condition variable wait / `std::this_thread::sleep_for(std::chrono::milliseconds(1))` when the disruptor queue has no incoming items.
2. **Recompile Native Sirius Core Binary**:
   - Build target: `bin/sirius.bc` and `bin/libextension.sync.dylib` for `darwin-arm64`.
3. **Keep Standard Sirius Mainnet Properties**:
    - `extension.harvesting = true`
    - `extension.fastfinality = true`
    - `extension.partialtransaction = true`
    - `isDbrbProcess = false` (for peer nodes)


