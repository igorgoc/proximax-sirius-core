---
name: "sirius-node-sentry"
description: "Autonomous SRE and log detective for ProximaX Sirius validator node health, Home Assistant add-on monitoring, real-time log anomaly detection, memory profiling, and automated watchdog diagnostics."
---

# Sirius Node Sentry Runbook

This skill equips the agent to monitor, diagnose, and maintain the ProximaX Sirius blockchain validator node across Home Assistant add-on installations and native hosts.

## Core Capabilities

1. **Log Anomaly & Sync Velocity Diagnostics**:
   - Inspects node logs (`catapult_server.log`, Home Assistant container stdout).
   - Identifies stalled synchronization, consensus stalling, or peer disconnects on port 7900.
   - Diagnoses transaction expiration notices (`Failure_Chain_Transaction_Expired`).

2. **Memory & Resource Profiling**:
   - Analyzes RAM drops and climbs on constrained hardware (e.g. Raspberry Pi 4).
   - Correlates memory usage with RocksDB memtable flushing, block cache eviction, and state compaction cycles.

3. **Autonomous Local LLM Delegation**:
   - For logs > 100 lines, proactively runs `ask_remote` via LAN model (`http://192.168.1.111:8080`) to summarize anomalies and protect Antigravity token context.
   - Follows the Session Forget Rule if the endpoint is unavailable.

4. **Watchdog & Recovery Invariants**:
   - Enforces key privacy (never prints raw private keys; only public keys and transaction hashes).
   - Relies on `catapult.recovery` for WAL reconciliation; strictly forbids manual state byte surgery.
