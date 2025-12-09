package id_tools

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"log"
	"os"
	"path/filepath"

	"github.com/kutluhann/decentralized-file-sharing-system/constants"
)

// PrivateKeyFilePath is the path to the private key file
var PrivateKeyFilePath = "private_key.pem"

// SetDataDirectory sets the data directory for storing private keys
func SetDataDirectory(dir string) {
	PrivateKeyFilePath = filepath.Join(dir, "private_key.pem")
}

var ellipticCurve = elliptic.P256()

// typedef peerID as SHA256 type, it is not a string
type PeerID [32]byte

func GenerateNewPID() (*ecdsa.PrivateKey, PeerID) {

	privateKey, err := ecdsa.GenerateKey(ellipticCurve, rand.Reader)

	if err != nil {
		log.Fatal("Error generating ECDSA private key:", err)
	}

	peerID := GeneratePeerIDFromPublicKey(&privateKey.PublicKey)

	return privateKey, peerID
}

func SavePrivateKey(key *ecdsa.PrivateKey) {

	file, err := os.Create(PrivateKeyFilePath)
	if err != nil {
		log.Fatal("Error creating private key file:", err)
	}
	defer file.Close()

	keyBytes, _ := key.Bytes()
	_, err = file.Write(keyBytes)
	if err != nil {
		log.Fatal("Error writing private key to file:", err)
	}

}

func LoadPrivateKey() (*ecdsa.PrivateKey, PeerID) {
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

	privateKey, err := ecdsa.ParseRawPrivateKey(ellipticCurve, keyBytes)
	if err != nil {
		log.Fatal("Error parsing private key:", err)
	}

	peerID := GeneratePeerIDFromPublicKey(&privateKey.PublicKey)

	return privateKey, peerID

}

func GeneratePeerIDFromPublicKey(pubKey *ecdsa.PublicKey) PeerID {
	pubKeyBytes, _ := pubKey.Bytes()
	// apppend the pubKeyBytes with the system salt
	dataToHash := append(pubKeyBytes, []byte(constants.Salt)...)
	generatedPeerID := sha256.Sum256(dataToHash)

	return generatedPeerID
}

// It is to check whether the other peer's public key matches its peer ID
func CheckPublicKeyMatchesPeerID(pubKey *ecdsa.PublicKey, pid PeerID) bool {
	generatedPID := GeneratePeerIDFromPublicKey(pubKey)
	return generatedPID == pid
}

func GenerateSecureRandomMessage() string {
	randomMessage := rand.Text()
	return randomMessage
}

func SignMessage(privateKey ecdsa.PrivateKey, message string) []byte {
	hashedMessage := sha256.Sum256([]byte(message))
	signature, err := ecdsa.SignASN1(rand.Reader, &privateKey, hashedMessage[:])
	if err != nil {
		log.Fatal("Error signing message:", err)
	}
	return signature
}

func VerifySignature(publicKey ecdsa.PublicKey, message string, signature []byte) bool {
	hashedMessage := sha256.Sum256([]byte(message))
	valid := ecdsa.VerifyASN1(&publicKey, hashedMessage[:], signature)
	return valid
}

func VerifyIdentity(privateKey *ecdsa.PrivateKey, peerID PeerID) bool {
	if !CheckPublicKeyMatchesPeerID(&privateKey.PublicKey, peerID) {
		log.Println("Error: Public Key does not match Peer ID")
		return false
	}

	message := GenerateSecureRandomMessage()
	signature := SignMessage(*privateKey, message)
	isValid := VerifySignature(privateKey.PublicKey, message, signature)

	if !isValid {
		log.Println("Error: Cryptographic signature verification failed")
		return false
	}

	return true
}
