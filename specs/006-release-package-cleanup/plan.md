# Implementation Plan - Release Packages Standardization & Single Start/Stop Layout

## Execution Phases

1. **Clean Downloads Packages**:
   - Clean `/Users/igorgoc/Downloads/proximax-sirius-core-windows`.
   - Clean `/Users/igorgoc/Downloads/proximax-sirius-core-linux`.
   - Clean `/Users/igorgoc/Downloads/proximax-sirius-core-mac`.
2. **Generate Standardized Package READMEs**:
   - Write `README.md` for Windows package.
   - Write `README.md` for Linux package.
   - Write `README.md` for macOS package.
3. **Update Repository Packaging Scripts**:
   - Update `scripts/windows/package-windows.ps1` to stop staging secondary scripts at root and include `README.md`.
   - Update `scripts/linux/package-linux.sh` to stop staging secondary scripts at root and include `README.md`.
   - Update `scripts/macos/package-macos.sh` to stop staging secondary scripts at root and include `README.md`.
   - Remove obsolete `WINDOWS_HANDOVER.md` and `LINUX_HANDOVER.md` from the repo.
4. **Verification**:
   - Run `package-macos.sh` to test packaging on macOS host.
   - Verify all unit tests pass with `go test -race ./...`.
