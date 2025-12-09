const { app, BrowserWindow, ipcMain, dialog } = require('electron');
const path = require('path');
const fs = require('fs');
const crypto = require('crypto');
const axios = require('axios');

let mainWindow;

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1000,
    height: 800,
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true,
      preload: path.join(__dirname, 'preload.js')
    }
  });

  mainWindow.loadFile('index.html');

  // Open DevTools in development mode
  if (process.argv.includes('--dev')) {
    mainWindow.webContents.openDevTools();
  }
}

app.whenReady().then(createWindow);

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    app.quit();
  }
});

app.on('activate', () => {
  if (BrowserWindow.getAllWindows().length === 0) {
    createWindow();
  }
});

// Handle file selection
ipcMain.handle('select-file', async () => {
  const result = await dialog.showOpenDialog(mainWindow, {
    properties: ['openFile']
  });

  if (result.canceled || result.filePaths.length === 0) {
    return null;
  }

  const filePath = result.filePaths[0];
  const fileBuffer = fs.readFileSync(filePath);
  const fileName = path.basename(filePath);

  return {
    path: filePath,
    name: fileName,
    buffer: Array.from(fileBuffer), // Convert Buffer to array for IPC
    size: fileBuffer.length
  };
});

// Store file to file-storage server
ipcMain.handle('store-file', async (event, { fileStorageUrl, fileSalt, fileData }) => {
  try {
    const fileBuffer = Buffer.from(fileData.buffer);
    
    // Calculate hash exactly as the server does: SHA256(salt + fileBuffer)
    const hash = crypto.createHash('sha256')
      .update(fileSalt)
      .update(fileBuffer)
      .digest('hex');

    // Create form data
    const FormData = require('form-data');
    const formData = new FormData();
    formData.append('file', fileBuffer, fileData.name);

    // Upload to file-storage server
    const response = await axios.post(`${fileStorageUrl}/files`, formData, {
      headers: formData.getHeaders(),
      maxBodyLength: Infinity,
      maxContentLength: Infinity
    });

    return {
      success: true,
      hash: hash,
      serverHash: response.data.hash,
      originalName: fileData.name,
      size: fileData.size
    };
  } catch (error) {
    return {
      success: false,
      error: error.message
    };
  }
});

// Store hash to DHT node
ipcMain.handle('store-to-dht', async (event, { dhtNodeUrl, key, value }) => {
  try {
    const response = await axios.post(`${dhtNodeUrl}/store`, {
      key: key,
      value: value
    }, {
      headers: {
        'Content-Type': 'application/json'
      }
    });

    return {
      success: true,
      data: response.data
    };
  } catch (error) {
    return {
      success: false,
      error: error.message
    };
  }
});

// Get value from DHT node
ipcMain.handle('get-from-dht', async (event, { dhtNodeUrl, key }) => {
  try {
    const response = await axios.post(`${dhtNodeUrl}/get`, {
      key: key
    }, {
      headers: {
        'Content-Type': 'application/json'
      }
    });

    return {
      success: true,
      data: response.data
    };
  } catch (error) {
    return {
      success: false,
      error: error.message,
      response: error.response?.data
    };
  }
});

// Get file from file-storage server
ipcMain.handle('get-file', async (event, { fileStorageUrl, hash, savePath }) => {
  try {
    const response = await axios.get(`${fileStorageUrl}/files/${hash}`, {
      responseType: 'arraybuffer'
    });

    // Save the file
    fs.writeFileSync(savePath, Buffer.from(response.data));

    return {
      success: true,
      savedPath: savePath,
      size: response.data.byteLength
    };
  } catch (error) {
    return {
      success: false,
      error: error.message
    };
  }
});

// Show save dialog
ipcMain.handle('show-save-dialog', async (event, defaultName) => {
  const result = await dialog.showSaveDialog(mainWindow, {
    defaultPath: defaultName || 'downloaded-file',
    properties: ['createDirectory', 'showOverwriteConfirmation']
  });

  if (result.canceled) {
    return null;
  }

  return result.filePath;
});
