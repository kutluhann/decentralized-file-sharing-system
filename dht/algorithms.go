package dht

import (
	// "fmt"
	"fmt"
	"sort"

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
	sorter := &ContactSorter{
		contacts: ls.Shortlist,
		target:   ls.Target,
	}
	sort.Sort(sorter)
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
func (n *Node) NodeLookup(targetID NodeID) []Contact {
	// 1. INITIALIZATION
	// Start with the closest nodes we know locally.
	localCandidates := n.RoutingTable.FindClosest(targetID, constants.K)

	// Debug print
	for i := range localCandidates {
		fmt.Printf("Local Candidate: %s ID: %x\n", localCandidates[i].Name, localCandidates[i].ID)
	}

	state := NewLookupState(targetID, localCandidates)

	// 2. THE MAIN LOOP
	// We keep going until we run out of new people to ask.
	for {
		// A. SELECTION
		candidate := state.PickNextBest()

		// TERMINATION: If no unqueried nodes remain, we are done.
		if candidate == nil {
			break
		}

		// B. NETWORK CALL (RPC)
		// fmt.Printf("   [%s] asking -> [%s] ...\n", n.Name, candidate.Name)

		newNodes, err := n.Network.SendFindNode(*candidate, targetID)

		// Mark as contacted regardless of success/fail to avoid loops
		state.MarkContacted(candidate.ID)

		// C. UPDATE STATE
		if err == nil {
			// If successful, add the new suggestions to our list
			state.Append(newNodes)

			// Passive Update: Since they replied, we verify they are alive
			n.RoutingTable.AddContact(*candidate)

			// *** EARLY EXIT CHECK ***
			// If one of the returned nodes is the target, we are done!
			for _, receivedNode := range newNodes {
				if receivedNode.ID == targetID {
					// Found it! Return just this one.
					fmt.Println("FOUND NODE:", receivedNode.ID)
					return []Contact{receivedNode}
				}
			}
		}
	}

	// 3. RETURN RESULTS
	// Return the top K nodes from our sorted shortlist
	if len(state.Shortlist) > constants.K {
		return state.Shortlist[:constants.K]
	}
	return state.Shortlist
}

// ---------------------------------------------------------
// ALGORITHMS: JOIN
// ---------------------------------------------------------

func (n *Node) Join(bootstrapNode Contact) {
	// 1. Add the bootstrap node to our table
	n.RoutingTable.AddContact(bootstrapNode)

	// 2. Perform a Self-Lookup
	// This populates the buckets close to us.
	n.NodeLookup(n.ID)

	fmt.Printf("[%s] Joined network via %s (Routing Table size: %d)\n",
		n.Name, bootstrapNode.Name, n.RoutingTable.TotalContacts())
}
