# Implementation Plan: Codebase Cleanup and Dead Artifact Removal

**Branch**: `001-cleanup-dead-code` | **Date**: 2026-09-17 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/001-cleanup-dead-code/spec.md`

## Summary

Safely remove obsolete prototype CLI commands and legacy test scripts from `backend/cmd/` and `scripts/` while verifying that active production supervisors, packaging scripts, and Go unit tests retain 100% integrity.

## Technical Context

**Language/Version**: Go 1.22+, TypeScript 5+, Bash, PowerShell 5.1+

**Primary Dependencies**: Go standard library (`net/http`, `os/exec`), React 18, Vite 5, TailwindCSS

**Storage**: RocksDB (C++ Catapult engine), local properties files (`config-user.properties`, `config-harvesting.properties`)

**Testing**: `go test ./...` in `backend/`, `npm run build` in `frontend/`

**Target Platform**: macOS (arm64/x86_64), Linux (Ubuntu 22.04+ x86_64), Windows 10/11 x64 (Native + WSL2)

**Project Type**: Multi-platform node supervisor & web dashboard (Cockpit)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Key Privacy**: No changes to private key handling; zero keys exposed.
- [x] **Storage & Fast-Sync**: `backend/pkg/snapshot` retains the direct streaming decompression architecture; no flat-file byte surgery.
- [x] **Cross-Platform Isolation**: Platform scripts and Go build tags remain strictly segregated across `macos/`, `linux/`, and `windows/`.
- [x] **No Flat-File Byte Surgery**: No manipulation of RocksDB or chain data structures.

## Project Structure

### Documentation (this feature)

```text
specs/001-cleanup-dead-code/
├── spec.md              # Feature specification
├── plan.md              # Implementation plan
├── research.md          # Analysis & audit justifications
├── quickstart.md        # Verification quickstart guide
├── checklists/
│   └── requirements.md  # Quality checklist
└── tasks.md             # Implementation tasks
```

### Source Code Impact

```text
backend/
├── cmd/
│   ├── fast-extract/           # [REMOVE: Obsolete standalone CLI extractor]
│   ├── stream-chunk-restore/   # [REMOVE: Obsolete standalone chunk restore tool]
│   └── sign-release/          # [KEEP: Production release signing tool]
scripts/
├── common/
│   └── audit_chunks.py         # [REMOVE: Unused prototype flat-file script]
├── linux/
│   ├── fast-snapshot.sh        # [REMOVE: Outdated Docker-era downloader]
│   ├── test_config_persistence.sh      # [REMOVE: Superseded by Go unit tests]
│   └── test_transition_persistence.sh  # [REMOVE: Superseded by Go unit tests]
```

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None | N/A | Deletion of dead code directly simplifies the codebase. |
