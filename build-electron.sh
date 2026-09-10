#!/bin/bash
set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$DIR"

echo "========================================================="
echo "  Building ProximaX Sirius Standalone Native Desktop App"
echo "  Architecture: Apple Silicon ARM64 (Zero Docker)"
echo "========================================================="

# 1. Setup Node 20
if [ -f "$HOME/.nvm/nvm.sh" ]; then
    source "$HOME/.nvm/nvm.sh"
    nvm use 20 2>/dev/null || true
fi

# 2. Build Frontend UI
echo "-> 1/3 Building React UI..."
(cd frontend && npm run build)

# 3. Build Go Backend with Embedded UI
echo "-> 2/3 Compiling native Go backend..."
(cd backend && go build -o sirius-core .)

# 4. Bundle all dynamic libraries (.dylib) for 100% standalone distribution
echo "-> 3/4 Bundling relocatable dynamic libraries..."
python3 scripts/bundle_dylib_deps.py

# 5. Stage Clean Default Configurations (Prevent Packaging Private Keys)
echo "-> 4/5 Sanitizing release configuration templates..."
rm -rf electron/staging
mkdir -p electron/staging/chainconfig/resources electron/staging/chainconfig/data/00000
cp -R chainconfig/resources/* electron/staging/chainconfig/resources/

# Ensure active private keys are stripped from release bundle
rm -f electron/staging/chainconfig/resources/config-harvesting.properties
rm -f electron/staging/chainconfig/resources/config-storage.properties
rm -f electron/staging/chainconfig/resources/config-user.properties
rm -f electron/staging/chainconfig/resources/config-manager.properties
rm -f electron/staging/chainconfig/resources/harvest-stats.json
rm -rf electron/staging/chainconfig/logs electron/staging/chainconfig/replicator_service_logs

# Place clean default templates as starter configs
if [ -f "chainconfig/resources/config-harvesting.properties.template" ]; then
    cp chainconfig/resources/config-harvesting.properties.template electron/staging/chainconfig/resources/config-harvesting.properties
fi
if [ -f "chainconfig/resources/config-storage.properties.template" ]; then
    cp chainconfig/resources/config-storage.properties.template electron/staging/chainconfig/resources/config-storage.properties
fi
if [ -f "chainconfig/resources/config-user.properties.template" ]; then
    cp chainconfig/resources/config-user.properties.template electron/staging/chainconfig/resources/config-user.properties
fi
if [ -f "chainconfig/data/00000/00001.dat" ]; then
    cp chainconfig/data/00000/00001.dat electron/staging/chainconfig/data/00000/
    cp chainconfig/data/00000/hashes.dat electron/staging/chainconfig/data/00000/
fi

# 6. Package Electron App
echo "-> 5/5 Packaging macOS Desktop Application..."
cd electron
if [ ! -d "node_modules" ]; then
    echo "   Installing electron dependencies..."
    npm install
fi

npm run build:mac
rm -rf staging

echo "-> Updating /Applications and local app bundle..."
rm -rf "/Applications/ProximaX Sirius Mainnet Peer Node.app" "$DIR/ProximaX Sirius Mainnet Peer Node.app"
cp -R "dist/mac-arm64/ProximaX Sirius Mainnet Peer Node.app" "/Applications/ProximaX Sirius Mainnet Peer Node.app"
cp -R "dist/mac-arm64/ProximaX Sirius Mainnet Peer Node.app" "$DIR/ProximaX Sirius Mainnet Peer Node.app"

echo "-> Applying macOS code signature..."
xattr -cr "/Applications/ProximaX Sirius Mainnet Peer Node.app" "$DIR/ProximaX Sirius Mainnet Peer Node.app" "dist/mac-arm64/ProximaX Sirius Mainnet Peer Node.app"
codesign --force --deep --sign - "/Applications/ProximaX Sirius Mainnet Peer Node.app"
codesign --force --deep --sign - "$DIR/ProximaX Sirius Mainnet Peer Node.app"

echo "========================================================="
echo "  Build Complete!"
echo "  App Location:  /Applications/ProximaX Sirius Mainnet Peer Node.app"
echo "  Installer DMG: electron/dist/ProximaX Sirius Mainnet Peer Node-1.9.8-arm64.dmg"
echo ""
echo "  To launch:"
echo "    open \"/Applications/ProximaX Sirius Mainnet Peer Node.app\""
echo "========================================================="
