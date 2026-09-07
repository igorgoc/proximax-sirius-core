# ProximaX Sirius Core Native — Windows Deployment & Security Guide

## 1. PowerShell Execution Policy
When executing `start-node.ps1` or `stop-node.ps1` for the first time, Windows PowerShell may restrict script execution under default policies.

To permit running local scripts in your current session:
```powershell
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
.\start-node.ps1
```
Or launch directly with the `-ExecutionPolicy Bypass` flag:
```powershell
powershell -ExecutionPolicy Bypass -File .\start-node.ps1
```

## 2. NTFS ACL Security Hardening (POSIX 0600 Equivalent)
`start-node.ps1` automatically executes `icacls` on sensitive key files (`chainconfig\resources\config-harvesting.properties`, `config-user.properties`):
```powershell
icacls "$file" /inheritance:r /grant:r "$($env:USERNAME):(R,W)" /grant:r "SYSTEM:(R,W)"
```
- **Inheritance Disabled (`/inheritance:r`)**: Prevents unauthorized parent folder permissions (e.g. `Users` group) from cascading onto private keys.
- **Restricted Access**: Only your current interactive Windows user account and `NT AUTHORITY\SYSTEM` are granted read and write permissions.

## 3. Windows SmartScreen & Code Signing
If downloading pre-compiled community binaries without an Extended Validation (EV) code signing certificate, Windows Defender SmartScreen may display:
> "Windows protected your PC — Microsoft Defender SmartScreen prevented an unrecognized app from starting."

To proceed safely:
1. Click **"More info"**.
2. Click **"Run anyway"**.
3. Verify the SHA-256 checksum of your downloaded ZIP archive against the official `SHA256SUMS` published on the GitHub release:
   ```powershell
   Get-FileHash .\proximax-sirius-windows-amd64-1.9.7.zip -Algorithm SHA256
   ```

## 4. Production Service Setup (NSSM / WinSW)
To run ProximaX Sirius Core as an automatic Windows System Service (starting on boot without user login), use **NSSM (Non-Sucking Service Manager)**:
```powershell
nssm install ProximaXSirius "C:\proximax-sirius-core\sirius-core.exe"
nssm set ProximaXSirius AppDirectory "C:\proximax-sirius-core"
nssm set ProximaXSirius AppStdout "C:\proximax-sirius-core\chainconfig\logs\service.log"
nssm set ProximaXSirius AppStderr "C:\proximax-sirius-core\chainconfig\logs\service_error.log"
nssm start ProximaXSirius
```
