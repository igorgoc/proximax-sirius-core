# Research: Pure Browser Cockpit Simplification

## Decision 1: Complete Deletion of `electron/`
- **Rationale**: The Go supervisor already provides native HTTP serving on `:8080`, system tray / CLI lifecycle control (`start.sh`, `stop.sh`, `run.bat`), and native subprocess management. Electron adds unnecessary bundle weight and OS signing complexity without providing blockchain value.

## Decision 2: Elimination of Window Quit Interceptors
- **Rationale**: In desktop browser mode, tab closure does not and should not stop a background blockchain node. Node shutdown is handled explicitly via the UI's "Stop Node" button or platform scripts (`stop.sh`, `stop.bat`, `systemctl`).
