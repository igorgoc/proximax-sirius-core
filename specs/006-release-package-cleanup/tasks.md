# Tasks: Release Packages Standardization & Single Start/Stop Layout

- [x] 1. Clean up downloaded Windows package in `/Users/igorgoc/Downloads/proximax-sirius-core-windows` <!-- id: 1 -->
  - Delete `WINDOWS_HANDOVER.md`
  - Remove root clutter scripts (`WINDOWS_DEFENDER_NOTES.md`, `pick-directory.ps1`, `restart-node.ps1`, `restart.bat`, `run.bat`, `setup-wsl.bat`, `setup-wsl.ps1`, `start-node.ps1`, `stop-node.ps1`)
  - Create comprehensive `README.md`
- [x] 2. Clean up downloaded Linux package in `/Users/igorgoc/Downloads/proximax-sirius-core-linux` <!-- id: 2 -->
  - Remove root clutter scripts (`restart.sh`, `run.sh`, `start-node.sh`, `reset_to_genesis.sh`)
  - Create comprehensive `README.md`
- [x] 3. Clean up downloaded macOS package in `/Users/igorgoc/Downloads/proximax-sirius-core-mac` <!-- id: 3 -->
  - Remove root clutter script (`restart.sh`)
  - Create comprehensive `README.md`
- [x] 4. Update repository packaging scripts & documentation <!-- id: 4 -->
  - Update `scripts/windows/package-windows.ps1`
  - Update `scripts/linux/package-linux.sh`
  - Update `scripts/macos/package-macos.sh`
  - Delete obsolete `WINDOWS_HANDOVER.md` and `LINUX_HANDOVER.md` from repository root
- [x] 5. Verify packaging and test suite <!-- id: 5 -->
  - Test macOS packaging script
  - Run `go test -race ./...`
