# Implementation Plan: Pure Browser Cockpit & Complete Electron Removal

**Branch**: `005-remove-electron-pure-browser` | **Date**: 2026-09-18 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/005-remove-electron-pure-browser/spec.md`

## Summary

Dismantle all Electron desktop app wrappers, delete `electron/` directory and `sign_app.py`, remove Electron IPC hooks and quit dialogs from React frontend, and align all platform packaging scripts to produce pure standalone browser-cockpit bundles.

## Technical Context

**Language/Version**: Go 1.22+, React 18.3+, TypeScript 5.5+, TailwindCSS

**Target Architecture**: Native Go supervisor with embedded static React build, serving web UI on `http://localhost:8080` (Cockpit).

## Constitution Check

- [x] **Key Privacy**: No key masking logic modified.
- [x] **Architecture Standards**: Aligns 100% with Principle 4: "Go backend + embedded React/TypeScript UI served on port 8080".
- [x] **Cross-Platform Isolation**: Standalone Go binary handles all platforms cleanly.

## Project Structure Changes

```text
electron/                                     # [DELETE: Entire directory]
scripts/macos/sign_app.py                    # [DELETE: Obsolete .app signer]
frontend/src/components/QuitConfirmModal.tsx # [DELETE: Obsolete Electron quit modal]
frontend/src/components/Header.tsx           # [CLEAN: Remove isElectron and app-drag]
frontend/src/App.tsx                         # [CLEAN: Remove electronAPI hooks and QuitConfirmModal]
.gitignore                                   # [CLEAN: Remove electron/ and .app exclusions]
```
