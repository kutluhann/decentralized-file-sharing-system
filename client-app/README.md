# DHT File Sharing Client

Electron desktop application for storing and retrieving files using the decentralized DHT network.

## Features

- **File Upload**: Select and upload files to the file-storage server
- **DHT Integration**: Automatically stores file hashes in the DHT network
- **File Retrieval**: Query the DHT to find and download files
- **Hash Matching**: Uses the same SHA-256 + salt hashing as the file-storage server

## Architecture

The client acts as a bridge between two systems:

1. **File Storage Server** (`file-storage/`): Stores actual file content, indexed by hash
2. **DHT Network** (`main.go`): Stores key-value mappings (filename → file hash)

### Store Flow

1. User selects a file and provides a key (e.g., "my-document")
2. File is hashed using `SHA256(SALT + file_content)`
3. File is uploaded to file-storage server via `POST /files`
4. Hash is stored in DHT via `POST /store` with the user's key
5. DHT replicates the key→hash mapping to K closest nodes

### Retrieve Flow

1. User provides the same key (e.g., "my-document")
2. Client queries DHT via `POST /get` with the key
3. DHT returns the file hash via Kademlia lookup
4. Client downloads file from file-storage server via `GET /files/:hash`
5. User saves the file locally

## Setup

```bash
cd client-app
npm install
```

## Usage

```bash
npm start
```

### Configuration

Before storing or retrieving files, configure:

- **File Storage Server URL**: URL of the file-storage service (default: `http://localhost:4000`)
- **File Salt**: Must match the system salt `dfss-ulak-bibliotheca` (hardcoded)
- **DHT Node URL**: URL of any DHT node's HTTP API (default: `http://localhost:8000`)

### Storing a File

1. Click "Select File" and choose a file
2. Enter a memorable key (e.g., "contract-2025")
3. Click "Store File to System"
4. The app will:
   - Upload the file to storage server
   - Register the hash in the DHT
   - Show success with both hashes

### Retrieving a File

1. Enter the same key you used when storing
2. Click "Retrieve File from System"
3. Choose where to save the downloaded file
4. The app will:
   - Query the DHT for the hash
   - Download the file from storage server
   - Save it to your chosen location

## Technical Details

### Hash Calculation

The client calculates file hashes **exactly** as the server does:

```javascript
const hash = crypto.createHash('sha256')
  .update(fileSalt)      // Salt first
  .update(fileBuffer)     // Then file content
  .digest('hex');
```

This ensures the hash used for DHT storage matches the storage server's hash.

### DHT Key Hashing

The DHT node hashes the user-provided key:

```go
keyHash := sha256.Sum256([]byte(req.Key))  // "my-document" → hash
```

So when storing "my-document", the DHT actually stores:
- `SHA256("my-document")` → `SHA256(SALT + file_content)`

## Dependencies

- **Electron**: Desktop application framework
- **axios**: HTTP client for API requests
- **form-data**: For multipart file uploads
- **crypto**: Built-in Node.js module for hashing

## Development

Run with DevTools open:

```bash
npm run dev
```
