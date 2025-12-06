package testing

import (
	"fmt"
	"log"
	"os"

	"github.com/kutluhann/decentralized-file-sharing-system/config"
	"github.com/kutluhann/decentralized-file-sharing-system/id_tools"
)

func Id_Test() {

	fmt.Println("ID Test Function")

	privateKeyExists := false

	// test if private key exists
	if _, err := os.Stat(id_tools.PrivateKeyFilePath); err == nil {
		privateKeyExists = true
	}

	// cli that asks user if they want to generate a new key, or load existing one
	// it will have input as 1 and 2, and default to load existing one
	var choice int
	if privateKeyExists {
		fmt.Println("Private key file exists. Choose an option:")
		fmt.Println("1. Load existing private key")
		fmt.Println("2. Generate new private key")
		fmt.Print("Enter choice (1 or 2): ")
		fmt.Scanln(&choice)
	} else {
		fmt.Println("No existing private key found. Generating a new one.")
		choice = 2
	}

	if choice == 1 && privateKeyExists {
		privateKey, peerID := id_tools.LoadPrivateKey()
		config.GetConfig().SetPrivateKey(privateKey)
		config.GetConfig().SetPeerID(peerID)
	} else {
		fmt.Println("Generating new private key...")

		privateKey, peerID := id_tools.GenerateNewPID()
		config.GetConfig().SetPeerID(peerID)
		config.GetConfig().SetPrivateKey(privateKey)
		id_tools.SavePrivateKey(privateKey)

	}

	log.Default().Println("Public Key:", config.GetConfig().GetPrivateKey().PublicKey)
	log.Default().Println("Peer ID:", config.GetConfig().GetPeerID())

	peerID_verification_test()
}

func peerID_verification_test() {
	log.Default().Println("Peer ID Verification Test")

	peer1PrivateKey, _ := id_tools.GenerateNewPID()

	peer1PeerID := id_tools.GeneratePeerIDFromPublicKey(&peer1PrivateKey.PublicKey)

	log.Default().Println("Peer 1 Public Key:", peer1PrivateKey.PublicKey)
	log.Default().Println("Peer 1 ID:", peer1PeerID)

	// Peer 1 signs a message
	message := id_tools.GenerateSecureRandomMessage()
	signature := id_tools.SignMessage(*peer1PrivateKey, message)

	log.Default().Println("Message:", message)
	log.Default().Println("Signature:", signature)

	// Verifying the signature with public key and peer ID
	isValid := id_tools.VerifySignature(peer1PrivateKey.PublicKey, message, signature)
	if isValid {
		log.Default().Println("Signature is valid. Peer ID verification successful.")
	} else {
		log.Default().Println("Signature is invalid. Peer ID verification failed.")
	}

}
