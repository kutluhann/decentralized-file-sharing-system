package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/kutluhann/decentralized-file-sharing-system/dht"
)

// StoreRequest represents the JSON payload for storing data
type StoreRequest struct {
	Key   string `json:"key"`   // Human-readable key (will be hashed to NodeID)
	Value string `json:"value"` // Value to store (string or base64 for binary)
}

// StoreResponse represents the response after storing
type StoreResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	KeyHash string `json:"key_hash"` // The actual hash used as key
}

// GetRequest represents the JSON payload for retrieving data
type GetRequest struct {
	Key string `json:"key"` // Human-readable key (will be hashed to NodeID)
}

// GetResponse represents the response after retrieval
type GetResponse struct {
	Success  bool   `json:"success"`
	Message  string `json:"message,omitempty"`
	KeyHash  string `json:"key_hash"`
	Value    string `json:"value,omitempty"`
	HopCount int    `json:"hop_count"` // Number of network hops to find the value
}

// StatusResponse represents node status information
type StatusResponse struct {
	NodeID        string `json:"node_id"`
	IP            string `json:"ip"`
	Port          int    `json:"port"`
	StoredKeys    int    `json:"stored_keys"`
	KnownPeers    int    `json:"known_peers"`
	NetworkStatus string `json:"network_status"`
}

// HTTPServer wraps the DHT node and provides HTTP endpoints
type HTTPServer struct {
	Node *dht.Node
	Port int
}

// NewHTTPServer creates a new HTTP server instance
func NewHTTPServer(node *dht.Node, port int) *HTTPServer {
	return &HTTPServer{
		Node: node,
		Port: port,
	}
}

// Start begins listening for HTTP requests
func (s *HTTPServer) Start() error {

	// SERVE FRONTEND FILES
	fs := http.FileServer(http.Dir("./frontend"))
	http.Handle("/", fs)

	// Set up routes
	http.HandleFunc("/store", s.handleStore)
	http.HandleFunc("/get", s.handleGet)
	http.HandleFunc("/status", s.handleStatus)
	http.HandleFunc("/health", s.handleHealth)
	http.HandleFunc("/routing-table", s.handleRoutingTable)

	addr := fmt.Sprintf(":%d", s.Port)
	fmt.Printf("[HTTP-API] Starting HTTP server on %s\n", addr)
	fmt.Printf("[HTTP-API] Endpoints available:\n")
	fmt.Printf("[HTTP-API]   POST   /store  - Store a key-value pair\n")
	fmt.Printf("[HTTP-API]   POST   /get    - Retrieve a value by key\n")
	fmt.Printf("[HTTP-API]   GET    /status - Get node status\n")
	fmt.Printf("[HTTP-API]   GET    /health - Health check\n")

	return http.ListenAndServe(addr, nil)
}

// handleStore handles POST requests to store data in the DHT
func (s *HTTPServer) handleStore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var req StoreRequest
	err = json.Unmarshal(body, &req)
	if err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Key == "" || req.Value == "" {
		http.Error(w, "Key and value are required", http.StatusBadRequest)
		return
	}

	// Hash the key to get NodeID
	// TODO: Burada Kaan'ın fonksiyonları kullanmmız gerekmez mi?
	keyHash := sha256.Sum256([]byte(req.Key))
	nodeID := dht.NodeID(keyHash)
	keyHashHex := hex.EncodeToString(keyHash[:])

	fmt.Printf("[HTTP-API] Store request: key='%s' -> hash=%s, value_size=%d bytes\n",
		req.Key, keyHashHex[:16], len(req.Value))

	// Store in DHT
	err = s.Node.Store(nodeID, []byte(req.Value))
	if err != nil {
		resp := StoreResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to store: %v", err),
			KeyHash: keyHashHex,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Success response
	resp := StoreResponse{
		Success: true,
		Message: "Successfully stored in DHT",
		KeyHash: keyHashHex,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleGet handles POST requests to retrieve data from the DHT
func (s *HTTPServer) handleGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var req GetRequest
	err = json.Unmarshal(body, &req)
	if err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Key == "" {
		http.Error(w, "Key is required", http.StatusBadRequest)
		return
	}

	// Hash the key to get NodeID
	keyHash := sha256.Sum256([]byte(req.Key))
	nodeID := dht.NodeID(keyHash)
	keyHashHex := hex.EncodeToString(keyHash[:])

	fmt.Printf("[HTTP-API] Get request: key='%s' -> hash=%s\n",
		req.Key, keyHashHex[:16])

	// Retrieve from DHT (with hop count)
	value, hopCount, err := s.Node.FindValue(nodeID)
	if err != nil {
		resp := GetResponse{
			Success:  false,
			Message:  fmt.Sprintf("Key not found: %v", err),
			KeyHash:  keyHashHex,
			HopCount: hopCount,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Success response
	resp := GetResponse{
		Success:  true,
		KeyHash:  keyHashHex,
		Value:    string(value),
		HopCount: hopCount,
	}

	fmt.Printf("[HTTP-API] ✓ Value found in %d hops\n", hopCount)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleStatus returns information about the node
func (s *HTTPServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Count stored keys
	s.Node.StorageMux.RLock()
	storedKeys := len(s.Node.Storage)
	s.Node.StorageMux.RUnlock()

	// Count known peers (approximate - count non-empty buckets)
	knownPeers := 0
	for i := 0; i < 256; i++ {
		knownPeers += s.Node.RoutingTable.Buckets[i].Len()
	}

	resp := StatusResponse{
		NodeID:        s.Node.Self.ID.String(), /*[:16] + "..."*/
		IP:            s.Node.Self.IP,
		Port:          s.Node.Self.Port,
		StoredKeys:    storedKeys,
		KnownPeers:    knownPeers,
		NetworkStatus: "connected",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleHealth is a simple health check endpoint
func (s *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	})
}

// The Handler
func (s *HTTPServer) handleRoutingTable(w http.ResponseWriter, r *http.Request) {
	// Enable CORS if running frontend separately
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	tableInfo := s.Node.GetRoutingTableInfo()
	json.NewEncoder(w).Encode(tableInfo)
}
