# Decentralized Temper-Proof File Sharing System

DHT implementation with ECDSA authentication and Proof of Space Sybil resistance.

## How to Run

### Local Setup

Start genesis node:
```bash
go run main.go -genesis -port 8080 -http 8000
```

Join network:
```bash
go run main.go -port 8081 -http 8001 -bootstrap 127.0.0.1:8080
```

### Docker

Start 1 bootstrap + 5 nodes:
```bash
docker-compose up -d --scale dht-node=5
```

Stop:
```bash
docker-compose down -v
```

### Usage

Store value:
```bash
curl -X POST http://localhost:8000/store \
  -H "Content-Type: application/json" \
  -d '{"key":"myfile","value":"data"}'
```

Get value:
```bash
curl -X POST http://localhost:8000/get \
  -H "Content-Type: application/json" \
  -d '{"key":"myfile"}'
```
