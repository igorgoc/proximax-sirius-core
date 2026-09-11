import React, { useState, useEffect } from 'react';
import { Key, Zap, CheckCircle2, ChevronRight, ChevronLeft, AlertCircle, Play, Copy, Check, ExternalLink, Loader2, RefreshCw, Eye, EyeOff } from 'lucide-react';
import { NodeConfig, KeyPairInfo, AccountLinkResult } from '../types';
import { getExplorerAddressUrl, getExplorerPublicKeyUrl, getExplorerTxUrl } from '../utils/explorer';
import { DirectoryDropdown } from './DirectoryDropdown';

interface SetupWizardProps {
  isOpen: boolean;
  onClose: () => void;
  config: NodeConfig | null;
  onFinished: (startImmediately: boolean) => void;
}

export const SetupWizard: React.FC<SetupWizardProps> = ({
  isOpen,
  onClose,
  config,
  onFinished,
}) => {
  const [step, setStep] = useState(1);

  // Step 1: Friendly name, Host & Data Path
  const [friendlyName, setFriendlyName] = useState(config?.friendlyName || 'mainnet-validator-01');
  const [host, setHost] = useState(config?.host || '');
  const [dataPath, setDataPath] = useState(config?.dataPath || './chainconfig/data');

  // Step 2: Boot Key
  const [bootKey, setBootKey] = useState(config?.bootKey && config.bootKey !== 'BOOTKEY_PRIVATE_KEY' ? config.bootKey : '');
  const [bootKeyPair, setBootKeyPair] = useState<KeyPairInfo | null>(null);
  const [generatingBootKey, setGeneratingBootKey] = useState(false);

  // Step 3: Harvesting
  const [harvestOption, setHarvestOption] = useState<'generate' | 'existing'>('generate');
  const [accountPrivKey, setAccountPrivKey] = useState('');
  const [apiNode, setApiNode] = useState('http://aldebaran.xpxsirius.io:3000');
  const [existingHarvestKey, setExistingHarvestKey] = useState(config?.harvestKey && config.harvestKey !== 'REMOTE_ACCOUNT_PRIVATE_KEY' ? config.harvestKey : '');
  const [showHarvestKey, setShowHarvestKey] = useState(false);
  const [generatingHarvestKey, setGeneratingHarvestKey] = useState(false);
  const [linking, setLinking] = useState(false);
  const [linkResult, setLinkResult] = useState<AccountLinkResult | null>(null);
  const [harvestError, setHarvestError] = useState<string | null>(null);

  // General state
  const [saving, setSaving] = useState(false);
  const [copiedKey, setCopiedKey] = useState<string | null>(null);
  const [detectingIp, setDetectingIp] = useState(false);
  const [loadingConfig, setLoadingConfig] = useState(!config);

  // Sync config whenever it changes or fetch it when opening without config
  useEffect(() => {
    if (config) {
      if (config.friendlyName) setFriendlyName(config.friendlyName);
      if (config.host) setHost(config.host);
      if (config.dataPath) setDataPath(config.dataPath);
      if (config.bootKey && config.bootKey !== 'BOOTKEY_PRIVATE_KEY') setBootKey(config.bootKey);
      if (config.harvestKey && config.harvestKey !== 'REMOTE_ACCOUNT_PRIVATE_KEY') setExistingHarvestKey(config.harvestKey);
      setLoadingConfig(false);
    }
  }, [config]);

  useEffect(() => {
    if (isOpen && !config) {
      setLoadingConfig(true);
      fetch(`/api/config?_t=${Date.now()}`)
        .then((res) => (res.ok ? res.json() : null))
        .then((cfg: NodeConfig | null) => {
          if (cfg) {
            if (cfg.friendlyName) setFriendlyName(cfg.friendlyName);
            if (cfg.host) setHost(cfg.host);
            if (cfg.dataPath) setDataPath(cfg.dataPath);
            if (cfg.bootKey && cfg.bootKey !== 'BOOTKEY_PRIVATE_KEY') setBootKey(cfg.bootKey);
            if (cfg.harvestKey && cfg.harvestKey !== 'REMOTE_ACCOUNT_PRIVATE_KEY') setExistingHarvestKey(cfg.harvestKey);
          }
        })
        .catch((err) => console.error('Failed to pre-fetch config for SetupWizard:', err))
        .finally(() => setLoadingConfig(false));
    }
  }, [isOpen, config]);

  const detectPublicIp = async () => {
    setDetectingIp(true);
    try {
      const res = await fetch('/api/network/public-ip');
      if (res.ok) {
        const data = await res.json();
        if (data && data.publicIp) {
          setHost(data.publicIp);
        }
      }
    } catch (e) {
      console.error(e);
    } finally {
      setDetectingIp(false);
    }
  };

  // Auto-generate a boot key on first step 2 view if empty
  const generateBootKey = async () => {
    setGeneratingBootKey(true);
    try {
      const res = await fetch('/api/keys/generate', { method: 'POST' });
      const data = await res.json();
      if (data && data.privateKey) {
        setBootKey(data.privateKey);
        setBootKeyPair(data);
      }
    } catch (e) {
      console.error(e);
    } finally {
      setGeneratingBootKey(false);
    }
  };

  const handleLinkAccount = async () => {
    if (!accountPrivKey || accountPrivKey.length !== 64) {
      setHarvestError('Please enter a valid 64-character account private key.');
      return;
    }

    setLinking(true);
    setHarvestError(null);
    try {
      const res = await fetch('/api/harvesting/link', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          accountPrivateKey: accountPrivKey.trim(),
          apiNode: apiNode.trim(),
        }),
      });

      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to announce account link');

      setLinkResult(data);
      setExistingHarvestKey(data.remotePrivateKey);
    } catch (e: any) {
      setHarvestError(e.message);
    } finally {
      setLinking(false);
    }
  };

  const generateHarvestKey = async () => {
    setGeneratingHarvestKey(true);
    try {
      const res = await fetch('/api/keys/generate', { method: 'POST' });
      const data = await res.json();
      if (data && data.privateKey) {
        setExistingHarvestKey(data.privateKey);
      }
    } catch (e) {
      console.error(e);
    } finally {
      setGeneratingHarvestKey(false);
    }
  };

  const handleFinish = async (startImmediately: boolean) => {
    setSaving(true);
    try {
      const harvestKeyVal = (existingHarvestKey.trim() || linkResult?.remotePrivateKey || '').trim();
      if (!harvestKeyVal || harvestKeyVal === 'REMOTE_ACCOUNT_PRIVATE_KEY' || harvestKeyVal.length !== 64) {
        setHarvestError('A valid 64-character hexadecimal harvest key is mandatory.');
        setStep(3);
        setSaving(false);
        return;
      }

      let finalBootKey = bootKey.trim();
      if (!finalBootKey || finalBootKey === harvestKeyVal) {
        // Automatically generate a unique transport boot key if missing or conflicting
        const genRes = await fetch('/api/keys/generate', { method: 'POST' });
        const genData = await genRes.json();
        if (genData?.privateKey) {
          finalBootKey = genData.privateKey;
        }
      }

      const payload: NodeConfig = {
        friendlyName: friendlyName.trim(),
        host: host.trim(),
        bootKey: finalBootKey,
        bootKeySource: 'generated',
        harvestKey: harvestKeyVal,
        beneficiary: config?.beneficiary || '0000000000000000000000000000000000000000000000000000000000000000',
        isAutoHarvesting: true,
        maxUnlockedAccounts: 5,
        port: 7900,
        apiPort: 7901,
        dbrbPort: 7903,
        dataDirectory: '/data',
        dataPath: dataPath.trim() || './chainconfig/data',
        isConfigured: true,
      };

      const res = await fetch('/api/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });

      if (!res.ok) {
        const errData = await res.json().catch(() => ({}));
        throw new Error(errData.error || errData.message || 'Failed to save configuration');
      }

      // Read-After-Write Verification: confirm server stored exactly what we sent
      const verifyRes = await fetch(`/api/config?_t=${Date.now()}`);
      if (verifyRes.ok) {
        const verified: NodeConfig = await verifyRes.json();
        if (payload.dataPath && verified.dataPath && verified.dataPath !== payload.dataPath) {
          throw new Error(`Blockchain data path verification failed: server stored '${verified.dataPath}', expected '${payload.dataPath}'`);
        }
      }

      onFinished(startImmediately);
    } catch (e: any) {
      alert(e.message);
    } finally {
      setSaving(false);
    }
  };

  const copyToClipboard = (text: string, id: string) => {
    navigator.clipboard.writeText(text);
    setCopiedKey(id);
    setTimeout(() => setCopiedKey(null), 2000);
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 bg-black/80 backdrop-blur-xs flex items-center justify-center p-3 sm:p-5 select-none animate-fadeIn">
      <div className="bg-[#181B20] border border-[#262B34] w-full max-w-2xl rounded-xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh] text-slate-200 my-auto">
        {/* Wizard Header */}
        <div className="bg-[#0F1115] text-white px-5 py-3.5 flex items-center justify-between border-b border-[#262B34]">
          <div className="flex items-center space-x-2.5">
            <div className="w-7 h-7 rounded-md bg-[#181B20] border border-[#262B34] flex items-center justify-center font-bold text-xs text-blue-400">
              SX
            </div>
            <div>
              <div className="flex items-center space-x-2">
                <h2 className="text-xs font-semibold uppercase tracking-wider">
                  ProximaX Sirius Node Setup Wizard
                </h2>
                <span className="text-[10px] font-mono bg-slate-800 text-slate-400 px-1.5 py-0.5 rounded border border-slate-700/50">
                  STEP {step} OF 4
                </span>
              </div>
              <p className="text-[11px] text-slate-400 font-mono">
                {step === 1 ? 'Node Identity & Data Path' : step === 2 ? 'P2P Boot Key Setup' : step === 3 ? 'POS+ Delegated Validating' : 'Summary & Deployment'}
              </p>
            </div>
          </div>

          <button
            onClick={onClose}
            className="text-slate-400 hover:text-white text-xs px-2 py-1 rounded hover:bg-[#262B34] transition-colors"
          >
            ✕
          </button>
        </div>

        {/* Step Progress Dots */}
        <div className="bg-[#0F1115] px-5 py-2 flex items-center justify-between border-b border-[#262B34] text-[11px] font-mono">
          <span className={step >= 1 ? 'text-blue-400 font-medium' : 'text-slate-500'}>1. Identity &amp; Data Path</span>
          <span className="text-slate-600">→</span>
          <span className={step >= 2 ? 'text-blue-400 font-medium' : 'text-slate-500'}>2. Boot Key</span>
          <span className="text-slate-600">→</span>
          <span className={step >= 3 ? 'text-blue-400 font-medium' : 'text-slate-500'}>3. Harvesting</span>
          <span className="text-slate-600">→</span>
          <span className={step >= 4 ? 'text-blue-400 font-medium' : 'text-slate-500'}>4. Launch</span>
        </div>

        {/* Step Content */}
        <div className="p-6 overflow-y-auto flex-1 text-xs space-y-4 scrollbar-thin">
          {/* STEP 1: Name, Network Identity & Data Path */}
          {step === 1 && (
            loadingConfig ? (
              <div className="py-16 flex flex-col items-center justify-center text-center space-y-3">
                <Loader2 className="w-6 h-6 animate-spin text-blue-400" />
                <p className="text-xs text-slate-400">Loading current configuration from disk...</p>
              </div>
            ) : (
              <div className="space-y-4">
                <div>
                  <h3 className="text-sm font-bold text-white">
                    Welcome to ProximaX Sirius Mainnet Peer Node
                  </h3>
                <p className="text-slate-400 mt-1">
                  This wizard will configure your validator node according to official ProximaX onboarding standards and prepare all configuration files (<code>config-user.properties</code>, <code>config-node.properties</code>, <code>config-harvesting.properties</code>).
                </p>
              </div>

              <div className="space-y-3 pt-2">
                <div>
                  <label className="block font-semibold text-slate-300 mb-1">
                    Node Friendly Name <span className="text-rose-400">*</span>
                  </label>
                  <input
                    type="text"
                    value={friendlyName}
                    onChange={(e) => setFriendlyName(e.target.value)}
                    placeholder="e.g. mainnet-validator-01"
                    className="w-full px-3 py-2 bg-[#0F1115] border border-[#262B34] rounded-md focus:outline-none focus:border-blue-500 font-mono text-xs text-slate-100 placeholder-slate-500"
                  />
                  <p className="text-[11px] text-slate-500 mt-1">
                    Assign a public identifier for your node that appears in telemetry and peer discovery.
                  </p>
                </div>

                <div>
                  <div className="flex items-center justify-between mb-1">
                    <label className="font-semibold text-slate-300">
                      Host Domain or Public IP (Optional)
                    </label>
                    <button
                      type="button"
                      onClick={detectPublicIp}
                      disabled={detectingIp}
                      className="text-xs text-blue-400 hover:underline flex items-center space-x-1 font-mono"
                    >
                      <span>{detectingIp ? 'Detecting...' : 'Auto-Detect Public IP'}</span>
                    </button>
                  </div>
                  <input
                    type="text"
                    value={host}
                    onChange={(e) => setHost(e.target.value)}
                    placeholder="Leave empty for automatic detection"
                    className="w-full px-3 py-2 bg-[#0F1115] border border-[#262B34] rounded-md focus:outline-none focus:border-blue-500 font-mono text-xs text-slate-100 placeholder-slate-500"
                  />
                  <p className="text-[11px] text-slate-500 mt-1">
                    If you have a static IP or DNS domain name, enter it here.
                  </p>
                </div>

                {/* Blockchain Data Path Selector */}
                <div className="p-3.5 bg-[#0F1115] rounded-lg border border-[#262B34] space-y-2">
                  <DirectoryDropdown
                    label="Blockchain Data Directory (data.path)"
                    value={dataPath}
                    onChange={setDataPath}
                    placeholder={typeof navigator !== 'undefined' && /win/i.test(navigator.userAgent || '') ? './chainconfig/data or D:\\Sirius_data' : './chainconfig/data or /Volumes/ExternalSSD/sirius_data'}
                    prompt="Select Sirius Blockchain Data Directory"
                  />
                  <p className="text-[11px] text-slate-400">
                    Specify where blockchain block database files will be stored. Default: <code className="text-blue-400">./chainconfig/data</code>. You can choose an external SSD or custom folder.
                  </p>
                  </div>
                </div>
              </div>
            )
          )}

          {/* STEP 2: Boot Key Setup */}
          {step === 2 && (
            <div className="space-y-4">
              <div>
                <h3 className="text-sm font-semibold text-white">
                  Step 2: Boot Key Setup (Networking Identity)
                </h3>
                <p className="text-slate-400 mt-1">
                  The Boot Key is the node's local server cryptographic keypair stored in <code>config-user.properties</code>. It identifies and authenticates your node on the P2P network. It is independent of your Harvesting Validator key.
                </p>
              </div>

              <div className="space-y-3 pt-2">
                <div className="flex items-center justify-between">
                  <label className="font-semibold text-slate-300 text-xs">
                    Boot Key Private Key (64 hex characters)
                  </label>
                  <button
                    type="button"
                    onClick={generateBootKey}
                    disabled={generatingBootKey}
                    className="px-2.5 py-1 bg-blue-600 hover:bg-blue-500 text-white rounded text-[11px] font-semibold flex items-center space-x-1 transition-colors shadow-xs"
                  >
                    <Key className="w-3 h-3" />
                    <span>{generatingBootKey ? 'Generating...' : 'Generate Random Key'}</span>
                  </button>
                </div>

                <div className="relative">
                  <input
                    type="text"
                    value={bootKey}
                    onChange={(e) => setBootKey(e.target.value)}
                    placeholder="Enter or generate 64-char hexadecimal boot key..."
                    className="w-full px-3 py-2 bg-[#0F1115] border border-[#262B34] rounded-md focus:outline-none focus:border-blue-500 font-mono text-xs text-slate-100 placeholder-slate-500"
                  />
                  {bootKey && (
                    <button
                      onClick={() => copyToClipboard(bootKey, 'bootKey')}
                      className="absolute right-2 top-2 p-1 text-slate-400 hover:text-white"
                    >
                      {copiedKey === 'bootKey' ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                    </button>
                  )}
                </div>

                {bootKeyPair && (
                  <div className="p-3.5 bg-[#0F1115] rounded-lg border border-[#262B34] space-y-1.5 font-mono text-[11px]">
                    <div className="text-blue-400 font-semibold font-sans">Generated Transport Keypair:</div>
                    <div className="text-slate-400 break-all">
                      <strong className="text-slate-200 font-sans">PublicKey:</strong> {bootKeyPair.publicKey}
                    </div>
                    <div className="text-slate-400 break-all">
                      <strong className="text-slate-200 font-sans">Public Address:</strong> {bootKeyPair.address}
                    </div>
                  </div>
                )}
              </div>
            </div>
          )}

          {/* STEP 3: Delegated Harvesting */}
          {step === 3 && (
            <div className="space-y-4">
              <div>
                <h3 className="text-sm font-bold text-white">
                  Step 3: Delegated Harvesting Configuration
                </h3>
                <p className="text-slate-400 mt-1">
                  How would you like to configure delegated validating and harvesting for this node?
                </p>
              </div>

              {/* Radio Options */}
              <div className="space-y-2">
                <label className={`flex items-start space-x-3 p-3.5 rounded-lg border cursor-pointer transition-all ${
                  harvestOption === 'generate' ? 'bg-blue-600/15 border-blue-500/40 text-white' : 'bg-[#0F1115] border-[#262B34] text-slate-300'
                }`}>
                  <input
                    type="radio"
                    name="harvestOption"
                    value="generate"
                    checked={harvestOption === 'generate'}
                    onChange={() => setHarvestOption('generate')}
                    className="mt-0.5"
                  />
                  <div>
                    <div className="font-semibold text-white">
                      Create & Link New Delegate Remote Key (Recommended)
                    </div>
                    <div className="text-slate-400 text-[11px] mt-0.5">
                      Uses Sirius Chain tool to create a remote key and announce AccountLinkTransaction.
                    </div>
                  </div>
                </label>

                <label className={`flex items-start space-x-3 p-3.5 rounded-lg border cursor-pointer transition-all ${
                  harvestOption === 'existing' ? 'bg-blue-600/15 border-blue-500/40 text-white' : 'bg-[#0F1115] border-[#262B34] text-slate-300'
                }`}>
                  <input
                    type="radio"
                    name="harvestOption"
                    value="existing"
                    checked={harvestOption === 'existing'}
                    onChange={() => setHarvestOption('existing')}
                    className="mt-0.5"
                  />
                  <div>
                    <div className="font-semibold text-white">
                      Enter or Generate Delegate Harvesting Key
                    </div>
                    <div className="text-slate-400 text-[11px] mt-0.5">
                      Enter your 64-hex remote key or generate a key pair for <code>config-harvesting.properties</code>.
                    </div>
                  </div>
                </label>
              </div>

              {/* Sub-form based on selection */}
              {harvestOption === 'generate' && (
                <div className="p-3.5 bg-[#0F1115] rounded-lg border border-[#262B34] space-y-3">
                  <div>
                    <label className="block font-semibold text-slate-300 mb-1">
                      Your Mainnet Account Private Key
                    </label>
                    <input
                      type="password"
                      value={accountPrivKey}
                      onChange={(e) => setAccountPrivKey(e.target.value)}
                      placeholder="64-character account private key..."
                      className="w-full px-3 py-2 bg-[#181B20] border border-[#262B34] rounded-md focus:outline-hidden focus:border-blue-500 font-mono text-xs text-slate-100 placeholder-slate-500"
                    />
                  </div>

                  <div>
                    <label className="block font-semibold text-slate-300 mb-1">
                      API Node
                    </label>
                    <select
                      value={apiNode}
                      onChange={(e) => setApiNode(e.target.value)}
                      className="w-full px-3 py-2 bg-[#181B20] border border-[#262B34] rounded-md focus:outline-hidden focus:border-blue-500 text-xs text-slate-100 font-mono"
                    >
                      <option value="http://aldebaran.xpxsirius.io:3000">http://aldebaran.xpxsirius.io:3000</option>
                      <option value="http://betelgeuse.xpxsirius.io:3000">http://betelgeuse.xpxsirius.io:3000</option>
                      <option value="http://bigcalvin.xpxsirius.io:3000">http://bigcalvin.xpxsirius.io:3000</option>
                    </select>
                  </div>

                  {harvestError && (
                    <div className="p-2.5 bg-rose-950/40 border border-rose-500/30 rounded-lg text-rose-400 text-xs flex items-center space-x-1.5">
                      <AlertCircle className="w-3.5 h-3.5 flex-shrink-0" />
                      <span>{harvestError}</span>
                    </div>
                  )}

                  {linkResult ? (
                    <div className="p-3 bg-[#181B20] border border-[#262B34] rounded-lg text-emerald-400 space-y-1.5 font-mono text-[11px]">
                      <div className="font-bold font-sans flex items-center gap-1">
                        <CheckCircle2 className="w-3.5 h-3.5" /> Account Linked Successfully!
                      </div>
                      <div className="break-all text-slate-400 flex items-center justify-between">
                        <div>
                          <strong className="text-slate-200 font-sans">Tx Hash: </strong>
                          <span>{linkResult.txHash}</span>
                        </div>
                        <a
                          href={getExplorerTxUrl(linkResult.txHash)}
                          target="_blank"
                          rel="noreferrer"
                          className="text-blue-400 hover:underline flex items-center gap-1 ml-2 flex-shrink-0 font-sans text-[10px]"
                          title="View on Explorer"
                        >
                          <span>Explorer</span>
                          <ExternalLink className="w-3 h-3" />
                        </a>
                      </div>
                      <div className="break-all text-slate-400 flex items-center justify-between">
                        <div>
                          <strong className="text-slate-200 font-sans">Remote Public Key: </strong>
                          <span>{linkResult.remotePublicKey}</span>
                        </div>
                        <a
                          href={getExplorerPublicKeyUrl(linkResult.remotePublicKey)}
                          target="_blank"
                          rel="noreferrer"
                          className="text-blue-400 hover:underline flex items-center gap-1 ml-2 flex-shrink-0 font-sans text-[10px]"
                          title="View on Explorer"
                        >
                          <span>Explorer</span>
                          <ExternalLink className="w-3 h-3" />
                        </a>
                      </div>
                      {linkResult.remoteAddress && (
                        <div className="break-all text-slate-400 flex items-center justify-between">
                          <div>
                            <strong className="text-slate-200 font-sans">Remote Address: </strong>
                            <span>{linkResult.remoteAddress}</span>
                          </div>
                          <a
                            href={getExplorerAddressUrl(linkResult.remoteAddress)}
                            target="_blank"
                            rel="noreferrer"
                            className="text-blue-400 hover:underline flex items-center gap-1 ml-2 flex-shrink-0 font-sans text-[10px]"
                            title="View on Explorer"
                          >
                            <span>Explorer</span>
                            <ExternalLink className="w-3 h-3" />
                          </a>
                        </div>
                      )}
                    </div>
                  ) : (
                    <button
                      type="button"
                      onClick={handleLinkAccount}
                      disabled={linking || !accountPrivKey}
                      className="w-full py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded-md text-xs font-semibold shadow-xs transition-colors flex items-center justify-center space-x-2"
                    >
                      <Zap className="w-3.5 h-3.5" />
                      <span>{linking ? 'Broadcasting Link Tx...' : 'Broadcast Delegated Harvesting Transaction'}</span>
                    </button>
                  )}
                </div>
              )}

              {harvestOption === 'existing' && (
                <div className="p-3.5 bg-[#0F1115] rounded-lg border border-[#262B34] space-y-3">
                  <div className="flex items-center justify-between">
                    <div>
                      <label className="block font-semibold text-slate-300">
                        Remote Harvesting Private Key
                      </label>
                      <p className="text-[11px] text-slate-400">
                        Enter an existing 64-hex key or generate a new random key pair.
                      </p>
                    </div>
                    <button
                      type="button"
                      disabled={generatingHarvestKey}
                      onClick={generateHarvestKey}
                      className="px-2.5 py-1 bg-[#181B20] hover:bg-[#262B34] border border-[#262B34] text-blue-400 hover:text-blue-300 rounded text-xs font-semibold transition-colors flex items-center space-x-1"
                    >
                      <RefreshCw className={`w-3 h-3 ${generatingHarvestKey ? 'animate-spin' : ''}`} />
                      <span>{generatingHarvestKey ? 'Generating...' : 'Generate New'}</span>
                    </button>
                  </div>
                  <div className="relative">
                    <input
                      type={showHarvestKey ? 'text' : 'password'}
                      value={existingHarvestKey}
                      onChange={(e) => setExistingHarvestKey(e.target.value.trim())}
                      placeholder="64-character remote private key..."
                      className="w-full pr-10 px-3 py-2 bg-[#181B20] border border-[#262B34] rounded-md focus:outline-hidden focus:border-blue-500 font-mono text-xs text-slate-100 placeholder-slate-500"
                    />
                    <button
                      type="button"
                      onClick={() => setShowHarvestKey(!showHarvestKey)}
                      className="absolute right-2 top-2 p-1 text-slate-400 hover:text-white"
                      title={showHarvestKey ? 'Hide key' : 'Show key'}
                    >
                      {showHarvestKey ? <EyeOff className="w-3.5 h-3.5" /> : <Eye className="w-3.5 h-3.5" />}
                    </button>
                  </div>
                  {existingHarvestKey.length > 0 && existingHarvestKey.length !== 64 && (
                    <p className="text-[11px] text-amber-400">
                      Must be exactly 64 hexadecimal characters ({existingHarvestKey.length}/64).
                    </p>
                  )}
                  {existingHarvestKey.length === 64 && /^[0-9a-fA-F]{64}$/.test(existingHarvestKey) && (
                    <p className="text-[11px] text-emerald-400 flex items-center space-x-1">
                      <Check className="w-3 h-3" />
                      <span>Valid 64-hex harvest key configured</span>
                    </p>
                  )}
                </div>
              )}
            </div>
          )}

          {/* STEP 4: Review & Finish */}
          {step === 4 && (
            <div className="space-y-4">
              <div>
                <h3 className="text-sm font-bold text-white">
                  Step 4: Review Configuration & Launch
                </h3>
                <p className="text-slate-400 mt-1">
                  Everything is ready. Your node configuration will be written to the respective property files.
                </p>
              </div>

              <div className="bg-[#0F1115] rounded-lg p-4 border border-[#262B34] space-y-2.5 font-mono text-xs">
                <div className="flex justify-between py-1 border-b border-[#262B34]">
                  <span className="text-slate-400">Friendly Name:</span>
                  <span className="font-semibold text-white">{friendlyName}</span>
                </div>
                <div className="flex justify-between py-1 border-b border-[#262B34]">
                  <span className="text-slate-400">Host:</span>
                  <span className="text-slate-200">{host || 'Auto-detect'}</span>
                </div>
                <div className="flex justify-between py-1 border-b border-[#262B34]">
                  <span className="text-slate-400">Boot Key (Transport):</span>
                  <span className="text-slate-200 font-mono">
                    {bootKeyPair?.publicKey ? `Public: ${bootKeyPair.publicKey.substring(0, 10)}...${bootKeyPair.publicKey.substring(54)}` : (bootKey ? '●●●●●●●●●●●●●●●●' : 'Not Set')}
                  </span>
                </div>
                <div className="flex justify-between py-1 border-b border-[#262B34]">
                  <span className="text-slate-400">Blockchain Data Path:</span>
                  <span className="text-blue-400">{dataPath || './chainconfig/data'}</span>
                </div>
                <div className="flex justify-between py-1">
                  <span className="text-slate-400">Harvesting Mode:</span>
                  <span className="font-semibold text-emerald-400">
                    Delegated Harvesting Enabled
                  </span>
                </div>
              </div>
            </div>
          )}
        </div>

        {/* Wizard Footer Navigation */}
        <div className="bg-[#0F1115] px-5 py-3 border-t border-[#262B34] flex items-center justify-between">
          <div>
            {step > 1 && (
              <button
                type="button"
                onClick={() => setStep(step - 1)}
                className="px-3.5 py-1.5 bg-[#181B20] hover:bg-[#262B34] border border-[#262B34] text-slate-200 rounded-md text-xs font-semibold flex items-center space-x-1 transition-colors"
              >
                <ChevronLeft className="w-3.5 h-3.5" />
                <span>Back</span>
              </button>
            )}
          </div>

          <div className="flex items-center space-x-2">
            {step < 4 ? (
              <button
                type="button"
                disabled={
                  loadingConfig ||
                  (step === 1 && !friendlyName) ||
                  (step === 2 && !bootKey) ||
                  (step === 3 && !(
                    (harvestOption === 'generate' && !!linkResult?.remotePrivateKey && linkResult.remotePrivateKey.length === 64) ||
                    (harvestOption === 'existing' && existingHarvestKey.trim().length === 64 && /^[0-9a-fA-F]{64}$/.test(existingHarvestKey.trim()))
                  ))
                }
                onClick={() => setStep(step + 1)}
                className="px-4 py-1.5 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded-md text-xs font-semibold shadow-xs transition-colors flex items-center space-x-1"
              >
                <span>Next</span>
                <ChevronRight className="w-3.5 h-3.5" />
              </button>
            ) : (
              <>
                <button
                  type="button"
                  disabled={loadingConfig || saving}
                  onClick={() => handleFinish(false)}
                  className="px-3.5 py-1.5 bg-[#181B20] hover:bg-[#262B34] text-slate-300 hover:text-white rounded-md text-xs font-semibold border border-[#262B34] transition-colors"
                >
                  Save & Exit
                </button>
                <button
                  type="button"
                  disabled={loadingConfig || saving}
                  onClick={() => handleFinish(true)}
                  className="px-4 py-1.5 bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white rounded-md text-xs font-semibold shadow-xs transition-colors flex items-center space-x-1.5"
                >
                  <Play className="w-3.5 h-3.5 fill-current" />
                  <span>Save & Start Node Now</span>
                </button>
              </>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};
