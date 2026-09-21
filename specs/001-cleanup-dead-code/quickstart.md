# Quickstart & Verification Guide

## Prerequisites
- Go 1.22+ installed
- Node 18+ / 20+ installed
- Bash shell

## Verification Workflow

1. **Verify Backend Build & Unit Tests**:
   ```bash
   cd backend
   go test -v ./...
   ```
   *Expected Outcome*: All packages compile and all unit tests pass with zero failures.

2. **Verify Frontend Build**:
   ```bash
   cd frontend
   npm run build
   ```
   *Expected Outcome*: TypeScript compilation and Vite build succeed without error (`built in ...s`).

3. **Verify Packaging Scripts Integrity**:
   - Check `scripts/macos/package-macos.sh`
   - Check `scripts/linux/package-linux.sh`
   - Check `scripts/windows/package-windows.ps1`
   *Expected Outcome*: All platform packagers only reference active binaries (`sirius-core` and engine components).
