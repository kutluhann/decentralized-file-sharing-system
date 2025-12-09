let selectedFile = null;

const FILE_SALT = 'dfss-ulak-bibliotheca'

// DOM elements
const fileStorageUrlInput = document.getElementById('fileStorageUrl');
const dhtNodeUrlInput = document.getElementById('dhtNodeUrl');
const selectedFileInfo = document.getElementById('selectedFileInfo');
const selectFileBtn = document.getElementById('selectFileBtn');
const storeBtn = document.getElementById('storeBtn');
const storeStatus = document.getElementById('storeStatus');
const getKeyInput = document.getElementById('getKey');
const getBtn = document.getElementById('getBtn');
const getStatus = document.getElementById('getStatus');

// Select file button
selectFileBtn.addEventListener('click', async () => {
  const fileData = await window.electronAPI.selectFile();
  
  if (fileData) {
    selectedFile = fileData;
    selectedFileInfo.textContent = `${fileData.name} (${formatBytes(fileData.size)})`;
    selectedFileInfo.classList.add('selected');
    storeBtn.disabled = false;
  }
});

// Store file button
storeBtn.addEventListener('click', async () => {
  if (!selectedFile) {
    showStatus(storeStatus, 'error', 'No file selected');
    return;
  }

  const fileStorageUrl = fileStorageUrlInput.value.trim();
  const fileSalt = FILE_SALT;
  const dhtNodeUrl = dhtNodeUrlInput.value.trim();

  if (!fileStorageUrl || !fileSalt || !dhtNodeUrl) {
    showStatus(storeStatus, 'error', 'Please fill in all configuration fields');
    return;
  }

  storeBtn.disabled = true;
  showStatus(storeStatus, 'info', 'Uploading file to storage server...');

  try {
    // Step 1: Store file to file-storage server
    const storeResult = await window.electronAPI.storeFile({
      fileStorageUrl,
      fileSalt,
      fileData: selectedFile
    });

    if (!storeResult.success) {
      showStatus(storeStatus, 'error', `Failed to store file: ${storeResult.error}`);
      storeBtn.disabled = false;
      return;
    }

    const fileHash = storeResult.hash;
    showStatus(storeStatus, 'info', 
      `File stored! Hash: ${fileHash}<br>Registering in DHT...`);

    // Step 2: Store in DHT using fileHash as BOTH key and value
    // The DHT stores: hash(fileHash) -> fileHash
    const dhtResult = await window.electronAPI.storeToDHT({
      dhtNodeUrl,
      key: fileHash,
      value: fileStorageUrl
    });

    if (!dhtResult.success) {
      showStatus(storeStatus, 'error', 
        `File stored but DHT registration failed: ${dhtResult.error}<br>` +
        `You can still retrieve using hash: <div class="hash-display">${fileHash}</div>`);
      storeBtn.disabled = false;
      return;
    }

    // Success!
    showStatus(storeStatus, 'success', 
      `✓ Success! File stored and registered in DHT<br>` +
      `<strong>File Hash (use this to retrieve):</strong> <div class="hash-display">${fileHash}</div>` +
      `<strong>File:</strong> ${selectedFile.name} (${formatBytes(selectedFile.size)})`);

  } catch (error) {
    showStatus(storeStatus, 'error', `Unexpected error: ${error.message}`);
  } finally {
    storeBtn.disabled = false;
  }
});

// Get file button
getBtn.addEventListener('click', async () => {
  const dhtNodeUrl = dhtNodeUrlInput.value.trim();
  const fileStorageUrl = fileStorageUrlInput.value.trim();
  const fileHash = getKeyInput.value.trim();

  if (!dhtNodeUrl || !fileStorageUrl || !fileHash) {
    showStatus(getStatus, 'error', 'Please fill in all configuration fields and file hash');
    return;
  }

  // Validate hash format (64 hex characters)
  if (!/^[a-f0-9]{64}$/i.test(fileHash)) {
    showStatus(getStatus, 'error', 'Invalid file hash format. Must be 64 hexadecimal characters.');
    return;
  }

  getBtn.disabled = true;
  showStatus(getStatus, 'info', 'Querying DHT for file...');

  try {
    // Step 1: Query DHT using file hash as key
    const dhtResult = await window.electronAPI.getFromDHT({
      dhtNodeUrl,
      key: fileHash
    });

    if (!dhtResult.success) {
      showStatus(getStatus, 'error', 
        `File not found in DHT: ${dhtResult.error}<br>` +
        `${dhtResult.response ? JSON.stringify(dhtResult.response) : ''}`);
      getBtn.disabled = false;
      return;
    }

    // The value should be the same as the key (file hash)
    const storageAddress = dhtResult.data.value;
    showStatus(getStatus, 'info', 
      `Found in DHT! Fetching file metadata...`);

    // Step 2: Get file metadata to extract original filename
    const metadataResult = await window.electronAPI.getFileMetadata({
      fileStorageUrl: storageAddress,
      hash: fileHash
    });

    let defaultFilename = 'downloaded-file';
    if (metadataResult.success && metadataResult.filename) {
      defaultFilename = metadataResult.filename;
      showStatus(getStatus, 'info', 
        `Found file: ${defaultFilename}. Choose where to save...`);
    } else {
      showStatus(getStatus, 'info', 'Choose where to save file...');
    }

    // Step 3: Ask user where to save with original filename as default
    const savePath = await window.electronAPI.showSaveDialog(defaultFilename);
    if (!savePath) {
      showStatus(getStatus, 'info', 'Download cancelled by user');
      getBtn.disabled = false;
      return;
    }

    showStatus(getStatus, 'info', 'Downloading file...');

    // Step 4: Download file from file-storage server
    const fileResult = await window.electronAPI.getFile({
      fileStorageUrl: storageAddress,
      hash: fileHash,
      savePath: savePath,
    });

    if (!fileResult.success) {
      showStatus(getStatus, 'error', `Failed to download file: ${fileResult.error}`);
      getBtn.disabled = false;
      return;
    }

    // Step 5 - Check integrity
    const calculatedHashResult = await window.electronAPI.getFileHash({
      fileBuffer: fileResult.buffer,
    });

    if(calculatedHashResult.success && calculatedHashResult.hash !== fileHash) {
      showStatus(getStatus, 'error', 
        `File integrity check failed! Expected hash: ${fileHash}, Received hash: ${calculatedHashResult.hash}`);
      getBtn.disabled = false;
      return;
    }

    // Success!
    showStatus(getStatus, 'success', 
      `✓ File retrieved successfully!<br>` +
      `<strong>Saved to:</strong> ${fileResult.savedPath}<br>` +
      `<strong>Size:</strong> ${formatBytes(fileResult.size)}<br>` +
      `<strong>Storage Address:</strong> <div class="hash-display">${storageAddress}</div>`);

  } catch (error) {
    showStatus(getStatus, 'error', `Unexpected error: ${error.message}`);
  } finally {
    getBtn.disabled = false;
  }
});

// Helper functions
function showStatus(element, type, message) {
  element.className = `status ${type}`;
  element.innerHTML = message;
}

function formatBytes(bytes) {
  if (bytes === 0) return '0 Bytes';
  const k = 1024;
  const sizes = ['Bytes', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
}
