package main

import (
	"crypto/ecdsa"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/kutluhann/decentralized-file-sharing-system/api"
	"github.com/kutluhann/decentralized-file-sharing-system/dht"
	"github.com/kutluhann/decentralized-file-sharing-system/id_tools"
)

func main() {
	isGenesis := flag.Bool("genesis", false, "Start as a Genesis Node (no bootstrap)")
	port := flag.Int("port", 8080, "UDP port to listen on")
	httpPort := flag.Int("http", 8000, "HTTP API port for client requests")
	bootstrapIP := flag.String("bootstrap", "", "Bootstrap Node IP:Port (e.g. 127.0.0.1:8080)")
	flag.Parse()

	fmt.Printf("Starting DHT Node on port %d...\n", *port)

	var privateKey *ecdsa.PrivateKey
	var peerID id_tools.PeerID

	keyFile := "private_key.pem"

	if _, err := os.Stat(keyFile); err == nil {
		fmt.Println("Loading existing private key from", keyFile)
		privateKey, peerID = id_tools.LoadPrivateKey()
	} else {
		fmt.Println("Generating new identity...")
		privateKey, peerID = id_tools.GenerateNewPID()
		id_tools.SavePrivateKey(privateKey)
	}

	fmt.Println("Verifying identity integrity...")
	if !id_tools.VerifyIdentity(privateKey, peerID) {
		log.Fatal("CRITICAL: Identity verification failed! Exiting.")
	}
	fmt.Println("Identity verified successfully.")

	contact := dht.Contact{
		ID:       dht.NodeID(peerID),
		IP:       "127.0.0.1",
		Port:     *port,
		LastSeen: time.Now(),
	}

	network, err := dht.NewNetwork(fmt.Sprintf(":%d", *port), dht.NodeID(peerID))
	if err != nil {
		log.Fatalf("Failed to start network: %v", err)
	}

	node := dht.NewNode(contact, privateKey)
	node.Network = network
	network.SetHandler(node)

	fmt.Printf("Node initialized with ID: %s\n", node.Self.ID.String())

	// Initialize Proof of Space plot for Sybil resistance
	fmt.Println("Initializing Proof of Space...")
	if err := node.InitializePosPlot(); err != nil {
		log.Fatalf("Failed to initialize PoS plot: %v", err)
	}
	fmt.Println("✓ Proof of Space ready")

	// Start UDP network listener for DHT protocol
	go network.Listen()

	// Start HTTP API server for client requests
	httpServer := api.NewHTTPServer(node, *httpPort)
	go func() {
		err := httpServer.Start()
		if err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	fmt.Printf("HTTP API listening on port %d\n", *httpPort)

	if *isGenesis {
		fmt.Println("--> Running as GENESIS Node. Waiting for connections...")
	} else {
		if *bootstrapIP == "" {
			log.Fatal("FATAL: Bootstrap address required for non-genesis nodes. Use -bootstrap flag (e.g., -bootstrap 127.0.0.1:8080)")
		}

		_, err := net.ResolveUDPAddr("udp", *bootstrapIP)
		if err != nil {
			log.Fatalf("FATAL: Invalid bootstrap address format '%s': %v\n", *bootstrapIP, err)
		}

		fmt.Printf("--> Bootstrapping... Connecting to %s\n", *bootstrapIP)

		// Step 1: Secure Handshake (Authentication)
		bootstrapContact, err := node.JoinNetwork(*bootstrapIP)
		if err != nil {
			log.Fatalf("FATAL: Failed to join network: %v\n", err)
		}

		fmt.Println("✓ Secure handshake complete!")
		fmt.Printf("✓ Bootstrap node: %s at %s:%d\n",
			bootstrapContact.ID.String()[:16], bootstrapContact.IP, bootstrapContact.Port)

		fmt.Printf("[JOIN] Starting Kademlia bootstrap process\n")

		// 1. Add the bootstrap node to our routing table
		node.RoutingTable.Update(bootstrapContact)
		fmt.Printf("[JOIN] Added bootstrap node %s to routing table\n",
			bootstrapContact.ID.String()[:16])

		// 2. Perform a Self-Lookup
		// This is the core of Kademlia's bootstrap: by looking up our own ID,
		// we populate the buckets closest to us, which are the most important.
		fmt.Printf("[JOIN] Performing self-lookup to populate routing table\n")
		closestNodes := node.NodeLookup(node.Self.ID)

		fmt.Printf("[JOIN] ✓ Bootstrap complete. Found %d nodes close to self\n", len(closestNodes))

		fmt.Println("✓ Successfully joined the network!")
	}

	select {}
}
