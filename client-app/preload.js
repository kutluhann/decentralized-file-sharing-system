const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('electronAPI', {
  selectFile: () => ipcRenderer.invoke('select-file'),
  storeFile: (data) => ipcRenderer.invoke('store-file', data),
  storeToDHT: (data) => ipcRenderer.invoke('store-to-dht', data),
  getFromDHT: (data) => ipcRenderer.invoke('get-from-dht', data),
  getFileMetadata: (data) => ipcRenderer.invoke('get-file-metadata', data),
  getFile: (data) => ipcRenderer.invoke('get-file', data),
  showSaveDialog: (defaultName) => ipcRenderer.invoke('show-save-dialog', defaultName),
  getFileHash: (data) => ipcRenderer.invoke('get-file-hash', data)
});
