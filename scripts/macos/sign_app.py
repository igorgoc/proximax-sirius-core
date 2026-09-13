#!/usr/bin/env python3
import os, sys, subprocess, shutil

def sign_app(app_path):
    print(f"-> Performing inside-out code signing on: {app_path}")
    if not os.path.exists(app_path):
        print(f"Error: {app_path} does not exist")
        sys.exit(1)

    # 0. Clean extended attributes (quarantine / resource forks)
    subprocess.run(['xattr', '-cr', app_path], check=False)

    # 1. Sign all dylibs and binaries in Resources/bin
    res_bin = os.path.join(app_path, 'Contents/Resources/bin')
    if os.path.exists(res_bin):
        for root, _, files in os.walk(res_bin):
            for f in files:
                p = os.path.join(root, f)
                if not os.path.islink(p):
                    subprocess.run(['codesign', '--force', '--sign', '-', p], check=False)

    # 2. Sign backend binary
    backend_bin = os.path.join(app_path, 'Contents/Resources/sirius-core')
    if os.path.exists(backend_bin):
        subprocess.run(['codesign', '--force', '--sign', '-', backend_bin], check=False)

    # 3. Sign all Frameworks and Helpers inside-out
    fw_dir = os.path.join(app_path, 'Contents/Frameworks')
    if os.path.exists(fw_dir):
        # Sign helper executables and nested frameworks
        for item in sorted(os.listdir(fw_dir)):
            p = os.path.join(fw_dir, item)
            if item.endswith('.framework') or item.endswith('.app'):
                subprocess.run(['codesign', '--force', '--deep', '--sign', '-', p], check=False)

    # 4. Sign main executable
    macos_dir = os.path.join(app_path, 'Contents/MacOS')
    if os.path.exists(macos_dir):
        for item in os.listdir(macos_dir):
            p = os.path.join(macos_dir, item)
            subprocess.run(['codesign', '--force', '--sign', '-', p], check=False)

    # 5. Sign the outer bundle
    subprocess.run(['codesign', '--force', '--sign', '-', app_path], check=False)

    # 6. Verify signature strictly
    res = subprocess.run(['codesign', '--verify', '--deep', '--strict', app_path], capture_output=True, text=True)
    if res.returncode != 0:
        print(f"Code signing verification failed:\n{res.stderr}")
        sys.exit(1)
    
    print(f"✓ Verified signature: {app_path} is valid on disk and satisfies Designated Requirement!")

if __name__ == '__main__':
    target = sys.argv[1] if len(sys.argv) > 1 else 'electron/dist/mac-arm64/ProximaX Sirius Mainnet Peer Node.app'
    sign_app(target)
