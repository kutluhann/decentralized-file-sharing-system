package dht

import (
	"fmt"

	"github.com/kutluhann/decentralized-file-sharing-system/constants"
)

// ---------------------------------------------------------
// LOOKUP STATE HELPER
// Manages the list of candidates during a search.
// ---------------------------------------------------------
type LookupState struct {
	Target    NodeID
	Shortlist []Contact       // The list of all nodes we know about in this search
	Contacted map[NodeID]bool // Keeps track of who we already queried
}

func NewLookupState(target NodeID, initialNodes []Contact) *LookupState {
	state := &LookupState{
		Target:    target,
		Shortlist: make([]Contact, 0),
		Contacted: make(map[NodeID]bool),
	}
	state.Append(initialNodes)
	return state
}

// Append adds new contacts to the shortlist if they aren't already there.
func (ls *LookupState) Append(contacts []Contact) {
	for _, c := range contacts {
		// Check for duplicates in Shortlist
		exists := false
		for _, existing := range ls.Shortlist {
			if existing.ID == c.ID {
				exists = true
				break
			}
		}
		if !exists {
			ls.Shortlist = append(ls.Shortlist, c)
		}
	}
	// Always resort after adding new blood
	ls.Sort()
}

// Sort orders the Shortlist by distance to the Target.
func (ls *LookupState) Sort() {
	// Inline bubble sort by XOR distance (simple but effective for small lists)
	// Alternative: Use Go's sort.Slice with custom comparator
	for i := 0; i < len(ls.Shortlist); i++ {
		for j := i + 1; j < len(ls.Shortlist); j++ {
			distI := ls.Shortlist[i].ID.Xor(ls.Target)
			distJ := ls.Shortlist[j].ID.Xor(ls.Target)
			if distJ.Less(distI) {
				ls.Shortlist[i], ls.Shortlist[j] = ls.Shortlist[j], ls.Shortlist[i]
			}
		}
	}
}

// PickNextBest returns the closest node that has NOT been queried yet.
func (ls *LookupState) PickNextBest() *Contact {
	for i := range ls.Shortlist {
		// We use a pointer so we return the actual object
		c := &ls.Shortlist[i]

		// If we haven't contacted them yet...
		if !ls.Contacted[c.ID] {
			return c
		}
	}
	return nil // Everyone has been contacted!
}

// MarkContacted records that we have queried this node.
func (ls *LookupState) MarkContacted(id NodeID) {
	ls.Contacted[id] = true
}

// ---------------------------------------------------------
// THE NodeLookup algorithm (Iterative Node Lookup)
// ---------------------------------------------------------

// NodeLookup performs the iterative lookup for a target ID.
// It keeps crawling the network until it finds the k closest nodes.
//
// TODO: Currently uses direct handler calls for testing. In production,
// this should use Network.SendFindNode() for actual RPC over UDP.
func (n *Node) NodeLookup(targetID NodeID) []Contact {
	// 1. INITIALIZATION
	// Start with the closest nodes we know locally.
	localCandidates := n.RoutingTable.GetClosestNodes(targetID, constants.K)

	// Debug print
	fmt.Printf("[LOOKUP] Searching for target: %s\n", targetID.String()[:16])
	fmt.Printf("[LOOKUP] Starting with %d local candidates\n", len(localCandidates))

	state := NewLookupState(targetID, localCandidates)

	// 2. THE MAIN LOOP
	// We keep going until we run out of new people to ask.
	for {
		// A. SELECTION
		candidate := state.PickNextBest()

		// TERMINATION: If no unqueried nodes remain, we are done.
		if candidate == nil {
			fmt.Printf("[LOOKUP] No more unqueried nodes, terminating\n")
			break
		}

		// B. NETWORK CALL (RPC)
		fmt.Printf("[LOOKUP] Querying %s:%d for nodes closer to target\n",
			candidate.IP, candidate.Port)

		// Send FIND_NODE RPC over UDP
		newNodes, err := n.Network.SendFindNode(*candidate, targetID)

		// Mark as contacted regardless of success/fail to avoid loops
		state.MarkContacted(candidate.ID)

		// C. UPDATE STATE
		if err != nil {
			fmt.Printf("[LOOKUP] ✗ Failed to query %s:%d: %v\n",
				candidate.IP, candidate.Port, err)
			continue
		}

		// If successful, add the new suggestions to our list
		state.Append(newNodes)

		// Passive Update: Since they replied, we verify they are alive
		n.RoutingTable.Update(*candidate)

		// *** EARLY EXIT CHECK ***
		// If one of the returned nodes is the target, we are done!
		for _, receivedNode := range newNodes {
			if receivedNode.ID == targetID {
				// Found it! Return just this one.
				fmt.Printf("[LOOKUP] ✓ Found exact target node: %s\n", receivedNode.ID.String()[:16])
				return []Contact{receivedNode}
			}
		}
	}

	// 3. RETURN RESULTS
	// Return the top K nodes from our sorted shortlist
	fmt.Printf("[LOOKUP] Lookup complete, returning %d closest nodes\n",
		min(len(state.Shortlist), constants.K))

	if len(state.Shortlist) > constants.K {
		return state.Shortlist[:constants.K]
	}
	return state.Shortlist
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
