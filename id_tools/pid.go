package id_tools

import (
	"crypto/sha256"
	"log"
	"os"

	ecies "github.com/ecies/go/v2"
	"github.com/kutluhann/decentralized-file-sharing-system/constants"
)

// constant for file storage path
const PrivateKeyFilePath = "private_key.pem"

// typedef peerID as SHA256 type, it is not a string
type PeerID [32]byte

func GenerateNewPID() (*ecies.PrivateKey, PeerID) {
	privateKey, err := ecies.GenerateKey()

	if err != nil {
		log.Fatal("Error generating ECIES private key:", err)
	}

	peerID := generatePeerIDFromPublicKey(privateKey.PublicKey)

	return privateKey, peerID
}

func SavePrivateKey(key *ecies.PrivateKey) {

	file, err := os.Create(PrivateKeyFilePath)
	if err != nil {
		log.Fatal("Error creating private key file:", err)
	}
	defer file.Close()

	keyBytes := key.Bytes()
	_, err = file.Write(keyBytes)
	if err != nil {
		log.Fatal("Error writing private key to file:", err)
	}

}

func LoadPrivateKey() (*ecies.PrivateKey, PeerID) {
	file, err := os.Open(PrivateKeyFilePath)
	if err != nil {
		log.Fatal("Error opening private key file:", err)
	}
	defer file.Close()

	fileInfo, _ := file.Stat()

	keyBytes := make([]byte, fileInfo.Size())
	_, err = file.Read(keyBytes)
	if err != nil {
		log.Fatal("Error reading private key from file:", err)
	}
	privateKey := ecies.NewPrivateKeyFromBytes(keyBytes)

	peerID := generatePeerIDFromPublicKey(privateKey.PublicKey)

	return privateKey, peerID

}

func generatePeerIDFromPublicKey(pubKey *ecies.PublicKey) PeerID {
	pubKeyHex := pubKey.Hex(false)
	textToHash := pubKeyHex + constants.Salt
	generatedPeerID := sha256.Sum256([]byte(textToHash))

	return generatedPeerID
}
