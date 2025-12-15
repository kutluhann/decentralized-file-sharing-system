package dht

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

type MessageHandler interface {
	HandlePing(sender Contact)
	HandleFindNode(sender Contact, targetID NodeID) []Contact
	HandleStore(sender Contact, key NodeID, value []byte)
	HandleFindValue(sender Contact, key NodeID) ([]byte, []Contact)

	// Handshake
	HandleJoinRequest(sender Contact, payload JoinRequestPayload) (JoinChallengePayload, error)
	HandleJoinResponse(sender Contact, payload JoinResponsePayload) (JoinAckPayload, error)
}

type Network struct {
	Conn             *net.UDPConn
	Handler          MessageHandler
	SelfID           NodeID
	ResponseChannels map[string]chan Message // RPCID -> Response Channel
	ResponseMutex    sync.RWMutex
}

func NewNetwork(address string, selfID NodeID) (*Network, error) {
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}

	return &Network{
		Conn:             conn,
		SelfID:           selfID,
		ResponseChannels: make(map[string]chan Message),
	}, nil
}

// RegisterResponseChannel registers a channel to receive response for a specific RPC ID
func (s *Network) RegisterResponseChannel(rpcID string, ch chan Message) {
	s.ResponseMutex.Lock()
	defer s.ResponseMutex.Unlock()
	s.ResponseChannels[rpcID] = ch
}

// UnregisterResponseChannel removes the response channel for an RPC ID
func (s *Network) UnregisterResponseChannel(rpcID string) {
	s.ResponseMutex.Lock()
	defer s.ResponseMutex.Unlock()
	delete(s.ResponseChannels, rpcID)
}

func (s *Network) SetHandler(h MessageHandler) {
	s.Handler = h
}

func (s *Network) Listen() {
	fmt.Println("Listening for UDP packets on", s.Conn.LocalAddr().String())
	buf := make([]byte, 65535) // buffer size is increased to maximum to avoid network failures

	for {
		n, remoteAddr, err := s.Conn.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("Error reading from UDP:", err)
			continue
		}

		packetData := make([]byte, n)
		copy(packetData, buf[:n])

		go s.handlePacket(packetData, remoteAddr)
	}
}

func (s *Network) handlePacket(data []byte, addr *net.UDPAddr) {
	var msg Message
	err := json.Unmarshal(data, &msg)
	if err != nil {
		fmt.Println("JSON decode error:", err)
		return
	}

	sender := Contact{
		ID:   msg.SenderID,
		IP:   addr.IP.String(),
		Port: addr.Port,
	}

	// Check if this is a response to a pending RPC call (client-side handling)
	isResponse := msg.Type == PING_RES || msg.Type == FIND_NODE_RES ||
		msg.Type == FIND_VALUE_RES || msg.Type == STORE_RES ||
		msg.Type == JOIN_CHALLENGE || msg.Type == JOIN_ACK ||
		msg.Type == POS_CHALLENGE

	if isResponse {
		// This is a response - route it to the waiting channel
		s.ResponseMutex.RLock()
		ch, exists := s.ResponseChannels[msg.RPCID]
		s.ResponseMutex.RUnlock()

		if exists {
			select {
			case ch <- msg:
				// Successfully delivered response
			default:
				fmt.Println("Warning: Response channel full, dropping message")
			}
		} else {
			fmt.Printf("Warning: No response channel for RPCID %s (may have timed out)\n", msg.RPCID)
		}
		return
	}

	// This is a request - handle it with the handler (server-side handling)
	if s.Handler == nil {
		fmt.Println("Warning: No message handler set, dropping packet.")
		return
	}

	switch msg.Type {
	case PING:
		s.Handler.HandlePing(sender)
		s.sendResponse(msg.RPCID, PING_RES, PingResponse{Timestamp: 0}, addr)

	case FIND_NODE:
		payloadBytes, _ := json.Marshal(msg.Payload)
		var req FindNodeRequest
		json.Unmarshal(payloadBytes, &req)

		nodes := s.Handler.HandleFindNode(sender, req.TargetID)
		s.sendResponse(msg.RPCID, FIND_NODE_RES, FindNodeResponse{Nodes: nodes}, addr)

	case STORE:
		payloadBytes, _ := json.Marshal(msg.Payload)
		var req StoreRequest
		json.Unmarshal(payloadBytes, &req)

		s.Handler.HandleStore(sender, req.Key, req.Value)
		s.sendResponse(msg.RPCID, STORE_RES, StoreResponse{Success: true}, addr)

	case FIND_VALUE:
		payloadBytes, _ := json.Marshal(msg.Payload)
		var req FindValueRequest
		json.Unmarshal(payloadBytes, &req)

		val, nodes := s.Handler.HandleFindValue(sender, req.Key)
		res := FindValueResponse{
			Found: val != nil,
			Value: val,
			Nodes: nodes,
		}
		s.sendResponse(msg.RPCID, FIND_VALUE_RES, res, addr)

	// --- Secure Join Handshake (Server-Side) ---

	case JOIN_REQ:
		payloadBytes, _ := json.Marshal(msg.Payload)
		var req JoinRequestPayload
		json.Unmarshal(payloadBytes, &req)

		challenge, err := s.Handler.HandleJoinRequest(sender, req)
		if err != nil {
			fmt.Println("[SERVER] Join Request rejected:", err)
			return
		}
		s.sendResponse(msg.RPCID, JOIN_CHALLENGE, challenge, addr)

	case JOIN_RES:
		payloadBytes, _ := json.Marshal(msg.Payload)
		var req JoinResponsePayload
		json.Unmarshal(payloadBytes, &req)

		// After signature verification, send PoS challenge
		_, err := s.Handler.HandleJoinResponse(sender, req)
		if err != nil {
			s.sendResponse(msg.RPCID, JOIN_ACK, JoinAckPayload{Success: false, Message: err.Error()}, addr)
			return
		}

		// Signature verified, now send PoS challenge
		if handler, ok := s.Handler.(interface {
			HandlePosChallenge(sender Contact) (*PosChallengePayload, error)
		}); ok {
			posChallenge, err := handler.HandlePosChallenge(sender)
			if err != nil {
				s.sendResponse(msg.RPCID, JOIN_ACK, JoinAckPayload{Success: false, Message: "PoS challenge failed"}, addr)
				return
			}
			s.sendResponse(msg.RPCID, POS_CHALLENGE, *posChallenge, addr)
		} else {
			// Fallback: no PoS support, just approve
			s.sendResponse(msg.RPCID, JOIN_ACK, JoinAckPayload{Success: true, Message: "Welcome to the DHT network!"}, addr)
		}

	case POS_PROOF:
		payloadBytes, _ := json.Marshal(msg.Payload)
		var proof PosProofPayload
		json.Unmarshal(payloadBytes, &proof)

		if handler, ok := s.Handler.(interface {
			HandlePosProof(sender Contact, payload PosProofPayload) (JoinAckPayload, error)
		}); ok {
			ack, err := handler.HandlePosProof(sender, proof)
			if err != nil {
				s.sendResponse(msg.RPCID, JOIN_ACK, JoinAckPayload{Success: false, Message: err.Error()}, addr)
				return
			}
			s.sendResponse(msg.RPCID, JOIN_ACK, ack, addr)
		} else {
			s.sendResponse(msg.RPCID, JOIN_ACK, JoinAckPayload{Success: false, Message: "PoS not supported"}, addr)
		}
	}
}

func (s *Network) sendResponse(rpcID string, msgType MessageType, payload interface{}, addr *net.UDPAddr) {
	resp := Message{
		Type:     msgType,
		RPCID:    rpcID,
		SenderID: s.SelfID,
		Payload:  payload,
	}
	s.SendMessageToUDPAddr(resp, addr)
}

func (s *Network) SendMessageToUDPAddr(msg Message, addr *net.UDPAddr) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = s.Conn.WriteToUDP(data, addr)
	return err
}

func (s *Network) SendMessage(msg Message, address string) error {
	udpAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return err
	}
	return s.SendMessageToUDPAddr(msg, udpAddr)
}

// SendFindNode sends a FIND_NODE RPC request over UDP and waits for response
func (s *Network) SendFindNode(target Contact, searchID NodeID) ([]Contact, error) {
	rpcID := generateRPCID()

	msg := Message{
		Type:     FIND_NODE,
		RPCID:    rpcID,
		SenderID: s.SelfID,
		Payload: FindNodeRequest{
			TargetID: searchID,
		},
	}

	// Register response channel
	respChan := make(chan Message, 1)
	s.RegisterResponseChannel(rpcID, respChan)
	defer s.UnregisterResponseChannel(rpcID)

	// Send request
	addr := fmt.Sprintf("%s:%d", target.IP, target.Port)
	err := s.SendMessage(msg, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to send FIND_NODE: %v", err)
	}

	// Wait for response with timeout
	select {
	case resp := <-respChan:
		if resp.Type != FIND_NODE_RES {
			return nil, fmt.Errorf("expected FIND_NODE_RES, got %v", resp.Type)
		}

		// Parse response payload
		payloadBytes, _ := json.Marshal(resp.Payload)
		var findNodeResp FindNodeResponse
		err := json.Unmarshal(payloadBytes, &findNodeResp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse FIND_NODE response: %v", err)
		}

		return findNodeResp.Nodes, nil

	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout waiting for FIND_NODE response from %s", addr)
	}
}

// SendStore sends a STORE request to store a key-value pair on a remote node
func (s *Network) SendStore(target Contact, key NodeID, value []byte) error {
	rpcID := generateRPCID()

	msg := Message{
		Type:     STORE,
		RPCID:    rpcID,
		SenderID: s.SelfID,
		Payload: StoreRequest{
			Key:   key,
			Value: value,
		},
	}

	// Register response channel
	respChan := make(chan Message, 1)
	s.RegisterResponseChannel(rpcID, respChan)
	defer s.UnregisterResponseChannel(rpcID)

	// Send request
	addr := fmt.Sprintf("%s:%d", target.IP, target.Port)
	err := s.SendMessage(msg, addr)
	if err != nil {
		return fmt.Errorf("failed to send STORE: %v", err)
	}

	// Wait for response with timeout
	select {
	case resp := <-respChan:
		if resp.Type != STORE_RES {
			return fmt.Errorf("expected STORE_RES, got %v", resp.Type)
		}

		// Parse response payload
		payloadBytes, _ := json.Marshal(resp.Payload)
		var storeResp StoreResponse
		err := json.Unmarshal(payloadBytes, &storeResp)
		if err != nil {
			return fmt.Errorf("failed to parse STORE response: %v", err)
		}

		if !storeResp.Success {
			return fmt.Errorf("remote node failed to store value")
		}

		return nil

	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout waiting for STORE response from %s", addr)
	}
}

// SendFindValue sends a FIND_VALUE request to retrieve a value from a remote node
// Returns: value (if found), nodes (closest nodes if not found), error
func (s *Network) SendFindValue(target Contact, key NodeID) ([]byte, []Contact, error) {
	rpcID := generateRPCID()

	msg := Message{
		Type:     FIND_VALUE,
		RPCID:    rpcID,
		SenderID: s.SelfID,
		Payload: FindValueRequest{
			Key: key,
		},
	}

	// Register response channel
	respChan := make(chan Message, 1)
	s.RegisterResponseChannel(rpcID, respChan)
	defer s.UnregisterResponseChannel(rpcID)

	// Send request
	addr := fmt.Sprintf("%s:%d", target.IP, target.Port)
	err := s.SendMessage(msg, addr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to send FIND_VALUE: %v", err)
	}

	// Wait for response with timeout
	select {
	case resp := <-respChan:
		if resp.Type != FIND_VALUE_RES {
			return nil, nil, fmt.Errorf("expected FIND_VALUE_RES, got %v", resp.Type)
		}

		// Parse response payload
		payloadBytes, _ := json.Marshal(resp.Payload)
		var findValueResp FindValueResponse
		err := json.Unmarshal(payloadBytes, &findValueResp)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse FIND_VALUE response: %v", err)
		}

		if findValueResp.Found {
			// Value found! Return it (nodes will be nil)
			return findValueResp.Value, nil, nil
		}

		// Value not found, return closest nodes instead
		return nil, findValueResp.Nodes, nil

	case <-time.After(5 * time.Second):
		return nil, nil, fmt.Errorf("timeout waiting for FIND_VALUE response from %s", addr)
	}
}

// generateRPCID creates a simple RPC ID (we could use the id_tools function, but keeping it simple)
func generateRPCID() string {
	return fmt.Sprintf("rpc-%d", time.Now().UnixNano())
}
