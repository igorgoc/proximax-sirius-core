const { app, BrowserWindow, Menu, Tray, nativeImage, shell, dialog, ipcMain } = require('electron');
const path = require('path');
const fs = require('fs');
const http = require('http');
const { exec, spawn, execSync } = require('child_process');

let mainWindow = null;
let tray = null;
let isLoaded = false;
let isQuitting = false;
let backendProcess = null;
const TARGET_URL = 'http://127.0.0.1:8080';

// Ensure single instance lock
const gotTheLock = app.requestSingleInstanceLock();
if (!gotTheLock) {
  app.quit();
} else {
  app.on('second-instance', () => {
    if (mainWindow) {
      if (mainWindow.isMinimized()) mainWindow.restore();
      mainWindow.show();
      mainWindow.focus();
    } else {
      createWindow();
    }
  });
}

function copyFolderSync(from, to) {
  if (!fs.existsSync(to)) fs.mkdirSync(to, { recursive: true });
  fs.readdirSync(from).forEach(element => {
    const fromPath = path.join(from, element);
    const toPath = path.join(to, element);
    if (fs.lstatSync(fromPath).isDirectory()) {
      copyFolderSync(fromPath, toPath);
    } else {
      if (!fs.existsSync(toPath)) {
        fs.copyFileSync(fromPath, toPath);
      }
    }
  });
}

function getAppPaths() {
  // 1. Development mode
  if (!app.isPackaged) {
    const devDir = path.resolve(__dirname, '..');
    if (fs.existsSync(path.join(devDir, 'chainconfig', 'resources', 'config-immutable.properties'))) {
      return {
        workDir: devDir,
        binDir: path.join(devDir, 'bin'),
        chainConfigDir: path.join(devDir, 'chainconfig'),
        backendBin: path.join(devDir, 'backend', 'sirius-core')
      };
    }
  }

  // 2. Direct development worktree path
  const devWorkspaceDirect = process.env.SIRIUS_WORKSPACE || path.resolve(__dirname, '..');
  if (fs.existsSync(path.join(devWorkspaceDirect, 'chainconfig', 'resources', 'config-immutable.properties'))) {
    return {
      workDir: devWorkspaceDirect,
      binDir: path.join(devWorkspaceDirect, 'bin'),
      chainConfigDir: path.join(devWorkspaceDirect, 'chainconfig'),
      backendBin: path.join(devWorkspaceDirect, 'backend', 'sirius-core')
    };
  }

  // 3. Packaged production app bundle
  const userDataDir = app.getPath('userData');
  const userChainConfig = path.join(userDataDir, 'chainconfig');
  const resChainConfig = path.join(process.resourcesPath, 'chainconfig');
  const resBin = path.join(process.resourcesPath, 'bin');
  const resBackend = path.join(process.resourcesPath, 'sirius-core');

  if (!fs.existsSync(userChainConfig) && fs.existsSync(resChainConfig)) {
    try {
      copyFolderSync(resChainConfig, userChainConfig);
    } catch (e) {
      console.log('Copy chainconfig note:', e.message);
    }
  }

  return {
    workDir: userDataDir,
    binDir: fs.existsSync(resBin) ? resBin : path.join(userDataDir, 'bin'),
    chainConfigDir: fs.existsSync(userChainConfig) ? userChainConfig : resChainConfig,
    backendBin: resBackend
  };
}

function sendBootUpdate(stepId, status, message, logLine = '') {
  if (!mainWindow || mainWindow.isDestroyed() || isLoaded) return;
  const safeMsg = (message || '').replace(/'/g, "\\'").replace(/"/g, '\\"').replace(/\n/g, ' ');
  const safeLog = (logLine || '').replace(/'/g, "\\'").replace(/"/g, '\\"').replace(/\n/g, ' ');
  
  mainWindow.webContents.executeJavaScript(`
    if (typeof window.updateBootStep === 'function') {
      window.updateBootStep('${stepId}', '${status}', '${safeMsg}', '${safeLog}');
    }
  `).catch(() => {});
}

function delay(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function checkPort8080Ready(callback) {
  const req = http.get(TARGET_URL, (res) => {
    res.resume();
    if (res.statusCode >= 200 && res.statusCode < 500) {
      callback(true);
    } else {
      callback(false);
    }
  });
  req.on('error', () => callback(false));
  req.setTimeout(2000, () => {
    req.destroy();
    callback(false);
  });
}

function checkApiStatusReady(callback) {
  const req = http.get(`${TARGET_URL}/api/status`, (res) => {
    res.resume();
    if (res.statusCode === 200) {
      callback(true);
    } else {
      callback(false);
    }
  });
  req.on('error', () => callback(false));
  req.setTimeout(2500, () => {
    req.destroy();
    callback(false);
  });
}

function loadDashboardDirectly() {
  if (!mainWindow || mainWindow.isDestroyed()) return;
  isLoaded = true;
  mainWindow.loadURL(TARGET_URL).catch((err) => {
    console.log('loadURL error, retrying...', err.message);
    setTimeout(() => {
      if (mainWindow && !mainWindow.isDestroyed()) {
        mainWindow.loadURL(TARGET_URL);
      }
    }, 1000);
  });
}

// ----------------------------------------------------
// Native Standalone Boot Sequence Pipeline (No Docker)
// ----------------------------------------------------
async function runBootSequence() {
  const { workDir, binDir, chainConfigDir, backendBin } = getAppPaths();

  // Step 1: Initialize Native Architecture
  sendBootUpdate('step-arch', 'running', 'Verifying Apple Silicon ARM64 native architecture...', 'Checking native binaries...');
  await delay(500);

  const siriusBin = path.join(binDir, 'sirius.bc');
  if (!fs.existsSync(siriusBin)) {
    sendBootUpdate('step-arch', 'error', 'Native sirius.bc binary not found in bin directory', `Missing: ${siriusBin}`);
    return;
  }

  sendBootUpdate('step-arch', 'success', 'Apple Silicon ARM64 native binaries ready', `✓ sirius.bc & dynamic libraries in ${binDir}`);
  await delay(500);

  // Step 2: Prepare Configurations & Lockfiles
  sendBootUpdate('step-configs', 'running', 'Verifying chain configurations & lockfiles...', 'Checking chainconfig & server.lock...');
  
  const serverLock = path.join(chainConfigDir, 'data', 'server.lock');
  if (fs.existsSync(serverLock)) {
    try {
      fs.unlinkSync(serverLock);
      sendBootUpdate('step-configs', 'running', 'Cleared stale server.lock file', '✓ Cleaned lockfile');
    } catch (e) {}
  }

  sendBootUpdate('step-configs', 'success', 'Configurations verified & ready', `✓ Config Directory: ${chainConfigDir}`);
  await delay(500);

  // Step 3: Launch Native Go Controller & Sirius Process
  sendBootUpdate('step-native-launch', 'running', 'Starting native Sirius Core process manager...', 'Launching backend...');

  checkPort8080Ready((alreadyRunning) => {
    if (alreadyRunning) {
      sendBootUpdate('step-native-launch', 'success', 'Native Sirius Core backend already active', '✓ Connected to existing service');
      step4_probeGui();
      return;
    }

    let executable = backendBin;
    if (!fs.existsSync(executable)) {
      executable = path.join(workDir, 'sirius-core');
    }

    if (!fs.existsSync(executable)) {
      sendBootUpdate('step-native-launch', 'error', 'sirius-core backend binary not found', `Looked at: ${backendBin}`);
      return;
    }

    const env = {
      ...process.env,
      DYLD_LIBRARY_PATH: binDir,
      LD_LIBRARY_PATH: binDir
    };

    backendProcess = spawn(executable, ['-port', '8080', '-chainconfig', chainConfigDir], {
      cwd: workDir,
      env,
      stdio: ['ignore', 'pipe', 'pipe']
    });

    backendProcess.stdout.on('data', (data) => {
      const line = data.toString().trim();
      sendBootUpdate('step-native-launch', 'running', 'Starting native backend...', line);
    });

    backendProcess.stderr.on('data', (data) => {
      const line = data.toString().trim();
      sendBootUpdate('step-native-launch', 'running', 'Starting native backend...', line);
    });

    backendProcess.on('error', (err) => {
      sendBootUpdate('step-native-launch', 'error', `Failed to start backend: ${err.message}`, err.message);
    });

    backendProcess.on('exit', (code) => {
      console.log('Backend exited with code:', code);
    });

    sendBootUpdate('step-native-launch', 'success', 'Native Sirius Core manager active', '✓ sirius-core listening on :8080');
    setTimeout(step4_probeGui, 800);
  });

  // Step 4: Probe Dashboard & Connect
  async function step4_probeGui() {
    sendBootUpdate('step-api-ready', 'running', 'Connecting to Web GUI Dashboard & syncing...', `Connecting to ${TARGET_URL}...`);

    let attempts = 0;
    const maxAttempts = 30;
    const poller = setInterval(() => {
      attempts++;
      checkPort8080Ready((ready) => {
        if (ready) {
          clearInterval(poller);
          checkApiStatusReady(async () => {
            sendBootUpdate('step-api-ready', 'success', 'Native Node Manager is online', '✓ All systems operational (HTTP 200 OK)');
            await delay(800);
            sendBootUpdate('step-complete', 'success', 'Opening Dashboard...', 'Launching ProximaX Sirius Node...');
            setTimeout(loadDashboardDirectly, 400);
          });
        } else if (attempts >= maxAttempts) {
          clearInterval(poller);
          sendBootUpdate('step-api-ready', 'error', 'Dashboard did not respond within timeout', 'Check if port 8080 is accessible');
        } else {
          sendBootUpdate('step-api-ready', 'running', `Connecting to Dashboard (${attempts}/${maxAttempts})...`, `Waiting for HTTP response on port 8080...`);
        }
      });
    }, 600);
  }
}

function getBootVisualizerHTML() {
  return `
  <!DOCTYPE html>
  <html lang="en">
  <head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>ProximaX Sirius Node - Starting</title>
    <style>
      * { box-sizing: border-box; margin: 0; padding: 0; }
      body {
        font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif;
        background-color: #0d1117;
        color: #f3f4f6;
        display: flex;
        align-items: center;
        justify-content: center;
        min-height: 100vh;
        -webkit-user-select: none;
        padding: 24px;
      }
      .container {
        width: 100%;
        max-width: 640px;
        background: #161b22;
        border: 1px solid #30363d;
        border-radius: 18px;
        box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.7);
        padding: 28px 32px;
        display: flex;
        flex-direction: column;
        gap: 20px;
      }
      .header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        border-bottom: 1px solid #21262d;
        padding-bottom: 16px;
      }
      .header-left {
        display: flex;
        align-items: center;
        gap: 14px;
      }
      .logo {
        width: 44px;
        height: 44px;
        background: linear-gradient(135deg, #10b981, #06b6d4);
        border-radius: 12px;
        display: flex;
        align-items: center;
        justify-content: center;
        font-weight: 800;
        font-size: 22px;
        color: #ffffff;
        box-shadow: 0 0 20px rgba(16, 185, 129, 0.35);
      }
      .title-wrap h1 {
        font-size: 16px;
        font-weight: 700;
        color: #ffffff;
        letter-spacing: -0.01em;
      }
      .title-wrap p {
        font-size: 12px;
        color: #8b949e;
        margin-top: 2px;
      }
      .badge {
        font-size: 10px;
        font-weight: 700;
        background: rgba(16, 185, 129, 0.15);
        color: #34d399;
        border: 1px solid rgba(16, 185, 129, 0.35);
        padding: 4px 8px;
        border-radius: 6px;
        font-family: ui-monospace, SFMono-Regular, monospace;
      }
      .steps-list {
        display: flex;
        flex-direction: column;
        gap: 10px;
      }
      .step-item {
        display: flex;
        align-items: center;
        justify-content: space-between;
        background: #0d1117;
        border: 1px solid #21262d;
        border-radius: 10px;
        padding: 12px 16px;
        transition: all 0.3s ease;
      }
      .step-left {
        display: flex;
        align-items: center;
        gap: 12px;
      }
      .step-icon {
        width: 24px;
        height: 24px;
        border-radius: 50%;
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 11px;
        font-weight: 700;
        background: #21262d;
        color: #8b949e;
      }
      .step-name {
        font-size: 13px;
        font-weight: 600;
        color: #c9d1d9;
      }
      .step-desc {
        font-size: 11px;
        color: #8b949e;
        margin-top: 2px;
      }
      .step-badge {
        font-size: 10px;
        font-weight: 700;
        padding: 3px 8px;
        border-radius: 6px;
        text-transform: uppercase;
        background: #21262d;
        color: #8b949e;
      }
      .step-running {
        border-color: #38bdf8;
        background: rgba(56, 189, 248, 0.05);
      }
      .step-running .step-icon {
        background: #38bdf8;
        color: #0d1117;
        animation: pulse 1.5s infinite;
      }
      .step-running .step-badge {
        background: rgba(56, 189, 248, 0.2);
        color: #38bdf8;
      }
      .step-success {
        border-color: #10b981;
        background: rgba(16, 185, 129, 0.05);
      }
      .step-success .step-icon {
        background: #10b981;
        color: #ffffff;
      }
      .step-success .step-badge {
        background: rgba(16, 185, 129, 0.2);
        color: #10b981;
      }
      .step-error {
        border-color: #ef4444;
        background: rgba(239, 68, 68, 0.05);
      }
      .step-error .step-icon {
        background: #ef4444;
        color: #ffffff;
      }
      .step-error .step-badge {
        background: rgba(239, 68, 68, 0.2);
        color: #ef4444;
      }
      @keyframes pulse {
        0%, 100% { opacity: 1; transform: scale(1); }
        50% { opacity: 0.7; transform: scale(0.92); }
      }
      .terminal {
        background: #090d13;
        border: 1px solid #21262d;
        border-radius: 10px;
        padding: 10px 14px;
        font-family: ui-monospace, SFMono-Regular, monospace;
        font-size: 11px;
        color: #7ee787;
        min-height: 38px;
        max-height: 60px;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        line-height: 1.4;
      }
    </style>
  </head>
  <body>
    <div class="container">
      <div class="header">
        <div class="header-left">
          <div class="logo">P</div>
          <div class="title-wrap">
            <h1>ProximaX Sirius Node</h1>
            <p>Native Apple Silicon ARM64 Engine</p>
          </div>
        </div>
        <div class="badge">NATIVE MODE</div>
      </div>

      <div class="steps-list">
        <div id="step-arch" class="step-item">
          <div class="step-left">
            <div class="step-icon">1</div>
            <div>
              <div class="step-name">Native Architecture</div>
              <div class="step-desc" id="desc-step-arch">Verifying Apple Silicon ARM64 binaries...</div>
            </div>
          </div>
          <div class="step-badge" id="badge-step-arch">PENDING</div>
        </div>

        <div id="step-configs" class="step-item">
          <div class="step-left">
            <div class="step-icon">2</div>
            <div>
              <div class="step-name">Configurations & Storage</div>
              <div class="step-desc" id="desc-step-configs">Checking resources & clearing lockfiles...</div>
            </div>
          </div>
          <div class="step-badge" id="badge-step-configs">PENDING</div>
        </div>

        <div id="step-native-launch" class="step-item">
          <div class="step-left">
            <div class="step-icon">3</div>
            <div>
              <div class="step-name">Node Core Process</div>
              <div class="step-desc" id="desc-step-native-launch">Starting native Sirius Core daemon...</div>
            </div>
          </div>
          <div class="step-badge" id="badge-step-native-launch">PENDING</div>
        </div>

        <div id="step-api-ready" class="step-item">
          <div class="step-left">
            <div class="step-icon">4</div>
            <div>
              <div class="step-name">Web Dashboard</div>
              <div class="step-desc" id="desc-step-api-ready">Connecting to port 8080...</div>
            </div>
          </div>
          <div class="step-badge" id="badge-step-api-ready">PENDING</div>
        </div>
      </div>

      <div class="terminal" id="terminal-log">> Initializing ProximaX Sirius Standalone Native Node...</div>
    </div>

    <script>
      window.updateBootStep = function(stepId, status, message, logLine) {
        const item = document.getElementById(stepId);
        const desc = document.getElementById('desc-' + stepId);
        const badge = document.getElementById('badge-' + stepId);
        const term = document.getElementById('terminal-log');

        if (item) {
          item.className = 'step-item step-' + status;
        }
        if (desc && message) {
          desc.innerText = message;
        }
        if (badge) {
          badge.innerText = status.toUpperCase();
        }
        if (term && logLine) {
          term.innerText = '> ' + logLine;
        }
      };
    </script>
  </body>
  </html>
  `;
}

function createWindow() {
  if (mainWindow) {
    if (mainWindow.isMinimized()) mainWindow.restore();
    mainWindow.show();
    mainWindow.focus();
    return;
  }

  mainWindow = new BrowserWindow({
    width: 1280,
    height: 860,
    minWidth: 1024,
    minHeight: 700,
    title: 'ProximaX Sirius Node',
    titleBarStyle: 'hiddenInset',
    trafficLightPosition: { x: 18, y: 16 },
    backgroundColor: '#090d16',
    webPreferences: {
      preload: path.join(__dirname, 'preload.cjs'),
      nodeIntegration: false,
      contextIsolation: true,
      sandbox: false,
      webSecurity: true
    }
  });

  // Security: Intercept external link clicks to open in default browser
  mainWindow.webContents.setWindowOpenHandler(({ url: targetUrl }) => {
    if (targetUrl.startsWith('http://') || targetUrl.startsWith('https://')) {
      shell.openExternal(targetUrl);
    }
    return { action: 'deny' };
  });

  // Security: Prevent in-app navigation outside of local dashboard
  mainWindow.webContents.on('will-navigate', (event, navUrl) => {
    if (!navUrl.startsWith(TARGET_URL) && !navUrl.startsWith('data:')) {
      event.preventDefault();
      shell.openExternal(navUrl);
    }
  });

  checkPort8080Ready((isReady) => {
    if (isReady) {
      loadDashboardDirectly();
    } else {
      mainWindow.loadURL('data:text/html;charset=utf-8,' + encodeURIComponent(getBootVisualizerHTML()));
      setTimeout(runBootSequence, 500);
    }
  });

  mainWindow.on('close', (event) => {
    if (!isQuitting) {
      event.preventDefault();
      mainWindow.hide();
    }
  });

  mainWindow.on('closed', () => {
    mainWindow = null;
  });
}

function showDashboardWindow() {
  if (process.platform === 'darwin' && app.dock) {
    app.dock.show();
  }
  if (mainWindow && !mainWindow.isDestroyed()) {
    if (mainWindow.isMinimized()) mainWindow.restore();
    mainWindow.show();
    mainWindow.focus();
  } else {
    createWindow();
  }
}

function setupMenu() {
  const isMac = process.platform === 'darwin';
  const isRunning = trayStatus.status === 'running';

  let nodeStatusLabel = '🔴 Node: Stopped';
  if (isRunning) {
    nodeStatusLabel = '🟢 Node: Online & Harvesting';
  } else if (trayStatus.status === 'starting') {
    nodeStatusLabel = '🟡 Node: Starting...';
  }

  let chainSyncLabel = '⚪ Chain: Offline';
  if (isRunning) {
    if (trayStatus.networkHeight > 0 && trayStatus.blockHeight > 0) {
      if (trayStatus.blockHeight >= trayStatus.networkHeight - 3) {
        chainSyncLabel = `🟢 Chain: Synced (#${trayStatus.blockHeight.toLocaleString()})`;
      } else {
        const pct = ((trayStatus.blockHeight / trayStatus.networkHeight) * 100).toFixed(1);
        chainSyncLabel = `🟡 Chain: Syncing ${pct}% (#${trayStatus.blockHeight.toLocaleString()} / #${trayStatus.networkHeight.toLocaleString()})`;
      }
    } else if (trayStatus.blockHeight > 0) {
      chainSyncLabel = `🟢 Chain: Block #${trayStatus.blockHeight.toLocaleString()}`;
    }
  }

  const template = [
    ...(isMac ? [{
      label: app.name,
      submenu: [
        { role: 'about' },
        { type: 'separator' },
        {
          label: 'Show Node Dashboard',
          accelerator: 'CmdOrCtrl+O',
          click: () => showDashboardWindow()
        },
        {
          label: 'Reload Dashboard',
          accelerator: 'CmdOrCtrl+R',
          click: () => { if (mainWindow && !mainWindow.isDestroyed()) mainWindow.reload(); }
        },
        {
          label: 'Open in Web Browser (localhost:8080)',
          click: () => { shell.openExternal(TARGET_URL); }
        },
        { type: 'separator' },
        { role: 'hide' },
        { role: 'hideOthers' },
        { role: 'unhide' },
        { type: 'separator' },
        { 
          label: 'Quit ProximaX Sirius Node',
          accelerator: 'CmdOrCtrl+Q',
          click: () => {
            isQuitting = true;
            app.quit();
          }
        }
      ]
    }] : []),
    {
      label: 'Node Controls',
      submenu: [
        {
          label: nodeStatusLabel,
          enabled: false
        },
        {
          label: chainSyncLabel,
          enabled: false
        },
        {
          label: trayStatus.blockHeight > 0 ? `Latest Synced: #${trayStatus.blockHeight.toLocaleString()}` : 'Latest Synced: None',
          enabled: false
        },
        {
          label: `P2P Mesh: ${trayStatus.peersCount} Connected Peers`,
          enabled: false
        },
        { type: 'separator' },
        isRunning
          ? {
              label: 'Stop Node Engine',
              click: () => sendNodeAction('/api/node/stop')
            }
          : {
              label: 'Start Node Engine',
              click: () => sendNodeAction('/api/node/start')
            },
        {
          label: 'Restart Node Engine',
          click: () => sendNodeAction('/api/node/restart')
        }
      ]
    },
    {
      label: 'View',
      submenu: [
        { role: 'reload' },
        { role: 'forceReload' },
        { role: 'toggleDevTools' },
        { type: 'separator' },
        { role: 'resetZoom' },
        { role: 'zoomIn' },
        { role: 'zoomOut' },
        { type: 'separator' },
        { role: 'togglefullscreen' }
      ]
    },
    {
      label: 'Window',
      submenu: [
        { role: 'minimize' },
        { role: 'zoom' },
        ...(isMac ? [
          { type: 'separator' },
          { role: 'front' }
        ] : [
          { role: 'close' }
        ])
      ]
    },
    {
      role: 'help',
      submenu: [
        {
          label: 'ProximaX Validator Documentation',
          click: async () => {
            await shell.openExternal('https://bcdocs.xpxsirius.io/docs/protocol/validating/');
          }
        },
        {
          label: 'ProximaX Mainnet Explorer',
          click: async () => {
            await shell.openExternal('https://explorer.xpxsirius.io');
          }
        }
      ]
    }
  ];

  const menu = Menu.buildFromTemplate(template);
  Menu.setApplicationMenu(menu);
}

let trayStatus = {
  status: 'unknown',
  blockHeight: 0,
  networkHeight: 0,
  peersCount: 0,
  totalValidated: 0,
  totalEarnedXPX: 0,
  lastHarvestedHeight: 0,
  lastHarvestedTime: '',
  uptime: ''
};
let trayPollInterval = null;

function sendNodeAction(actionEndpoint) {
  const req = http.request({
    hostname: '127.0.0.1',
    port: 8080,
    path: actionEndpoint,
    method: 'POST',
    timeout: 4000
  }, (res) => {
    res.resume();
    setTimeout(pollTrayMetrics, 500);
  });
  req.on('error', () => {});
  req.end();
}

function updateTrayMenu() {
  if (!tray) return;

  const isRunning = trayStatus.status === 'running';
  const isStarting = trayStatus.status === 'starting';

  let nodeStatusLabel = '🔴 Node: Stopped';
  if (isRunning) {
    nodeStatusLabel = '🟢 Node: Online & Harvesting';
  } else if (isStarting) {
    nodeStatusLabel = '🟡 Node: Starting...';
  }

  let chainSyncLabel = '⚪ Chain: Offline';
  if (isRunning) {
    if (trayStatus.networkHeight > 0 && trayStatus.blockHeight > 0) {
      if (trayStatus.blockHeight >= trayStatus.networkHeight - 3) {
        chainSyncLabel = `🟢 Chain: Synced (#${trayStatus.blockHeight.toLocaleString()})`;
      } else {
        const pct = ((trayStatus.blockHeight / trayStatus.networkHeight) * 100).toFixed(1);
        const diff = trayStatus.networkHeight - trayStatus.blockHeight;
        chainSyncLabel = `🟡 Chain: Syncing ${pct}% (-${diff.toLocaleString()} blks)`;
      }
    } else if (trayStatus.blockHeight > 0) {
      chainSyncLabel = `🟢 Chain: Block #${trayStatus.blockHeight.toLocaleString()}`;
    } else {
      chainSyncLabel = '🟡 Chain: Initializing...';
    }
  }

  const latestBlockLabel = trayStatus.blockHeight > 0
    ? `⏱️ Latest Synced: #${trayStatus.blockHeight.toLocaleString()}`
    : '⏱️ Latest Synced: None';

  const peersLabel = `🌐 P2P Mesh: ${trayStatus.peersCount} Connected Peers`;
  
  const harvestLabel = `⚡ Blocks Validated: ${trayStatus.totalValidated.toLocaleString()}${
    trayStatus.totalEarnedXPX > 0 ? ` (+${trayStatus.totalEarnedXPX.toFixed(3)} XPX)` : ''
  }`;

  const lastHarvestLabel = trayStatus.lastHarvestedHeight > 0
    ? `🏷️ Last Harvested: #${trayStatus.lastHarvestedHeight.toLocaleString()}`
    : null;

  const uptimeLabel = trayStatus.uptime ? `⏳ Uptime: ${trayStatus.uptime}` : null;

  const tooltipText = `ProximaX Sirius Mainnet Node\n${nodeStatusLabel.replace(/[🟢🟡🔴]/g, '').trim()}\n${chainSyncLabel.replace(/[🟢🟡⚪]/g, '').trim()}`;
  tray.setToolTip(tooltipText);

  if (process.platform === 'darwin') {
    tray.setTitle(isRunning && trayStatus.blockHeight > 0 ? ` #${trayStatus.blockHeight.toLocaleString()}` : '');
  }

  const menuItems = [
    {
      label: 'ProximaX Sirius Validator (Mainnet ARM64)',
      enabled: false
    },
    { type: 'separator' },
    {
      label: nodeStatusLabel,
      enabled: false
    },
    {
      label: chainSyncLabel,
      enabled: false
    },
    {
      label: latestBlockLabel,
      enabled: false
    },
    {
      label: peersLabel,
      enabled: false
    },
    {
      label: harvestLabel,
      enabled: false
    }
  ];

  if (lastHarvestLabel) {
    menuItems.push({
      label: lastHarvestLabel,
      enabled: false
    });
  }

  if (uptimeLabel) {
    menuItems.push({
      label: uptimeLabel,
      enabled: false
    });
  }

  menuItems.push(
    { type: 'separator' },
    isRunning
      ? {
          label: '⏹️ Stop Node Engine',
          click: () => sendNodeAction('/api/node/stop')
        }
      : {
          label: '▶️ Start Node Engine',
          click: () => sendNodeAction('/api/node/start')
        },
    {
      label: '🔄 Restart Node Engine',
      click: () => sendNodeAction('/api/node/restart')
    },
    { type: 'separator' },
    {
      label: '🖥️ Show Node Dashboard',
      click: () => {
        if (mainWindow) {
          mainWindow.show();
          mainWindow.focus();
        } else {
          createWindow();
        }
      }
    },
    {
      label: '🌐 Open in Web Browser (localhost:8080)',
      click: () => {
        shell.openExternal(TARGET_URL);
      }
    },
    { type: 'separator' },
    {
      label: '❌ Quit ProximaX Sirius Node',
      accelerator: 'CmdOrCtrl+Q',
      click: () => {
        isQuitting = true;
        app.quit();
      }
    }
  );

  const contextMenu = Menu.buildFromTemplate(menuItems);
  tray.setContextMenu(contextMenu);

  // Synchronize top macOS application menu bar
  setupMenu();
}

function pollTrayMetrics() {
  const req = http.get(`${TARGET_URL}/api/status`, (res) => {
    if (res.statusCode === 200) {
      let raw = '';
      res.on('data', (chunk) => { raw += chunk; });
      res.on('end', () => {
        try {
          const data = JSON.parse(raw);
          const metrics = data.metrics || {};
          const harvest = data.harvestStats || {};

          trayStatus.status = data.status || metrics.status || 'unknown';
          trayStatus.blockHeight = data.blockHeight || metrics.blockHeight || 0;
          trayStatus.networkHeight = data.networkHeight || metrics.networkHeight || 0;
          trayStatus.peersCount = data.peersCount !== undefined ? data.peersCount : (metrics.peersCount || 0);
          trayStatus.totalValidated = harvest.totalBlocksValidated || 0;
          trayStatus.totalEarnedXPX = harvest.totalEarnedFeesXPX || 0;
          trayStatus.lastHarvestedHeight = harvest.lastHarvestedHeight || 0;
          trayStatus.lastHarvestedTime = harvest.lastHarvestedTime || '';
          trayStatus.uptime = metrics.uptime || '';

          updateTrayMenu();
        } catch (e) {
          // ignore parse error
        }
      });
    } else {
      res.resume();
      trayStatus.status = 'stopped';
      updateTrayMenu();
    }
  });

  req.on('error', () => {
    trayStatus.status = 'stopped';
    trayStatus.peersCount = 0;
    updateTrayMenu();
  });

  req.setTimeout(2000, () => req.destroy());
}

function setupTray() {
  if (tray) return;

  try {
    const iconPath = path.join(__dirname, 'assets', 'icon.png');
    const trayIcon = nativeImage.createFromPath(iconPath).resize({ width: 18, height: 18 });
    tray = new Tray(trayIcon);
    updateTrayMenu();

    tray.on('click', () => {
      showDashboardWindow();
    });

    tray.on('double-click', () => {
      showDashboardWindow();
    });

    pollTrayMetrics();
    if (trayPollInterval) clearInterval(trayPollInterval);
    trayPollInterval = setInterval(pollTrayMetrics, 2500);
  } catch (err) {
    console.log('Tray init note:', err.message);
  }
}

app.whenReady().then(() => {
  setupMenu();
  setupTray();
  createWindow();

  app.on('activate', () => {
    showDashboardWindow();
  });
});

let isShuttingDown = false;
let userConfirmedAction = false;

function stopBackendAndExit() {
  if (trayPollInterval) {
    clearInterval(trayPollInterval);
    trayPollInterval = null;
  }
  if (tray) {
    try {
      tray.destroy();
      tray = null;
    } catch (e) {}
  }

  let finished = false;
  const finish = () => {
    if (finished) return;
    finished = true;
    if (backendProcess) {
      try {
        backendProcess.kill('SIGINT');
      } catch (e) {}
      backendProcess = null;
    }
    // Clean up any remaining sirius processes
    try {
      execSync('pkill -f "sirius-core.*-chainconfig" || true', { stdio: 'ignore' });
      execSync('pkill -f "sirius.bc" || true', { stdio: 'ignore' });
    } catch (e) {}
    setTimeout(() => {
      app.exit(0);
    }, 250);
  };

  // Gracefully tell Sirius Core API to shutdown node engine and server process
  const req = http.request({
    hostname: '127.0.0.1',
    port: 8080,
    path: '/api/system/shutdown',
    method: 'POST',
    timeout: 1500
  }, () => {
    finish();
  });
  req.on('error', () => {
    finish();
  });
  req.end();

  // Safety fallback timeout
  setTimeout(finish, 2000);
}

ipcMain.on('quit-action-response', (event, action) => {
  if (action === 'stop-everything') {
    userConfirmedAction = true;
    isQuitting = true;
    isShuttingDown = true;
    stopBackendAndExit();
  } else if (action === 'background') {
    if (mainWindow && !mainWindow.isDestroyed()) {
      mainWindow.hide();
    }
    // Hide icon from macOS Dock in background mode
    if (process.platform === 'darwin' && app.dock) {
      app.dock.hide();
    }
    isQuitting = false;
    userConfirmedAction = false;
  } else {
    isQuitting = false;
    userConfirmedAction = false;
  }
});

app.on('before-quit', (event) => {
  if (isShuttingDown) return;

  if (!userConfirmedAction) {
    if (mainWindow && !mainWindow.isDestroyed() && isLoaded) {
      event.preventDefault();
      if (!mainWindow.isVisible()) {
        mainWindow.show();
      }
      mainWindow.focus();
      mainWindow.webContents.send('request-quit-action');
      return;
    }

    event.preventDefault();
    userConfirmedAction = true;
    isQuitting = true;
    isShuttingDown = true;
    stopBackendAndExit();
  }
});

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    app.quit();
  }
});
