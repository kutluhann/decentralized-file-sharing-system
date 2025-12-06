package dht

import (
	"encoding/hex"
	"math/bits"

	"github.com/kutluhann/decentralized-file-sharing-system/id_tools"
)

type NodeID id_tools.PeerID

func (id NodeID) Xor(other NodeID) NodeID {
	var result NodeID
	for i := 0; i < len(id); i++ {
		result[i] = id[i] ^ other[i]
	}
	return result
}

func (id NodeID) PrefixLen(other NodeID) int {
	for i := 0; i < len(id); i++ {
		x := id[i] ^ other[i]

		if x != 0 {
			return i*8 + bits.LeadingZeros8(x)
		}
	}
	return len(id) * 8
}

func (id NodeID) Less(other NodeID) bool {
	for i := 0; i < len(id); i++ {
		if id[i] != other[i] {
			return id[i] < other[i]
		}
	}
	return false
}

func (id NodeID) String() string {
	return hex.EncodeToString(id[:])
}
