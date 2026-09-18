# Cross-Platform Architecture & Knowledge Base
## ProximaX Sirius Core Node: Windows, Linux, and macOS

This document aggregates critical lessons learned, architectural invariants, system traps, and best practices discovered across Windows (WSL2), Linux (native), and macOS environments. It serves as the cross-platform reference for developers and autonomous AI coding agents working across heterogeneous operating systems.

---

## 1. Windows System Output Localization Trap

### The Problem
Windows command-line utilities (`wsl.exe`, `netsh.exe`, `dism.exe`, `ipconfig.exe`, `sc.exe`) translate standard output based on the active Windows display language / system locale:
- **English**: `NAME`, `STATE`, `VERSION`, `"The Windows Subsystem for Linux optional component is not enabled"`
- **German**: `NAME`, `STATUS`, `VERSION`, `"Standardversion"`, `"Die optionale Komponente..."`
- **French**: `NOM`, `ÉTAT`, `VERSION`, `"Version par défaut"`, `"Le composant facultatif..."`
- **Russian**: `ИМЯ`, `СОСТОЯНИЕ`, `ВЕРСИЯ`, `"Версия по умолчанию"`
- **Chinese**: `名称`, `状态`, `版本`, `"默认版本"`

### The Danger
Any code that parses CLI text using English strings (e.g. `strings.Contains(out, "optional component")` or `strings.HasPrefix(line, "NAME")`) will silently fail on non-English machines, routing users into wrong remediation states or failing to detect installed distributions.

### Invariant Rules for Windows CLI Parsing
1. **Rely on Process Exit Codes First**:
   - If `wsl.exe --status` exits with non-zero, WSL2 is not ready or not installed. Never rely on matching `"optional component"`.
2. **Parse Language-Agnostic HRESULT Hex Codes**:
   - Windows consistently outputs invariant 32-bit hex error codes across all languages:
     - `0x80370102` (`WSL_E_VIRTUAL_MACHINE_PREREQUISITE_MINIMUM`): CPU virtualization disabled in BIOS/UEFI, or system reboot required after enabling VirtualMachinePlatform.
     - `0x80072ee7` (`ERROR_INTERNET_NAME_NOT_RESOLVED`): Network timeout / Microsoft Store or WSL CDN unreachable.
     - `0x8024500c` (`WU_E_REDIRECTOR_CONNECT_POLICY`): Windows Store / Windows Update blocked by corporate Group Policy.
     - `1223` (`ERROR_CANCELLED`): User clicked "No" on elevated UAC prompt.
   - Inspect both the output text stream and the process exit code (`uint32(exitErr.ExitCode())`).
3. **Use Structural / Type Invariants Instead of Header Names**:
   - To skip table headers in `wsl.exe -l -v`, do not match `"NAME"` or `"STATE"`. Instead, inspect the last column: a valid distribution row *always* ends with numeric version `"1"` or `"2"`. Header rows never end in `"1"` or `"2"` in any language.

---

## 2. Text Encoding & Pipe Traps on Windows

1. **UTF-16LE Output with or without BOM**:
   - `wsl.exe` and PowerShell frequently output UTF-16LE text interleaved with null bytes (`\x00`).
   - Always sanitize CLI output with a decode function (`cleanWSLOutput`) that detects UTF-16LE BOM (`0xFF, 0xFE`), null-interleaving, and falls back to UTF-8/ASCII stripping null bytes.
2. **PowerShell Script Execution Policy**:
   - On Windows, `npm` installed globally may resolve to `npm.ps1` under PowerShell, which is blocked by default ExecutionPolicy (`Restricted`).
   - Run Node/frontend builds using `cmd.exe /c npm run build` to bypass PowerShell script-signing restrictions.

---

## 3. Storage, State Synchrony & Shutdown Latency

### The RocksDB + Flat-File Co-dependency
Sirius Catapult stores blockchain state in two coupled storage layers:
1. Flat binary files (`supplemental.dat`, `BlockDifficultyCache.dat`, `index.dat`).
2. RocksDB key-value column families (`statedb/`).

**Invariants**:
- **Never perform flat-file byte surgery**: Modifying `index.dat` or flat files independently desynchronizes the RocksDB Merkle state tree, triggering `FastFinalityActions.cpp: rejecting block, signer invalid` and peer disconnections (`Verify_Error`).
- **Let `catapult.recovery` reconcile WAL**: RocksDB WAL logs are automatically reconciled by `catapult.recovery` at height > 1.
- **Aborted boot cleanup at height <= 1**: If height <= 1, partial `statedb` from interrupted Nemesis runs must be wiped so `NemesisBlockLoader` boots cleanly.

### DrvFS vs ext4 Latency and Shutdown Grace Periods
- Windows WSL2 mounts Windows NTFS drives under `/mnt/c/` via 9P DrvFS. DrvFS write latency is significantly higher than native Linux ext4.
- High-block-commit rates require time to flush memory tables to disk during shutdown.
- **Graceful Shutdown**: The supervisor sends `SIGINT` to `sirius.bc` and monitors multi-signal progress (log growth, log rotation, storage height updates in `index.dat`).
- **Never kill prematurely**: As long as forward progress is detected, the supervisor allows the engine to commit cleanly. Only if 60 consecutive seconds elapse with zero progress does it escalate to `SIGKILL`.
- **Sync filesystem caches**: Always execute `sync` inside WSL2 after process exit to commit dirty pages to disk before restarting.

---

## 4. Port Forwarding & Networking

### Architecture by OS:
| Platform | Engine Runtime | P2P Port 7900 | API Port 7901 | REST Port 3000 | PortProxy Required? |
|---|---|---|---|---|---|
| **Linux** | Native ELF | Direct bind | Direct bind | Direct bind | No |
| **macOS** | Docker Container | Host bridge | Host bridge | Host bridge | No (Docker handles) |
| **Windows** | WSL2 (Hyper-V VM) | WSL IP | WSL IP | WSL IP | **Yes** (netsh portproxy) |

### PortProxy Invariant
- WSL2 uses an internal Hyper-V virtual switch with a dynamic private IP.
- Windows host supervisor bridges inbound traffic using `netsh interface portproxy add v4tov4`.
- **Sequencing Invariant**: `SetupPortProxy()` MUST only be executed *after* WSL2 is verified running and `WSL2Ready`. Running it prematurely before WSL initializes causes unnecessary error logs and transient UAC prompts.
- **Zero-UAC Query**: Always query `netsh interface portproxy show all` first without elevation. If rules already match current WSL IP, skip elevation completely. If updates are needed, batch all ports into a single elevated command.

---

## 5. Dynamic Runtime Dependencies

### `libatomic1` Invariant (Ubuntu WSL2)
- Sirius Catapult's `libextension.fastfinality.so` requires `libatomic.so.1` for 64-bit and 128-bit atomic instructions.
- Standard minimal Ubuntu 22.04 WSL2 images omit `libatomic1`.
- The distribution provides `bin/libatomic.so.1` and the supervisor executes an automatic self-healing pre-flight check in WSL:
  ```sh
  dpkg -s libatomic1 >/dev/null 2>&1 || (apt-get update -qq && apt-get install -y -qq libatomic1)
  ```

---

## 6. Cross-Platform Code Isolation Rules

1. **Strict Build Tags**:
   - All Windows-specific APIs, PowerShell scripts, and WSL management reside strictly in files named `*_windows.go` with `//go:build windows`.
   - Non-Windows fallbacks reside in `*_other.go` with `//go:build !windows`.
   - General Go files use `if runtime.GOOS == "windows"` when calling platform-specific supervisor methods.
2. **Path Normalization**:
   - Always use `filepath.ToSlash` and `filepath.FromSlash` appropriately.
   - Internal paths stored in configuration files must use host formats (`C:\...` on Windows, `/var/...` on Linux).
   - WSL execution paths must be converted via `ToWSLPath` (`/mnt/c/...`).

---

## 7. Synchronizing Multiple Agent Sessions Across OSs

When working in parallel across Windows, Linux, and macOS environments:
1. **Rule File Synchronization**: Keep `GEMINI.md` checked into the repository root so every agent session automatically inherits the architectural invariants and storage rules.
2. **Knowledge Base Updates**: When a platform-specific trap (such as Windows CLI localization or macOS Docker volume binding) is discovered and fixed, document it here in `CROSS_PLATFORM_KNOWLEDGE.md` and commit to Git.
3. **Fail-Closed Verification**: Ensure all cross-platform paths are tested with automated Go tests (`go test ./pkg/...`) before committing.

---

## 8. Multi-Architecture Binary Contamination & Shell Syntax Error Trap

### The Trap: "Syntax error: `|` unexpected (expecting `)`)"
When an executable binary fails with shell errors like:
```
/bin/sirius.bc: 1: Syntax error: "|" unexpected (expecting ")")
catapult.recovery: 2: Syntax error: ")" unexpected
```
This is **never** a script syntax error. It occurs because:
1. The file is NOT a valid ELF binary for the host/guest architecture (e.g. it is a macOS Mach-O 64-bit arm64 binary checked out on a Linux/WSL x86_64 host).
2. The Linux kernel's `execve()` fails with `ENOEXEC` (*Exec format error*).
3. POSIX shells (`/bin/sh`) fallback to interpreting the file as an un-shebanged shell script.
4. The shell interprets the binary machine code bytes (`0xCF, 0xFA, 0xED, 0xFE` or load commands) as ASCII text, encountering characters like `|` or `)` and failing with a syntax error.

### Prevention & Auto-Healing Rules:
1. **Canonical Git Binaries**: The tracked engine binaries in `bin/` (`sirius.bc`, `catapult.recovery`, `catapult.broker`) must strictly remain Linux ELF x86_64 binaries.
2. **Never Commit Architecture-Swapped Binaries**: Agents on macOS downloading Darwin arm64 binaries must NOT commit them back to Git.
3. **Automated Supervisor Self-Healing**:
   - Both `start.sh` (on Linux) and `executeWSL()` in `backend/pkg/supervisor/wsl_windows.go` (on Windows) check the 4-byte magic number `\x7fELF` before invoking engine binaries.
   - If an architecture mismatch (Mach-O, PE, or corrupted) is detected, the supervisor automatically downloads and extracts the official `sirius-linux-amd64.tar.gz` release archive without manual intervention.
