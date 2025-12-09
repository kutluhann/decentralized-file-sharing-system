# File Storage Service

Minimal Express service that stores files on disk by the salted sha256 hash of their contents and serves them back when the same hash is requested.

## Setup

1. `cp .env.example .env` and adjust `PORT`, `FILE_SALT`, and optional `MAX_FILE_SIZE_MB`.
2. Install dependencies: `npm install` inside `file-storage/`.
3. Start the server: `npm start`.

The service writes data under the `storage/` directory and will create files for both the blob itself and a sidecar metadata JSON.

## API

### `POST /files`
Multipart upload with the field name `file`. The service calculates the salted hash, persists the file, and returns the hash key.

```bash
curl -X POST http://localhost:4000/files \
  -F "file=@/path/to/local-file.pdf"
```

Example response:

```json
{
  "hash": "f4bbd2c7...",
  "bytes": 12345,
  "originalName": "local-file.pdf"
}
```

### `GET /files/:hash`
Returns the raw file contents for a previously uploaded hash. The `Content-Type` and download filename are restored from the stored metadata.

```bash
curl -L -o downloaded.pdf http://localhost:4000/files/f4bbd2c7...
```

### `GET /health`
Simple readiness probe that returns `{ "ok": true }`.
