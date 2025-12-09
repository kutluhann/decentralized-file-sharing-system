const express = require('express');
const multer = require('multer');
const dotenv = require('dotenv');
const crypto = require('crypto');
const path = require('path');
const fs = require('fs/promises');
const fsSync = require('fs');

dotenv.config();

const PORT = Number(process.env.PORT || 4000);
const FILE_SALT = process.env.FILE_SALT || 'default_salt';
const MAX_FILE_SIZE_MB = Number(process.env.MAX_FILE_SIZE_MB || 10);
const MAX_FILE_SIZE_BYTES = MAX_FILE_SIZE_MB * 1024 * 1024;
const STORAGE_ROOT = path.join(__dirname, '..', 'storage');

if (!fsSync.existsSync(STORAGE_ROOT)) {
  fsSync.mkdirSync(STORAGE_ROOT, { recursive: true });
}

const app = express();
const upload = multer({
  storage: multer.memoryStorage(),
  limits: { fileSize: MAX_FILE_SIZE_BYTES }
});

app.get('/health', (_, res) => {
  res.json({ ok: true });
});

// Hash + persist uploaded file, returning the deterministic hash key.
app.post('/files', upload.single('file'), async (req, res, next) => {
  try {
    if (!req.file) {
      return res.status(400).json({ error: 'Missing file field named "file".' });
    }

    const hash = crypto.createHash('sha256').update(FILE_SALT).update(req.file.buffer).digest('hex');
    const filePath = path.join(STORAGE_ROOT, hash);
    const metadataPath = `${filePath}.json`;

    await fs.writeFile(filePath, req.file.buffer);
    await fs.writeFile(
      metadataPath,
      JSON.stringify(
        {
          originalName: req.file.originalname,
          mimeType: req.file.mimetype || 'application/octet-stream',
          size: req.file.size,
          uploadedAt: new Date().toISOString()
        },
        null,
        2
      )
    );

    return res.status(201).json({ hash, bytes: req.file.size, originalName: req.file.originalname });
  } catch (error) {
    return next(error);
  }
});

app.get('/files/:hash', async (req, res, next) => {
  try {
    const { hash } = req.params;
    const sanitizedHash = /^[a-f0-9]{64}$/i.test(hash) ? hash : null;
    if (!sanitizedHash) {
      return res.status(400).json({ error: 'Invalid hash provided.' });
    }

    const filePath = path.join(STORAGE_ROOT, sanitizedHash);
    const metadataPath = `${filePath}.json`;

    await fs.access(filePath);

    let meta = null;
    try {
      const metadataRaw = await fs.readFile(metadataPath, 'utf8');
      meta = JSON.parse(metadataRaw);
    } catch (err) {
      meta = null;
    }

    if (meta?.mimeType) {
      res.setHeader('Content-Type', meta.mimeType);
    }
    const downloadName = meta?.originalName || sanitizedHash;
    res.setHeader('Content-Disposition', `attachment; filename="${downloadName}"`);

    return res.sendFile(filePath);
  } catch (error) {
    if (error.code === 'ENOENT') {
      return res.status(404).json({ error: 'File not found for provided hash.' });
    }
    return next(error);
  }
});

app.use((err, _req, res, _next) => {
  console.error(err);
  res.status(500).json({ error: 'Internal server error.' });
});

app.listen(PORT, () => {
  console.log(`File storage service listening on port ${PORT}`);
});
