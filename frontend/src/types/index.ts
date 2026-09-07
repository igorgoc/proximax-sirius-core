export type NodeStatus = 'running' | 'stopped' | 'starting' | 'error' | 'unknown';

export interface NodeMetrics {
  status: NodeStatus;
  uptime?: string;
  cpuPercent?: string;
  memoryUsage?: string;
  diskUsage?: string;
  diskFree?: string;
  containerId?: string;
  image?: string;
  blockHeight: number;
  networkHeight: number;
  peersCount: number;
  errorMessage?: string;
  threadsCount?: number;
  networkRate?: string;
  diskRate?: string;
  processPid?: number;
}

export interface NodeConfig {
  friendlyName: string;
  host: string;
  bootKey?: string;
  bootPublicKey?: string;
  bootKeySource: 'harvest' | 'generated' | string;
  harvestKey?: string;
  harvestPublicKey?: string;
  harvestAddress?: string;
  hasBootKey?: boolean;
  hasHarvestKey?: boolean;
  beneficiary: string;
  isAutoHarvesting: boolean;
  maxUnlockedAccounts: number;
  port: number;
  apiPort: number;
  dbrbPort: number;
  dataDirectory: string;
  dataPath: string;
  migrateData?: boolean;
  isConfigured: boolean;
}

export interface KeyPairInfo {
  privateKey: string;
  publicKey: string;
  address: string;
}

export interface AccountLinkResult {
  txHash: string;
  harvesterTxHash?: string;
  signerPublicKey: string;
  remotePrivateKey?: string;
  remotePublicKey: string;
  remoteAddress: string;
  status: string;
  message: string;
}

export interface PeerInfo {
  publicKey: string;
  port: number;
  networkIdentifier: number;
  version: number;
  roles: number;
  host: string;
  friendlyName: string;
  latencyMs?: number;
}

export type SnapshotStage = 'idle' | 'downloading' | 'downloaded' | 'extracting' | 'complete' | 'cancelled' | 'error';

export interface DownloadProgress {
  downloadedBytes: number;
  totalBytes: number;
  percentage: number;
  speedMBs: number;
  etaSeconds: number;
}

export interface ExtractProgress {
  extractedFiles: number;
  extractedBytes: number;
  currentFile: string;
  percentage: number;
}

export interface SnapshotStatus {
  stage: SnapshotStage;
  download: DownloadProgress;
  extract: ExtractProgress;
  message: string;
  errorMessage?: string;
}

export type DataBackupStage = 'idle' | 'backing_up' | 'complete' | 'cancelled' | 'error';

export interface DataBackupStatus {
  stage: DataBackupStage;
  backedUpBytes: number;
  totalBytes: number;
  percentage: number;
  currentFile: string;
  targetFile?: string;
  format?: string;
  message: string;
  errorMessage?: string;
}

export interface StorageConvertStatus {
  status: 'idle' | 'running' | 'completed' | 'failed' | 'cancelled';
  processedDirs: number;
  totalDirs: number;
  convertedBlocks: number;
  deletedFiles: number;
  bytesWritten: number;
  percent: number;
  currentDir: string;
  message: string;
  error?: string;
}

export interface HarvesterStatus {
  isLinked: boolean;
  accountType?: number;
  accountAddress?: string;
  accountPublicKey?: string;
  linkedPublicKey?: string;
  linkedAddress?: string;
  linkedBalanceXPX?: string;
  linkedRawBalanceXPX?: number;
  balanceXPX?: string;
  rawBalanceXPX?: number;
  isCommitteeHarvester?: boolean;
  canHarvest?: boolean;
  effectiveBalance?: string;
  lastSigningBlockHeight?: number;
  activity?: number;
  greed?: number;
  isEligible?: boolean;
  statusText: string;
  explorerUrl?: string;
  linkedExplorerUrl?: string;
}

export interface ValidatedBlock {
  height: number;
  hash: string;
  timestamp: string;
  feeXPX: number;
  numTransactions: number;
  signer: string;
}

export interface HarvestStats {
  totalBlocksValidated: number;
  totalEarnedFeesXPX: number;
  lastHarvestedHeight?: number;
  lastHarvestedTime?: string;
  validatedBlocks: ValidatedBlock[];
}

export interface UpdateInfo {
  currentVersion: string;
  latestVersion: string;
  hasUpdate: boolean;
  releaseTitle?: string;
  releaseNotes?: string;
  releaseUrl?: string;
  publishedAt?: string;
  lastChecked?: string;
  officialFiles?: string[];
  isApplying?: boolean;
  updateMessage?: string;
}

export interface EngineUpdateStatus {
  currentVersion: string;
  targetVersion?: string;
  hasUpdate: boolean;
  releaseNotes?: string;
  releaseUrl?: string;
  isApplying: boolean;
  state: 'idle' | 'verifying' | 'swapping' | 'healthcheck' | 'completed' | 'rolled_back' | 'failed';
  message?: string;
  lastChecked?: string;
  rollbackOccurred?: boolean;
}

export interface SeedPingInfo {
  endpoint: string;
  latencyMs: number;
  status: string;
}

export interface PeersDetailResponse {
  peers: PeerInfo[];
  seedNodes: SeedPingInfo[];
  count: number;
}

export interface StorageConfig {
  key?: string;
  hasKey?: boolean;
  publicKey: string;
  address: string;
  host: string;
  port: number;
  storageDirectory: string;
  sandboxDirectory: string;
  storagePath?: string;
  defaultStoragePath?: string;
  resolvedPath?: string;
  useTcpSocket: boolean;
  useRpcReplicator: boolean;
  isConfigured: boolean;
}

export interface StorageMetrics {
  driveSizeBytes: number;
  driveSizeStr: string;
  sandboxSizeBytes: number;
  sandboxSizeStr: string;
  totalShardsCount: number;
  isStorageActive: boolean;
}

export interface ReplicatorAccountInfo {
  address: string;
  publicKey: string;
  balanceXPX: number;
  balanceSO?: number;
  balanceSI?: number;
  isRegistered: boolean;
  accountType: string;
  storageDeposit?: number;
}

export interface ReplicatorPeer {
  name: string;
  host: string;
  port: number;
  publicKey: string;
  latencyMs: number;
  isReachable: boolean;
}

export interface StorageStatus {
  config: StorageConfig;
  metrics: StorageMetrics;
  onChain: ReplicatorAccountInfo;
  peers: ReplicatorPeer[];
  lastScan?: string;
}

export interface OnboardReplicatorResult {
  txHash: string;
  signer: string;
  capacityGB: number;
  status: string;
  message: string;
}

export interface PortCheckResult {
  publicIp: string;
  localIp: string;
  routerName: string;
  upnpActive: boolean;
  upnpMessage: string;
  port7900Open: boolean;
  port7904Open: boolean;
  port7900Status: string;
  port7904Status: string;
  lastCheckTime?: string;
}

export interface ActiveValidatorSummary {
  publicKey: string;
  shortKey: string;
  blocksCount: number;
  sharePercent: number;
  lastSeenHeight: number;
  lastSeenTime: string;
  isSelf: boolean;
}

export interface NetworkValidatorStats {
  activeValidators4h: number;
  activeValidators24h: number;
  estimatedStakedPoolXPX: number;
  avgBlockTimeSec: number;
  totalNetworkFees4h: number;
  recentBlocksCount: number;
  topValidators: ActiveValidatorSummary[];
}


