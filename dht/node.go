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

	"github.com/kutluhann/decentralized-file-sharing-system/id_tools"
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

type Node struct {
	Self         Contact
	RoutingTable *RoutingTable
	// Storage    map[NodeID][]byte
	// StorageMux sync.RWMutex
	PrivKey           *ecdsa.PrivateKey
	Network           *Network
	PendingChallenges map[NodeID]PendingChallenge // For server side: track challenges sent to peers
	ChallengeMutex    sync.RWMutex
}

// CreateNode initializes the DHT node using the identity from config.
func NewNode(contact Contact, privateKey *ecdsa.PrivateKey) *Node {
	return &Node{
		Self:              contact,
		RoutingTable:      NewRoutingTable(contact),
		PrivKey:           privateKey,
		PendingChallenges: make(map[NodeID]PendingChallenge),
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

		// Step 4: Wait for JOIN_ACK
		select {
		case ackMsg := <-ackChan:
			if ackMsg.Type != JOIN_ACK {
				return Contact{}, fmt.Errorf("expected JOIN_ACK, got %v", ackMsg.Type)
			}

			payloadBytes, _ := json.Marshal(ackMsg.Payload)
			var ack JoinAckPayload
			json.Unmarshal(payloadBytes, &ack)

			if ack.Success {
				fmt.Printf("[JOIN] Step 4/4: ✓ Successfully joined network! Message: %s\n", ack.Message)
				return bootstrapContact, nil
			} else {
				return Contact{}, fmt.Errorf("[JOIN] Step 4/4: ✗ Join rejected: %s", ack.Message)
			}

		case <-time.After(10 * time.Second):
			return Contact{}, fmt.Errorf("[JOIN] timeout waiting for JOIN_ACK")
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
	// Store logic here
}

func (n *Node) HandleFindValue(sender Contact, key NodeID) ([]byte, []Contact) {
	n.RoutingTable.Update(sender)
	// Value logic here
	return nil, n.RoutingTable.GetClosestNodes(key, 20)
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
