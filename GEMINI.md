---
name: proximax-sirius-mainnet-peer
description: Architectural guidelines, privacy invariants, and storage rules for the ProximaX Sirius Mainnet Peer Node manager.
always_on: true
---

# ProximaX Sirius Mainnet Peer Node Guidelines

## 1. Key Privacy
- Never show raw private keys in UI result cards or overview panels.
- Display only public keys, addresses, and transaction hashes.
- Use password masking with reveal/hide toggle for all sensitive key inputs.
- Ensure private keys are only stored securely in `chainconfig/resources/config-harvesting.properties` and `config-user.properties`.

## 2. Fast-Sync & Storage Optimization
- Use direct streaming decompression from `https://huggingface.co/datasets/igorgoc/sirius-snapshot/resolve/main/sirius-data-backup-2026-09-10-131735.tar.zst` without intermediate tar file saving to eliminate disk overhead.
- Enforce Docker log rotation (`--log-opt max-size=250m --log-opt max-file=3`) and Sirius log caps (`rotationSize=25MB`, `maxTotalSize=250MB`).
- Maintain `.dockerignore` ignoring `chainconfig/data/` and `chainconfig/logs/` so Docker builds remain sub-10 seconds.
- Automatically clear `data/server.lock` on restart/recovery.

## 3. Configuration & External Storage
- Enforce strict separation between `bootKey` (P2P transport identity) and `harvestKey` (POS+ block harvesting identity). Never mirror harvest key to boot key.
- Support `data.path` in `config-user.properties` (default: `./chainconfig/data`) for storing blockchain data on external drives/SSDs.
- Mount `/Volumes:/Volumes` in `docker-compose.yml` so host-mounted SSD drives are accessible for directory browsing and container mounts.

## 4. Architecture & Frontend Standards
- Fully containerized in Docker to ensure zero pollution of the host OS.
- Always use `docker compose up -d --build` in `run.sh` so container images stay synchronized with code edits.
- Isolate background polling state (`/api/status` interval) from user form state so uncommitted form inputs are never overwritten.
- Go backend + embedded React/TypeScript UI served on port 3080.
- Connects to Sirius P2P on port 7900 and public REST APIs on port 3000.
