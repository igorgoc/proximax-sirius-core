# Implementation Plan: Frontend Cleanup and Bundle Optimization

**Branch**: `002-frontend-cleanup` | **Date**: 2026-09-17 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/002-frontend-cleanup/spec.md`

## Summary

Delete orphaned `SnapshotModal.tsx` and configure Vite manual chunking for vendor libraries (`lucide-react`, `react-vendor`) to eliminate the 500 kB chunk warning and accelerate initial cockpit load time.

## Technical Context

**Language/Version**: TypeScript 5.5+, React 18.3+, Vite 5.4+

**Primary Dependencies**: `lucide-react`, `@vitejs/plugin-react`

**Testing**: `npm run build` in `frontend/`, `go test ./...` in `backend/`

**Target Platform**: Multi-platform web dashboard embedded in Go binary

## Constitution Check

- [x] **Key Privacy**: No changes to password masking or key isolation.
- [x] **Storage & Fast-Sync**: `SnapshotSubTab.tsx` continues to manage direct streaming restore without regressions.
- [x] **Cross-Platform Isolation**: No changes to OS-specific routines.

## Project Structure

```text
frontend/
├── src/
│   ├── components/
│   │   ├── SnapshotModal.tsx      # [DELETE: Orphaned legacy modal]
│   │   └── SnapshotSubTab.tsx     # [PRESERVE: Active streaming UI]
└── vite.config.ts                 # [OPTIMIZE: Configure manualChunks]
```
