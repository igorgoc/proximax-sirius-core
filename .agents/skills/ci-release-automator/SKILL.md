---
name: "ci-release-automator"
description: "Autonomous CI/CD and build workflow orchestrator for ProximaX Sirius Core. Monitors GitHub Actions workflows across architectures (Linux amd64/arm64, macOS, Windows WSL), isolates runner errors, and diagnoses build failures with root causes and remediations."
---

# CI & Build Automator Runbook

This skill equips the agent to monitor GitHub Actions CI workflows, track multi-architecture matrix builds, isolate runner errors, and enforce cross-platform build validation.

## Core Capabilities

1. **GitHub Actions Workflow Monitoring & Triage**:
   - Checks CI pipeline status via `gh run list --limit 5` and `gh run watch`.
   - On matrix build failures, isolates failing runner jobs (macOS arm64, Linux x86_64, Linux aarch64, Windows WSL2).
   - Retrieves and analyzes only failed step output using `gh run view <run_id> --log-failed`.
   - Diagnoses exact compiler, linker, or test errors and proposes concrete code fixes.

2. **Cross-Platform Invariant Enforcement**:
   - Isolates Windows logic to `*_windows.go` and `runtime.GOOS == "windows"`.
   - Enforces `libatomic1` presence on Linux/WSL environments.
   - Enforces `.dockerignore` ignoring `chainconfig/data/` and `chainconfig/logs/`.
   - Verifies Docker builds and GitHub Actions matrix jobs stay synchronized with source changes.
