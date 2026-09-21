const EXPLORER_BASE_URL = 'https://explorer.xpxsirius.io';

export const getExplorerAddressUrl = (address: string): string => {
  if (!address) return EXPLORER_BASE_URL;
  return `${EXPLORER_BASE_URL}/#/account/${encodeURIComponent(address.trim())}`;
};

export const getExplorerPublicKeyUrl = (publicKey: string): string => {
  if (!publicKey) return EXPLORER_BASE_URL;
  return `${EXPLORER_BASE_URL}/#/account/${encodeURIComponent(publicKey.trim())}`;
};

export const getExplorerTxUrl = (txHash: string): string => {
  if (!txHash) return EXPLORER_BASE_URL;
  return `${EXPLORER_BASE_URL}/#/tx/${encodeURIComponent(txHash.trim())}`;
};

export const getExplorerBlockUrl = (height: number | string): string => {
  if (!height) return EXPLORER_BASE_URL;
  return `${EXPLORER_BASE_URL}/#/block/${height}`;
};

export const EXPLORER_URL = EXPLORER_BASE_URL;
