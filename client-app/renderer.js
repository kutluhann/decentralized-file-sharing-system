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
      const errorHopCount = dhtResult.data?.hop_count !== undefined ? dhtResult.data.hop_count : 0;
      showStatus(getStatus, 'error', 
        `File not found in DHT: ${dhtResult.error}<br>` +
        `<strong>Hop count:</strong> ${errorHopCount}<br>` +
        `${dhtResult.response ? JSON.stringify(dhtResult.response) : ''}`);
      getBtn.disabled = false;
      return;
    }

    // The value should be the same as the key (file hash)
    const storageAddress = dhtResult.data.value;
    const hopCount = dhtResult.data.hop_count !== undefined ? dhtResult.data.hop_count : 0;
    showStatus(getStatus, 'info', 
      `Found in DHT! Hop count: ${hopCount}<br>Fetching file metadata...`);

    // Step 2: Get file metadata to extract original filename
    const metadataResult = await window.electronAPI.getFileMetadata({
      fileStorageUrl: storageAddress,
      hash: fileHash
    });

    let defaultFilename = 'downloaded-file';
    if (metadataResult.success && metadataResult.filename) {
      defaultFilename = metadataResult.filename;
      showStatus(getStatus, 'info', 
        `Found file: ${defaultFilename}. Downloading...`);
    } else {
      showStatus(getStatus, 'info', 'Downloading file...');
    }

    // Step 3: Download file from file-storage server to memory
    const downloadResult = await window.electronAPI.downloadFile({
      fileStorageUrl: storageAddress,
      hash: fileHash
    });

    if (!downloadResult.success) {
      showStatus(getStatus, 'error', `Failed to download file: ${downloadResult.error}`);
      getBtn.disabled = false;
      return;
    }

    showStatus(getStatus, 'info', 'Verifying file integrity...');

    // Step 4: Check integrity before asking where to save
    const calculatedHashResult = await window.electronAPI.getFileHash({
      fileBuffer: downloadResult.buffer
    });

    if (!calculatedHashResult.success) {
      showStatus(getStatus, 'error', `Failed to calculate hash: ${calculatedHashResult.error}`);
      getBtn.disabled = false;
      return;
    }

    if (calculatedHashResult.hash !== fileHash) {
      showStatus(getStatus, 'error', 
        `⚠️ File integrity check FAILED!<br>` +
        `Expected: ${fileHash}<br>` +
        `Received: ${calculatedHashResult.hash}<br>` +
        `The file may be corrupted or tampered with.`);
      getBtn.disabled = false;
      return;
    }

    // Step 5: Integrity verified! Now ask user where to save
    showStatus(getStatus, 'info', 
      `✓ Integrity verified! Choose where to save ${defaultFilename}...`);

    const savePath = await window.electronAPI.showSaveDialog(defaultFilename);
    if (!savePath) {
      showStatus(getStatus, 'info', 'Save cancelled by user. File was verified but not saved.');
      getBtn.disabled = false;
      return;
    }

    // Step 6: Save the verified file
    showStatus(getStatus, 'info', 'Saving file...');

    const saveResult = await window.electronAPI.saveFile({
      buffer: downloadResult.buffer,
      savePath: savePath
    });

    if (!saveResult.success) {
      showStatus(getStatus, 'error', `Failed to save file: ${saveResult.error}`);
      getBtn.disabled = false;
      return;
    }

    // Success!
    showStatus(getStatus, 'success', 
      `✓ File retrieved and verified successfully!<br>` +
      `<strong>Saved to:</strong> ${saveResult.savedPath}<br>` +
      `<strong>Size:</strong> ${formatBytes(downloadResult.size)}<br>` +
      `<strong>Hash verified:</strong> ✓<br>` +
      `<strong>Hop count:</strong> ${hopCount}<br>` +
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
