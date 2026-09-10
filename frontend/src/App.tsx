import { useState, useEffect } from 'react';
import { Header } from './components/Header';
import { NavigationTabs } from './components/NavigationTabs';
import { StatusBar } from './components/StatusBar';
import { OverviewTab } from './components/OverviewTab';
import { ValidatorTab } from './components/ValidatorTab';
import { NetworkTab } from './components/NetworkTab';
import { StorageTab } from './components/StorageTab';
import { LogsTab } from './components/LogsTab';
import { ConfigTab } from './components/ConfigTab';
import { MaintenanceTab } from './components/MaintenanceTab';
import { SetupWizard } from './components/SetupWizard';
import { AboutModal } from './components/AboutModal';
import { QuitConfirmModal } from './components/QuitConfirmModal';
import { EngineUpdateBanner } from './components/EngineUpdateBanner';
import { EngineUpdateModal } from './components/EngineUpdateModal';
import { NodeMetrics, NodeConfig, HarvestStats, StorageStatus, PortCheckResult, NetworkValidatorStats, EngineUpdateStatus } from './types';

export function App() {
  const [activeTab, setActiveTab] = useState(() => {
    const param = new URLSearchParams(window.location.search).get('tab');
    if (param) return param;
    const hash = window.location.hash.replace('#', '');
    if (hash === 'storage' || hash.startsWith('storage-')) return 'validator';
    if (hash === 'snapshots' || hash === 'settings') return 'config';
    if (hash) return hash;
    return 'overview';
  });

  useEffect(() => {
    if (activeTab === 'validator' && (window.location.hash === '#storage' || window.location.hash.startsWith('#storage-'))) {
      return;
    }
    if (window.location.hash !== `#${activeTab}`) {
      window.history.replaceState(null, '', `#${activeTab}`);
    }
  }, [activeTab]);
  const [darkMode, setDarkMode] = useState(true);
  const [metrics, setMetrics] = useState<NodeMetrics | null>(null);
  const [config, setConfig] = useState<NodeConfig | null>(() => {
    try {
      const cached = localStorage.getItem('sirius_node_config');
      if (cached) return JSON.parse(cached);
    } catch {}
    return null;
  });
  const [harvestStats, setHarvestStats] = useState<HarvestStats | null>(null);
  const [networkValidatorStats, setNetworkValidatorStats] = useState<NetworkValidatorStats | null>(null);
  const [storageStatus, setStorageStatus] = useState<StorageStatus | null>(null);
  const [portCheck, setPortCheck] = useState<PortCheckResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [wizardOpen, setWizardOpen] = useState(false);
  const [aboutOpen, setAboutOpen] = useState(false);
  const [quitModalOpen, setQuitModalOpen] = useState(false);

  // Sync dark mode class
  useEffect(() => {
    if (darkMode) {
      document.documentElement.classList.add('dark');
    } else {
      document.documentElement.classList.remove('dark');
    }
  }, [darkMode]);

  // Periodic status poll
  const [updateInfo, setUpdateInfo] = useState<any>(null);
  const [engineStatus, setEngineStatus] = useState<EngineUpdateStatus | null>(() => {
    const testView = new URLSearchParams(window.location.search).get('test_view');
    if (testView === 'banner' || testView === 'modal') {
      return {
        currentVersion: 'v1.9.7',
        targetVersion: 'v1.9.8',
        hasUpdate: true,
        releaseNotes: 'Engine consensus performance enhancements, RocksDB memory cache optimization, and P2P fast-sync resilience improvements.',
        releaseUrl: 'https://github.com/igorgoc/cpp-xpx-chain/releases/tag/v1.9.8',
        isApplying: false,
        state: 'idle',
        rollbackOccurred: false,
      };
    }
    if (testView === 'progress') {
      return {
        currentVersion: 'v1.9.7',
        targetVersion: 'v1.9.8',
        hasUpdate: true,
        releaseNotes: 'Engine consensus performance enhancements, RocksDB memory cache optimization, and P2P fast-sync resilience improvements.',
        isApplying: true,
        state: 'swapping',
        message: 'Stopping engine gracefully, creating sirius.bc.bak backup, and performing atomic binary swap...',
        rollbackOccurred: false,
      };
    }
    if (testView === 'initial_setup') {
      return {
        currentVersion: 'none',
        targetVersion: 'v1.9.8',
        hasUpdate: true,
        isInstalled: false,
        isInitialSetup: true,
        releaseNotes: 'Official Sirius Engine v1.9.8 release with native consensus performance and P2P fast-sync improvements.',
        releaseUrl: 'https://github.com/igorgoc/cpp-xpx-chain/releases/tag/v1.9.8',
        isApplying: true,
        state: 'verifying',
        message: 'Downloading Sirius Engine v1.9.8 (verified, signed)...',
        rollbackOccurred: false,
      };
    }
    if (testView === 'initial_setup_failed') {
      return {
        currentVersion: 'none',
        targetVersion: 'v1.9.8',
        hasUpdate: true,
        isInstalled: false,
        isInitialSetup: true,
        releaseNotes: 'Official Sirius Engine v1.9.8 release with native consensus performance and P2P fast-sync improvements.',
        releaseUrl: 'https://github.com/igorgoc/cpp-xpx-chain/releases/tag/v1.9.8',
        isApplying: false,
        state: 'failed',
        message: 'Internet connection required for initial engine setup: dial tcp: lookup api.github.com: no such host',
        rollbackOccurred: false,
      };
    }
    if (testView === 'rollback') {
      return {
        currentVersion: 'v1.9.7',
        targetVersion: 'v1.9.8',
        hasUpdate: true,
        isInstalled: true,
        releaseNotes: 'Engine consensus performance enhancements, RocksDB memory cache optimization, and P2P fast-sync resilience improvements.',
        isApplying: false,
        state: 'rolled_back',
        rollbackOccurred: true,
        message: 'Post-update healthcheck failed: process crashed on boot (SIGSEGV). Automated rollback restored previous binary (v1.9.7) and restarted the node cleanly.',
      };
    }
    return null;
  });
  const [engineModalOpen, setEngineModalOpen] = useState(() => {
    const testView = new URLSearchParams(window.location.search).get('test_view');
    return testView === 'modal' || testView === 'progress' || testView === 'rollback' || testView === 'initial_setup' || testView === 'initial_setup_failed';
  });
  const [autoRecovery, setAutoRecovery] = useState(true);

  // Global event listener to open engine update modal from anywhere
  useEffect(() => {
    const handler = () => setEngineModalOpen(true);
    window.addEventListener('open-engine-updater', handler);
    return () => window.removeEventListener('open-engine-updater', handler);
  }, []);

  const fetchStatus = async () => {
    try {
      const res = await fetch('/api/status');
      if (res.ok) {
        const data = await res.json();
        setMetrics(data.metrics || null);
        setHarvestStats(data.harvestStats || null);
        if (data.networkValidatorStats) setNetworkValidatorStats(data.networkValidatorStats);
        if (data.storageStatus) setStorageStatus(data.storageStatus);
        if (data.portCheck) setPortCheck(data.portCheck);
        if (data.updateInfo) setUpdateInfo(data.updateInfo);
        if (data.engineStatus) {
          setEngineStatus(prev => {
            const testView = new URLSearchParams(window.location.search).get('test_view');
            if (testView) return prev;
            return data.engineStatus;
          });

          // Auto open engine setup modal on first load if engine is not installed or currently applying
          if ((!data.engineStatus.isInstalled || data.engineStatus.isApplying) && !sessionStorage.getItem('engine_setup_dismissed')) {
            setEngineModalOpen(true);
          }
        }
        if (typeof data.autoRecovery === 'boolean') setAutoRecovery(data.autoRecovery);

        setConfig((prev) => {
          if (!prev || JSON.stringify(prev) !== JSON.stringify(data.config)) {
            if (data.config) {
              try { localStorage.setItem('sirius_node_config', JSON.stringify(data.config)); } catch {}
            }
            return data.config || null;
          }
          return prev;
        });

        // Auto open setup wizard if not configured yet on first load
        if (data.config && !data.config.isConfigured && !sessionStorage.getItem('wizard_dismissed')) {
          setWizardOpen(true);
          sessionStorage.setItem('wizard_dismissed', 'true');
        }
      }
    } catch (e) {
      console.warn('Polling status error:', e);
    }
  };

  const fetchConfig = async () => {
    try {
      const res = await fetch(`/api/config?_t=${Date.now()}`);
      if (res.ok) {
        const data = await res.json();
        if (data && typeof data === 'object') {
          setConfig((prev) => {
            if (!prev || JSON.stringify(prev) !== JSON.stringify(data)) {
              return data;
            }
            return prev;
          });
        }
      }
    } catch (e) {
      console.warn('Config fetch error:', e);
    }
  };

  useEffect(() => {
    let intervalId: any = null;

    const getPollingInterval = () => {
      if (document.hidden) {
        return 30000; // 30s when browser tab is in background
      }
      if (activeTab === 'overview') {
        return 4000; // 4s active on main dashboard
      }
      return 10000; // 10s on static tabs (config, storage, maintenance, logs)
    };

    const restartPolling = () => {
      if (intervalId) clearInterval(intervalId);
      intervalId = setInterval(fetchStatus, getPollingInterval());
    };

    fetchConfig();
    fetchStatus();
    restartPolling();

    const handleVisibilityChange = () => {
      if (!document.hidden) {
        fetchStatus();
      }
      restartPolling();
    };

    document.addEventListener('visibilitychange', handleVisibilityChange);
    return () => {
      if (intervalId) clearInterval(intervalId);
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    };
  }, [activeTab]);

  // Listen for Electron quit request
  useEffect(() => {
    const electronAPI = (window as any).electronAPI;
    if (electronAPI && electronAPI.onRequestQuit) {
      electronAPI.onRequestQuit(() => {
        setQuitModalOpen(true);
      });
    }
  }, []);

  const handleStopEverything = async () => {
    const electronAPI = (window as any).electronAPI;
    if (electronAPI && electronAPI.sendQuitResponse) {
      electronAPI.sendQuitResponse('stop-everything');
    } else {
      await fetch('/api/system/shutdown', { method: 'POST' });
      setQuitModalOpen(false);
    }
  };

  const handleKeepBackground = () => {
    const electronAPI = (window as any).electronAPI;
    if (electronAPI && electronAPI.sendQuitResponse) {
      electronAPI.sendQuitResponse('background');
    }
    setQuitModalOpen(false);
  };

  const handleCancelQuit = () => {
    const electronAPI = (window as any).electronAPI;
    if (electronAPI && electronAPI.sendQuitResponse) {
      electronAPI.sendQuitResponse('cancel');
    }
    setQuitModalOpen(false);
  };

  const handleToggleAutoRecovery = async (enabled: boolean) => {
    try {
      const res = await fetch('/api/node/watchdog/toggle', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled }),
      });
      if (res.ok) {
        setAutoRecovery(enabled);
      }
    } catch (e) {
      console.error(e);
    }
  };

  const handleStartNode = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/node/start', { method: 'POST' });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to start node');
      await fetchStatus();
    } catch (e: any) {
      alert(e.message);
    } finally {
      setLoading(false);
    }
  };

  const handleStopNode = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/node/stop', { method: 'POST' });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to stop node');
      await fetchStatus();
    } catch (e: any) {
      alert(e.message);
    } finally {
      setLoading(false);
    }
  };

  const handleRestartNode = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/node/restart', { method: 'POST' });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to restart node');
      await fetchStatus();
    } catch (e: any) {
      alert(e.message);
    } finally {
      setLoading(false);
    }
  };

  const handleWizardFinished = (startImmediately: boolean) => {
    setWizardOpen(false);
    fetchStatus();
    if (startImmediately) {
      handleStartNode();
    }
  };

  return (
    <div className="flex flex-col h-screen overflow-hidden bg-[#090d16] text-slate-100 font-sans">
      {/* Top Operator Top Bar */}
      <Header
        metrics={metrics}
        config={config}
        darkMode={darkMode}
        setDarkMode={setDarkMode}
        onStart={handleStartNode}
        onStop={handleStopNode}
        onRestart={handleRestartNode}
        onOpenWizard={() => setWizardOpen(true)}
        onOpenAbout={() => setAboutOpen(true)}
        setActiveTab={setActiveTab}
        loading={loading}
      />

      {/* Sirius Engine Update Available Banner */}
      <EngineUpdateBanner
        engineStatus={engineStatus}
        onOpenModal={() => setEngineModalOpen(true)}
      />

      {/* Tabs navigation */}
      <NavigationTabs activeTab={activeTab} setActiveTab={setActiveTab} />

      {/* Main Operator Content Viewport */}
      <main className="flex-1 bg-[#0F1115] overflow-y-auto">
        {activeTab === 'overview' && (
          <OverviewTab
            metrics={metrics}
            config={config}
            harvestStats={harvestStats}
            networkValidatorStats={networkValidatorStats}
            storageStatus={storageStatus}
            portCheck={portCheck}
            updateInfo={updateInfo}
            engineStatus={engineStatus}
            autoRecovery={autoRecovery}
            onStart={handleStartNode}
            onStop={handleStopNode}
            onRestart={handleRestartNode}
            onOpenWizard={() => setWizardOpen(true)}
            setActiveTab={setActiveTab}
            loading={loading}
            onRefresh={fetchStatus}
            onToggleAutoRecovery={handleToggleAutoRecovery}
          />
        )}

        {activeTab === 'validator' && (
          <ValidatorTab
            metrics={metrics}
            config={config}
            harvestStats={harvestStats}
            networkValidatorStats={networkValidatorStats}
            storageStatus={storageStatus}
            portCheck={portCheck}
            loading={loading}
            onOpenSettings={() => setActiveTab('config')}
            onStartNode={handleStartNode}
            onRefresh={fetchStatus}
          />
        )}

        {activeTab === 'network' && (
          <NetworkTab
            metrics={metrics}
            config={config}
            storageStatus={storageStatus}
            portCheck={portCheck}
            onRefresh={fetchStatus}
            loading={loading}
          />
        )}

        {activeTab === 'config' && (
          <ConfigTab config={config} harvestStats={harvestStats} metrics={metrics} onRefreshConfig={fetchStatus} />
        )}

        {activeTab === 'storage' && (
          <StorageTab
            storageStatus={storageStatus}
            portCheck={portCheck}
            onRefresh={fetchStatus}
            loading={loading}
          />
        )}

        {activeTab === 'maintenance' && (
          <MaintenanceTab
            onOpenSettings={(subtab = 'snapshots') => {
              window.location.hash = `#${subtab}`;
              setActiveTab('config');
            }}
          />
        )}

        {activeTab === 'logs' && <LogsTab />}
      </main>

      {/* High-density operator telemetry status bar */}
      <StatusBar
        metrics={metrics}
        config={config}
        onOpenWizard={() => setWizardOpen(true)}
        onOpenAbout={() => setAboutOpen(true)}
      />

      {/* First-run / Configuration Wizard Modal */}
      <SetupWizard
        isOpen={wizardOpen}
        onClose={() => setWizardOpen(false)}
        config={config}
        onFinished={handleWizardFinished}
      />

      {/* About Modal */}
      <AboutModal
        isOpen={aboutOpen}
        onClose={() => setAboutOpen(false)}
      />

      {/* Modern Cyber-Operator Quit Confirm Modal */}
      <QuitConfirmModal
        isOpen={quitModalOpen}
        onClose={handleCancelQuit}
        onStopEverything={handleStopEverything}
        onKeepBackground={handleKeepBackground}
      />

      {/* Sirius Engine Update Modal */}
      <EngineUpdateModal
        isOpen={engineModalOpen}
        onClose={() => {
          setEngineModalOpen(false);
          sessionStorage.setItem('engine_setup_dismissed', 'true');
        }}
        engineStatus={engineStatus}
        onRefreshStatus={fetchStatus}
      />
    </div>
  );
};

export default App;
