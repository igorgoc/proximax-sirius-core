const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('electronAPI', {
  onRequestQuit: (callback) => {
    ipcRenderer.on('request-quit-action', () => callback());
  },
  sendQuitResponse: (action) => {
    ipcRenderer.send('quit-action-response', action);
  },
  isElectron: true
});
