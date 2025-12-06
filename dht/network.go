package dht

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
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
	buf := make([]byte, 4096)

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
		msg.Type == JOIN_CHALLENGE || msg.Type == JOIN_ACK

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

		ack, err := s.Handler.HandleJoinResponse(sender, req)
		if err != nil {
			s.sendResponse(msg.RPCID, JOIN_ACK, JoinAckPayload{Success: false, Message: err.Error()}, addr)
			return
		}
		s.sendResponse(msg.RPCID, JOIN_ACK, ack, addr)
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
