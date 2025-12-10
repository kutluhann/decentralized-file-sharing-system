package dht

type MessageType int

const (
	PING MessageType = iota
	STORE
	FIND_NODE
	FIND_VALUE

	PING_RES
	STORE_RES
	FIND_NODE_RES
	FIND_VALUE_RES

	// Secure Join Handshake Protocol
	JOIN_REQ       // Step 1: NewNode -> Genesis (I want to join, here is my PubKey)
	JOIN_CHALLENGE // Step 2: Genesis -> NewNode (Here is a nonce, sign it)
	JOIN_RES       // Step 3: NewNode -> Genesis (Here is the signature)
	JOIN_ACK       // Step 4: Genesis -> NewNode (Welcome / Go Away)
	
	// Proof of Space for Sybil Resistance
	POS_CHALLENGE  // Genesis -> NewNode (Prove you have allocated space)
	POS_PROOF      // NewNode -> Genesis (Here is my PoS proof)
)

type Message struct {
	Type     MessageType `json:"type"`
	SenderID NodeID      `json:"sender_id"`
	RPCID    string      `json:"rpc_id"`
	Payload  interface{} `json:"payload"`
}

type PingRequest struct {
	Timestamp int64 `json:"timestamp"`
}

type PingResponse struct {
	Timestamp int64 `json:"timestamp"`
}

type StoreRequest struct {
	Key   NodeID `json:"key"`
	Value []byte `json:"value"`
}

type StoreResponse struct {
	Success bool `json:"success"`
}

type FindNodeRequest struct {
	TargetID NodeID `json:"target_id"`
}

type FindNodeResponse struct {
	Nodes []Contact `json:"nodes"`
}

type FindValueRequest struct {
	Key NodeID `json:"key"`
}

type FindValueResponse struct {
	Found bool      `json:"found"`
	Value []byte    `json:"value,omitempty"`
	Nodes []Contact `json:"nodes,omitempty"`
}

type JoinRequestPayload struct {
	PeerID    NodeID `json:"peer_id"`
	PublicKey []byte `json:"public_key"`
}

type JoinChallengePayload struct {
	Nonce string `json:"nonce"`
}

type JoinResponsePayload struct {
	Signature []byte `json:"signature"`
}

type JoinAckPayload struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type PosChallengePayload struct {
	ChallengeValue [32]byte `json:"challenge_value"`
	StartIndex     uint64   `json:"start_index"`
	EndIndex       uint64   `json:"end_index"`
	Required       int      `json:"required"`
}

type PosProofElement struct {
	Layer       int      `json:"layer"`
	Index       uint64   `json:"index"`
	Value       [32]byte `json:"value"`
	ParentLeft  uint64   `json:"parent_left"`
	ParentRight uint64   `json:"parent_right"`
}

type PosProofPayload struct {
	ChallengeValue [32]byte          `json:"challenge_value"`
	StartIndex     uint64            `json:"start_index"`
	EndIndex       uint64            `json:"end_index"`
	Required       int               `json:"required"`
	ProofChain     []PosProofElement `json:"proof_chain"`
}
