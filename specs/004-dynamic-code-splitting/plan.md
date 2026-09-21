# Implementation Plan: Dynamic Code-Splitting for Cockpit UI

**Branch**: `004-dynamic-code-splitting` | **Date**: 2026-09-17 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/004-dynamic-code-splitting/spec.md`

## Summary

Convert secondary subtabs (`ConfigTab`, `MaintenanceTab`, `LogsTab`, `StorageTab`, `ValidatorTab`, `NetworkTab`) and modal dialogs in `App.tsx` into dynamic `React.lazy()` components with a responsive `Suspense` fallback.

## Technical Context

**Language/Version**: React 18.3+, TypeScript 5.5+, Vite 5.4+

**Pattern**: `React.lazy(() => import('./components/TabName').then(m => ({ default: m.TabName })))`

**Fallback Component**: High-performance cyber-operator loading spinner styled with TailwindCSS.

## Constitution Check

- [x] **Key Privacy**: No key masking logic modified.
- [x] **State Isolation**: Background polling (`/api/status`) continues unimpeded in `App.tsx` regardless of which tab is loaded.
- [x] **Cross-Platform Isolation**: No changes to OS-specific APIs.

## Project Structure

```text
frontend/src/
├── App.tsx                        # [MODIFY: Replace static tab imports with React.lazy + Suspense]
```
