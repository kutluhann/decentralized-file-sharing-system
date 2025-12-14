package dht

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/kutluhann/decentralized-file-sharing-system/constants"
	"github.com/kutluhann/decentralized-file-sharing-system/id_tools"
	"github.com/kutluhann/decentralized-file-sharing-system/pos"
)

type Contact struct {
	ID       NodeID
	IP       string
	Port     int
	LastSeen time.Time
}

// Challenge tracking for join handshake
type PendingChallenge struct {
	Nonce     string
	Timestamp time.Time
	PubKey    []byte
}

// ReplicationTimer tracks the ticker and cancel channel for a key's replication
type ReplicationTimer struct {
	Ticker *time.Ticker
	Stop   chan bool
}

type Node struct {
	Self              Contact
	RoutingTable      *RoutingTable
	Storage           map[NodeID][]byte // Local key-value storage
	StorageMux        sync.RWMutex      // Mutex for thread-safe storage access
	PrivKey           *ecdsa.PrivateKey
	Network           *Network
	PendingChallenges map[NodeID]PendingChallenge // For server side: track challenges sent to peers
	ChallengeMutex    sync.RWMutex
	ReplicationTimers map[NodeID]*ReplicationTimer // Timers for periodic re-replication of stored keys
	TimerMutex        sync.RWMutex                 // Mutex for thread-safe timer access
	PosPlot           *pos.Plot                    // Proof of Space plot for Sybil resistance
}

// CreateNode initializes the DHT node using the identity from config.
func NewNode(contact Contact, privateKey *ecdsa.PrivateKey) *Node {
	return &Node{
		Self:              contact,
		RoutingTable:      NewRoutingTable(contact),
		Storage:           make(map[NodeID][]byte), // Initialize storage map
		PrivKey:           privateKey,
		PendingChallenges: make(map[NodeID]PendingChallenge),
		ReplicationTimers: make(map[NodeID]*ReplicationTimer), // Initialize replication timers map
	}
}

// JoinNetwork initiates the bootstrap process with full handshake
// Returns the bootstrap node's Contact info on success
func (n *Node) JoinNetwork(bootstrapAddr string) (Contact, error) {
	fmt.Printf("[JOIN] Step 1/4: Sending JOIN_REQ to %s...\n", bootstrapAddr)

	// Step 1: Send JOIN_REQ with our PeerID and PublicKey
	pubKeyBytes, _ := x509.MarshalPKIXPublicKey(&n.PrivKey.PublicKey)
	rpcID := id_tools.GenerateSecureRandomMessage()

	joinReq := Message{
		Type:     JOIN_REQ,
		RPCID:    rpcID,
		SenderID: n.Self.ID,
		Payload: JoinRequestPayload{
			PeerID:    n.Self.ID,
			PublicKey: pubKeyBytes,
		},
	}

	// Create response channel and register it
	respChan := make(chan Message, 1)
	n.Network.RegisterResponseChannel(rpcID, respChan)
	defer n.Network.UnregisterResponseChannel(rpcID)

	// Send the request
	err := n.Network.SendMessage(joinReq, bootstrapAddr)
	if err != nil {
		return Contact{}, fmt.Errorf("failed to send JOIN_REQ: %v", err)
	}

	// Step 2: Wait for JOIN_CHALLENGE (10 seconds timeout)
	var bootstrapContact Contact

	select {
	case challengeMsg := <-respChan:
		if challengeMsg.Type != JOIN_CHALLENGE {
			return Contact{}, fmt.Errorf("expected JOIN_CHALLENGE, got %v", challengeMsg.Type)
		}

		fmt.Printf("[JOIN] Step 2/4: Received JOIN_CHALLENGE from %s\n", challengeMsg.SenderID.String()[:16])

		// Save bootstrap node info
		host, portStr, _ := net.SplitHostPort(bootstrapAddr)
		port, _ := strconv.Atoi(portStr)
		bootstrapContact = Contact{
			ID:       challengeMsg.SenderID,
			IP:       host,
			Port:     port,
			LastSeen: time.Now(),
		}

		// Extract challenge
		payloadBytes, _ := json.Marshal(challengeMsg.Payload)
		var challenge JoinChallengePayload
		json.Unmarshal(payloadBytes, &challenge)

		// Step 3: Sign the challenge
		fmt.Printf("[JOIN] Step 3/4: Signing challenge nonce...\n")
		signature := id_tools.SignMessage(*n.PrivKey, challenge.Nonce)

		// Send JOIN_RES with signature
		joinRes := Message{
			Type:     JOIN_RES,
			RPCID:    id_tools.GenerateSecureRandomMessage(),
			SenderID: n.Self.ID,
			Payload: JoinResponsePayload{
				Signature: signature,
			},
		}

		// Register new response channel for ACK
		ackRPCID := joinRes.RPCID
		ackChan := make(chan Message, 1)
		n.Network.RegisterResponseChannel(ackRPCID, ackChan)
		defer n.Network.UnregisterResponseChannel(ackRPCID)

		err := n.Network.SendMessage(joinRes, bootstrapAddr)
		if err != nil {
			return Contact{}, fmt.Errorf("failed to send JOIN_RES: %v", err)
		}

		// Step 4: Wait for POS_CHALLENGE
		select {
		case posMsg := <-ackChan:
			if posMsg.Type == JOIN_ACK {
				// Old flow - no PoS required, check if successful
				payloadBytes, _ := json.Marshal(posMsg.Payload)
				var ack JoinAckPayload
				json.Unmarshal(payloadBytes, &ack)
				if ack.Success {
					fmt.Printf("[JOIN] Step 4/4: ✓ Successfully joined network! Message: %s\n", ack.Message)
					return bootstrapContact, nil
				} else {
					return Contact{}, fmt.Errorf("[JOIN] Step 4/4: ✗ Join rejected: %s", ack.Message)
				}
			}
			
			if posMsg.Type != POS_CHALLENGE {
				return Contact{}, fmt.Errorf("expected POS_CHALLENGE or JOIN_ACK, got %v", posMsg.Type)
			}

			fmt.Printf("[JOIN] Step 4/6: Received POS_CHALLENGE from %s\n", posMsg.SenderID.String()[:16])

			// Extract PoS challenge
			payloadBytes, _ := json.Marshal(posMsg.Payload)
			var posChallenge PosChallengePayload
			json.Unmarshal(payloadBytes, &posChallenge)

			// Step 5: Generate PoS proof
			fmt.Printf("[JOIN] Step 5/6: Generating Proof of Space...\n")
			posProof, err := n.GeneratePosProof(&posChallenge)
			if err != nil {
				return Contact{}, fmt.Errorf("failed to generate PoS proof: %v", err)
			}

			// Send POS_PROOF
			posProofMsg := Message{
				Type:     POS_PROOF,
				RPCID:    id_tools.GenerateSecureRandomMessage(),
				SenderID: n.Self.ID,
				Payload:  *posProof,
			}

			// Register new response channel for final ACK
			finalRPCID := posProofMsg.RPCID
			finalChan := make(chan Message, 1)
			n.Network.RegisterResponseChannel(finalRPCID, finalChan)
			defer n.Network.UnregisterResponseChannel(finalRPCID)

			err = n.Network.SendMessage(posProofMsg, bootstrapAddr)
			if err != nil {
				return Contact{}, fmt.Errorf("failed to send POS_PROOF: %v", err)
			}

			// Step 6: Wait for final JOIN_ACK
			select {
			case ackMsg := <-finalChan:
				if ackMsg.Type != JOIN_ACK {
					return Contact{}, fmt.Errorf("expected JOIN_ACK, got %v", ackMsg.Type)
				}

				payloadBytes, _ := json.Marshal(ackMsg.Payload)
				var ack JoinAckPayload
				json.Unmarshal(payloadBytes, &ack)

				if ack.Success {
					fmt.Printf("[JOIN] Step 6/6: ✓ Successfully joined network! Message: %s\n", ack.Message)
					return bootstrapContact, nil
				} else {
					return Contact{}, fmt.Errorf("[JOIN] Step 6/6: ✗ Join rejected: %s", ack.Message)
				}

			case <-time.After(10 * time.Second):
				return Contact{}, fmt.Errorf("[JOIN] timeout waiting for final JOIN_ACK")
			}

		case <-time.After(10 * time.Second):
			return Contact{}, fmt.Errorf("[JOIN] timeout waiting for POS_CHALLENGE")
		}

	case <-time.After(10 * time.Second):
		return Contact{}, fmt.Errorf("[JOIN] timeout waiting for JOIN_CHALLENGE")
	}
}

// ---------------------------------------------------------
// SERVER HANDLERS (Implements MessageHandler Interface)
// ---------------------------------------------------------

func (n *Node) HandleFindNode(sender Contact, targetID NodeID) []Contact {
	n.RoutingTable.Update(sender)

	// Get closest nodes from routing table
	allNodes := n.RoutingTable.GetClosestNodes(targetID, 20)

	// Filter out the sender (they already know about themselves)
	var nodes []Contact
	for _, node := range allNodes {
		if node.ID != sender.ID {
			nodes = append(nodes, node)
			fmt.Printf("[SERVER] HandleFindNode: returning %s\n", node.ID.String()[:16])
		} else {
			fmt.Printf("[SERVER] HandleFindNode: skipping sender %s\n", sender.ID.String()[:16])
		}
	}

	fmt.Printf("[SERVER] HandleFindNode: returning %d nodes (filtered from %d)\n", len(nodes), len(allNodes))
	return nodes
}

func (n *Node) HandlePing(sender Contact) {
	n.RoutingTable.Update(sender)
}

func (n *Node) HandleStore(sender Contact, key NodeID, value []byte) {
	n.RoutingTable.Update(sender)

	// Actually store the data in local storage
	n.StorageMux.Lock()
	n.Storage[key] = value
	n.StorageMux.Unlock()

	fmt.Printf("[SERVER] ✓ Stored %d bytes for key %s (from %s)\n",
		len(value), key.String()[:16], sender.ID.String()[:16])

	// Start or restart the replication timer for this key
	n.startReplicationTimer(key, value)
}

// startReplicationTimer starts or restarts a recurring timer for re-replicating a key-value pair
func (n *Node) startReplicationTimer(key NodeID, value []byte) {
	n.TimerMutex.Lock()
	defer n.TimerMutex.Unlock()

	// Stop existing timer if one exists for this key
	if existingTimer, exists := n.ReplicationTimers[key]; exists {
		existingTimer.Ticker.Stop()
		close(existingTimer.Stop)
		fmt.Printf("[TIMER] Stopped existing replication timer for key %s\n", key.String()[:16])
	}

	// Create a new recurring timer using a ticker
	ticker := time.NewTicker(10 * time.Minute)
	stopChan := make(chan bool)

	// Store the timer tracking structure
	n.ReplicationTimers[key] = &ReplicationTimer{
		Ticker: ticker,
		Stop:   stopChan,
	}

	// Start the replication goroutine
	go func(k NodeID) {
		for {
			select {
			case <-ticker.C:
				// Get the current value from storage (it might have been updated)
				n.StorageMux.RLock()
				currentValue, exists := n.Storage[k]
				n.StorageMux.RUnlock()

				if !exists {
					// Key was deleted, stop the ticker
					ticker.Stop()
					n.TimerMutex.Lock()
					delete(n.ReplicationTimers, k)
					n.TimerMutex.Unlock()
					fmt.Printf("[TIMER] Key %s no longer in storage, stopping replication\n", k.String()[:16])
					return
				}

				fmt.Printf("[TIMER] Replication timer triggered for key %s, re-storing to network...\n", k.String()[:16])
				// Call the Store function which will send STORE messages to k closest nodes
				n.Store(k, currentValue)

			case <-stopChan:
				// Received stop signal
				ticker.Stop()
				fmt.Printf("[TIMER] Replication timer stopped for key %s\n", k.String()[:16])
				return
			}
		}
	}(key)

	fmt.Printf("[TIMER] Started replication timer for key %s (will trigger every 10 minutes)\n", key.String()[:16])
}

func (n *Node) HandleFindValue(sender Contact, key NodeID) ([]byte, []Contact) {
	n.RoutingTable.Update(sender)

	// Check if we have the value locally
	n.StorageMux.RLock()
	value, exists := n.Storage[key]
	n.StorageMux.RUnlock()

	if exists {
		fmt.Printf("[SERVER] ✓ Found value for key %s (returning %d bytes to %s)\n",
			key.String()[:16], len(value), sender.ID.String()[:16])
		return value, nil // Return the value, no contacts needed
	}

	// Don't have it - return closest nodes who might have it
	fmt.Printf("[SERVER] ✗ Key %s not found locally, returning closest nodes to %s\n",
		key.String()[:16], sender.ID.String()[:16])
	return nil, n.RoutingTable.GetClosestNodes(key, 20)
}

// BucketInfo represents a single bucket for JSON output
type BucketInfo struct {
	Index    int       `json:"index"`
	Contacts []Contact `json:"contacts"`
}

// GetRoutingTableInfo returns a snapshot of all non-empty buckets
func (n *Node) GetRoutingTableInfo() []BucketInfo {
	n.RoutingTable.mutex.RLock() // Use the public field 'mutex' directly if exposed, or add getter
	defer n.RoutingTable.mutex.RUnlock()

	var info []BucketInfo

	for i, bucket := range n.RoutingTable.Buckets {
		if bucket.Len() > 0 {
			contacts := bucket.GetContacts()
			info = append(info, BucketInfo{
				Index:    i,
				Contacts: contacts,
			})
		}
	}
	return info
}

// --- Handshake Handlers ---

// HandleJoinRequest is called by the server node when a new node wants to join
func (n *Node) HandleJoinRequest(sender Contact, payload JoinRequestPayload) (JoinChallengePayload, error) {
	fmt.Printf("[SERVER] Received JOIN_REQ from %s\n", payload.PeerID.String()[:16])

	// 1. Verify PubKey -> PeerID match (Sybil attack prevention)
	pubKey, err := x509.ParsePKIXPublicKey(payload.PublicKey)
	if err != nil {
		fmt.Printf("[SERVER] ✗ Invalid public key format from %s\n", payload.PeerID.String()[:16])
		return JoinChallengePayload{}, fmt.Errorf("invalid public key format")
	}

	ecdsaPubKey, ok := pubKey.(*ecdsa.PublicKey)
	if !ok {
		fmt.Printf("[SERVER] ✗ Public key is not ECDSA from %s\n", payload.PeerID.String()[:16])
		return JoinChallengePayload{}, fmt.Errorf("public key is not ECDSA")
	}

	// Critical check: Does the public key actually generate this PeerID?
	if !id_tools.CheckPublicKeyMatchesPeerID(ecdsaPubKey, id_tools.PeerID(payload.PeerID)) {
		fmt.Printf("[SERVER] ✗ SYBIL ATTACK DETECTED: PubKey doesn't match PeerID from %s\n", payload.PeerID.String()[:16])
		return JoinChallengePayload{}, fmt.Errorf("public key does not match PeerID - potential sybil attack")
	}

	fmt.Printf("[SERVER] ✓ PeerID verification passed\n")

	// 2. Generate Challenge (random nonce for signature verification)
	nonce := id_tools.GenerateSecureRandomMessage()

	// 3. Store the challenge for later verification (with 10 second expiry)
	n.ChallengeMutex.Lock()
	n.PendingChallenges[payload.PeerID] = PendingChallenge{
		Nonce:     nonce,
		Timestamp: time.Now(),
		PubKey:    payload.PublicKey,
	}
	n.ChallengeMutex.Unlock()

	fmt.Printf("[SERVER] Sending challenge nonce to %s (expires in 10s)\n", payload.PeerID.String()[:16])

	return JoinChallengePayload{Nonce: nonce}, nil
}

// HandleJoinResponse is called by server node when new node sends signature
func (n *Node) HandleJoinResponse(sender Contact, payload JoinResponsePayload) (JoinAckPayload, error) {
	fmt.Printf("[SERVER] Received JOIN_RES (signature) from %s\n", sender.ID.String()[:16])

	// 1. Retrieve the pending challenge
	n.ChallengeMutex.RLock()
	challenge, exists := n.PendingChallenges[sender.ID]
	n.ChallengeMutex.RUnlock()

	if !exists {
		fmt.Printf("[SERVER] ✗ No pending challenge for %s (may have expired)\n", sender.ID.String()[:16])
		return JoinAckPayload{Success: false, Message: "No pending challenge found"}, fmt.Errorf("no pending challenge")
	}

	// 2. Check if challenge expired (10 seconds timeout)
	if time.Since(challenge.Timestamp) > 10*time.Second {
		n.ChallengeMutex.Lock()
		delete(n.PendingChallenges, sender.ID)
		n.ChallengeMutex.Unlock()

		fmt.Printf("[SERVER] ✗ Challenge expired for %s\n", sender.ID.String()[:16])
		return JoinAckPayload{Success: false, Message: "Challenge expired"}, fmt.Errorf("challenge expired")
	}

	// 3. Parse the public key
	pubKey, err := x509.ParsePKIXPublicKey(challenge.PubKey)
	if err != nil {
		fmt.Printf("[SERVER] ✗ Failed to parse public key\n")
		return JoinAckPayload{Success: false, Message: "Invalid public key"}, fmt.Errorf("invalid public key")
	}

	ecdsaPubKey, ok := pubKey.(*ecdsa.PublicKey)
	if !ok {
		fmt.Printf("[SERVER] ✗ Public key is not ECDSA\n")
		return JoinAckPayload{Success: false, Message: "Invalid key type"}, fmt.Errorf("invalid key type")
	}

	// 4. Verify the signature
	if !id_tools.VerifySignature(*ecdsaPubKey, challenge.Nonce, payload.Signature) {
		fmt.Printf("[SERVER] ✗ Signature verification FAILED for %s\n", sender.ID.String()[:16])

		// Clean up
		n.ChallengeMutex.Lock()
		delete(n.PendingChallenges, sender.ID)
		n.ChallengeMutex.Unlock()

		return JoinAckPayload{Success: false, Message: "Invalid signature"}, fmt.Errorf("invalid signature")
	}

	fmt.Printf("[SERVER] ✓ Signature verification PASSED\n")

	// 5. Success! Add peer to routing table
	n.RoutingTable.Update(sender)

	// Clean up challenge
	n.ChallengeMutex.Lock()
	delete(n.PendingChallenges, sender.ID)
	n.ChallengeMutex.Unlock()

	fmt.Printf("[SERVER] ✓ Peer %s successfully joined and added to DHT!\n", sender.ID.String()[:16])

	return JoinAckPayload{Success: true, Message: "Welcome to the DHT network!"}, nil
}

// ---------------------------------------------------------
// CLIENT-SIDE DHT OPERATIONS (Store & Retrieve)
// ---------------------------------------------------------

// Store stores a key-value pair in the DHT by replicating it to K closest nodes
func (n *Node) Store(key NodeID, value []byte) error {
	fmt.Printf("[DHT-STORE] Storing key %s (%d bytes)...\n", key.String()[:16], len(value))

	// 1. Find K closest nodes to this key using NodeLookup
	closestNodes, _ := n.NodeLookup(key)

	if len(closestNodes) == 0 {
		fmt.Printf("[DHT-STORE] ✗ No nodes found in network, storing only locally\n")
		// Store locally at least
		n.StorageMux.Lock()
		n.Storage[key] = value
		n.StorageMux.Unlock()
		return fmt.Errorf("no nodes available for replication")
	}

	// 2. Replicate to all K closest nodes
	successCount := 0
	for _, contact := range closestNodes {
		// Don't send to ourselves
		if contact.ID == n.Self.ID {
			continue
		}

		fmt.Printf("[DHT-STORE] Replicating to node %s at %s:%d\n",
			contact.ID.String()[:16], contact.IP, contact.Port)

		err := n.Network.SendStore(contact, key, value)
		if err == nil {
			successCount++
			fmt.Printf("[DHT-STORE] ✓ Successfully replicated to %s\n", contact.ID.String()[:16])
		} else {
			fmt.Printf("[DHT-STORE] ✗ Failed to replicate to %s: %v\n", contact.ID.String()[:16], err)
		}
	}

	// 3. Also store locally (we might be one of the closest nodes)
	n.StorageMux.Lock()
	n.Storage[key] = value
	n.StorageMux.Unlock()
	fmt.Printf("[DHT-STORE] ✓ Stored locally\n")

	// Start replication timer for this key
	n.startReplicationTimer(key, value)

	fmt.Printf("[DHT-STORE] ✓ Complete: stored at %d remote nodes + local = %d total locations\n",
		successCount, successCount+1)

	return nil
}

// FindValue retrieves a value from the DHT using Kademlia iterative lookup
// Returns: value, hopCount, error
func (n *Node) FindValue(key NodeID) ([]byte, int, error) {
	fmt.Printf("[DHT-FIND] Searching for key %s...\n", key.String()[:16])

	// 1. Check locally first (hop count = 0)
	n.StorageMux.RLock()
	value, exists := n.Storage[key]
	n.StorageMux.RUnlock()

	if exists {
		fmt.Printf("[DHT-FIND] ✓ Found locally (%d bytes)\n", len(value))
		return value, 0, nil
	}

	fmt.Printf("[DHT-FIND] Not found locally, starting iterative FIND_VALUE lookup...\n")

	// 2. Initialize lookup state with closest known nodes
	localCandidates := n.RoutingTable.GetClosestNodes(key, 20) // K=20
	if len(localCandidates) == 0 {
		return nil, 0, fmt.Errorf("key not found: no nodes in network")
	}

	state := NewLookupState(key, localCandidates)
	hopCount := 0

	// 3. ITERATIVE FIND_VALUE LOOP (Kademlia protocol)
	// Unlike NodeLookup which uses FIND_NODE, this uses FIND_VALUE
	for {
		// A. Pick the next closest unqueried node
		candidate := state.PickNextBest()

		// TERMINATION: No more nodes to query
		if candidate == nil {
			fmt.Printf("[DHT-FIND] ✗ No more nodes to query, key not found (hops: %d)\n", hopCount)
			break
		}

		// B. Send FIND_VALUE RPC
		fmt.Printf("[DHT-FIND] [Hop %d] Querying node %s at %s:%d\n",
			hopCount+1, candidate.ID.String()[:16], candidate.IP, candidate.Port)

		hopCount++
		value, nodes, err := n.Network.SendFindValue(*candidate, key)

		// Mark as contacted to avoid re-querying
		state.MarkContacted(candidate.ID)

		// C. Handle errors
		if err != nil {
			fmt.Printf("[DHT-FIND] ✗ Failed to query %s: %v\n", candidate.ID.String()[:16], err)
			continue
		}

		// D. VALUE FOUND! (success case)
		if value != nil {
			fmt.Printf("[DHT-FIND] ✓ Found value at node %s (%d bytes) [hops: %d]\n",
				candidate.ID.String()[:16], len(value), hopCount)

			// Cache locally for future lookups
			// n.StorageMux.Lock()
			// n.Storage[key] = value
			// n.StorageMux.Unlock()

			return value, hopCount, nil
		}

		// E. VALUE NOT FOUND, but got closer nodes
		// Add returned nodes to shortlist and continue iteration
		if len(nodes) > 0 {
			fmt.Printf("[DHT-FIND] Node %s doesn't have key, returned %d closer nodes\n",
				candidate.ID.String()[:16], len(nodes))
			state.Append(nodes)
		} else {
			fmt.Printf("[DHT-FIND] Node %s doesn't have key, no new nodes returned\n",
				candidate.ID.String()[:16])
		}

		// Update routing table (node is alive)
		n.RoutingTable.Update(*candidate)
	}

	// 4. Key not found after exhausting all nodes
	return nil, hopCount, fmt.Errorf("key not found in DHT")
}

// ---------------------------------------------------------
// PROOF OF SPACE METHODS
// ---------------------------------------------------------

// InitializePosPlot generates or loads a PoS plot for this node
func (n *Node) InitializePosPlot() error {
	fmt.Printf("[PoS] Initializing Proof of Space plot...\n")
	
	plot, err := pos.GeneratePlot(
		id_tools.PeerID(n.Self.ID),
		constants.PlotSize,
		constants.PlotDataDir,
	)
	if err != nil {
		return fmt.Errorf("failed to generate PoS plot: %w", err)
	}
	
	n.PosPlot = plot
	fmt.Printf("[PoS] ✓ Plot initialized successfully\n")
	return nil
}

// GeneratePosProof creates a PoS proof for a given challenge
func (n *Node) GeneratePosProof(challenge *PosChallengePayload) (*PosProofPayload, error) {
	if n.PosPlot == nil {
		return nil, fmt.Errorf("PoS plot not initialized")
	}
	
	posChallenge := &pos.Challenge{
		Value:      challenge.ChallengeValue,
		StartIndex: challenge.StartIndex,
		EndIndex:   challenge.EndIndex,
		Required:   challenge.Required,
	}
	
	proof, err := n.PosPlot.GenerateProof(posChallenge)
	if err != nil {
		return nil, fmt.Errorf("failed to generate PoS proof: %w", err)
	}
	
	// Convert proof elements to payload format
	proofElements := make([]PosProofElement, len(proof.ProofChain))
	for i, elem := range proof.ProofChain {
		proofElements[i] = PosProofElement{
			Layer:       elem.Layer,
			Index:       elem.Index,
			Value:       elem.Value,
			ParentLeft:  elem.ParentLeft,
			ParentRight: elem.ParentRight,
		}
	}
	
	return &PosProofPayload{
		ChallengeValue: proof.Challenge,
		StartIndex:     challenge.StartIndex,
		EndIndex:       challenge.EndIndex,
		Required:       challenge.Required,
		ProofChain:     proofElements,
	}, nil
}

// HandlePosChallenge is called by server to create a PoS challenge for joining node
func (n *Node) HandlePosChallenge(sender Contact) (*PosChallengePayload, error) {
	fmt.Printf("[SERVER] Creating PoS challenge for %s\n", sender.ID.String()[:16])
	
	challenge, err := pos.GenerateChallenge(constants.PlotSize)
	if err != nil {
		return nil, fmt.Errorf("failed to generate PoS challenge: %w", err)
	}
	
	// Store challenge for verification (reuse PendingChallenges map)
	n.ChallengeMutex.Lock()
	if existing, exists := n.PendingChallenges[sender.ID]; exists {
		// Update with PoS challenge data (store in a way that we can verify later)
		existing.Timestamp = time.Now()
		n.PendingChallenges[sender.ID] = existing
	}
	n.ChallengeMutex.Unlock()
	
	return &PosChallengePayload{
		ChallengeValue: challenge.Value,
		StartIndex:     challenge.StartIndex,
		EndIndex:       challenge.EndIndex,
		Required:       challenge.Required,
	}, nil
}

// HandlePosProof is called by server to verify PoS proof from joining node
func (n *Node) HandlePosProof(sender Contact, payload PosProofPayload) (JoinAckPayload, error) {
	fmt.Printf("[SERVER] Received PoS proof from %s (chain length: %d)\n", sender.ID.String()[:16], len(payload.ProofChain))
	
	// Recreate challenge from payload
	challenge := &pos.Challenge{
		Value:      payload.ChallengeValue,
		StartIndex: payload.StartIndex,
		EndIndex:   payload.EndIndex,
		Required:   payload.Required,
	}
	
	// Convert payload proof elements back to pos.ProofElement
	proofChain := make([]pos.ProofElement, len(payload.ProofChain))
	for i, elem := range payload.ProofChain {
		proofChain[i] = pos.ProofElement{
			Layer:       elem.Layer,
			Index:       elem.Index,
			Value:       elem.Value,
			ParentLeft:  elem.ParentLeft,
			ParentRight: elem.ParentRight,
		}
	}
	
	proof := &pos.Proof{
		Challenge:  payload.ChallengeValue,
		ProofChain: proofChain,
	}
	
	// Verify the proof - this checks the entire dependency chain
	if !pos.VerifyProof(id_tools.PeerID(sender.ID), challenge, proof) {
		fmt.Printf("[SERVER] ✗ PoS verification FAILED for %s - invalid dependency chain!\n", sender.ID.String()[:16])
		
		// Clean up
		n.ChallengeMutex.Lock()
		delete(n.PendingChallenges, sender.ID)
		n.ChallengeMutex.Unlock()
		
		return JoinAckPayload{Success: false, Message: "PoS verification failed - invalid proof chain"}, fmt.Errorf("PoS verification failed")
	}
	
	fmt.Printf("[SERVER] ✓ PoS verification PASSED for %s - valid dependency chain confirmed\n", sender.ID.String()[:16])
	
	// Add to routing table
	n.RoutingTable.Update(sender)
	
	// Clean up challenge
	n.ChallengeMutex.Lock()
	delete(n.PendingChallenges, sender.ID)
	n.ChallengeMutex.Unlock()
	
	fmt.Printf("[SERVER] ✓ Peer %s successfully joined with PoS verification!\n", sender.ID.String()[:16])
	
	return JoinAckPayload{Success: true, Message: "Welcome to the DHT network (PoS verified with layered proof)!"}, nil
}
