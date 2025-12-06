package id_tools

import (
	"encoding/hex"
	"math/bits"
)

func (id PeerID) Xor(other PeerID) PeerID {
	var result PeerID
	for i := 0; i < len(id); i++ {
		result[i] = id[i] ^ other[i]
	}
	return result
}

func (id PeerID) PrefixLen(other PeerID) int {
	for i := 0; i < len(id); i++ {
		x := id[i] ^ other[i]

		if x != 0 {
			return i * 8 + bits.LeadingZeros8(x)
		}
	}
	return len(id) * 8
}

func (id PeerID) Less(other PeerID) bool {
    for i := 0; i < len(id); i++ {
        if id[i] != other[i] {
            return id[i] < other[i]
        }
    }
    return false
}

func (id PeerID) String() string {
	return hex.EncodeToString(id[:])
}