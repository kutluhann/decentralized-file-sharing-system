package config

import (
	"crypto/ecdsa"
	"sync"

	"os"

	"github.com/joho/godotenv"
	"github.com/kutluhann/decentralized-file-sharing-system/id_tools"
)

// Config is a simple in-memory for runtime configuration
// (private keys, context, derived keys from env, etc).
type Config struct {
	privateKey           *ecdsa.PrivateKey
	StorageEncryptionKey string
	peerID               id_tools.PeerID
}

var (
	config     *Config
	configOnce sync.Once
)

func Init() *Config {
	configOnce.Do(func() {

		godotenv.Load()
		storageEncryptionKey := os.Getenv("STORAGE_ENCYRPTION_KEY")

		config = &Config{
			privateKey:           nil,
			StorageEncryptionKey: storageEncryptionKey,
		}
	})
	return config
}

func GetConfig() *Config {
	if config == nil {
		return Init()
	}
	return config
}

func (c *Config) SetPrivateKey(key *ecdsa.PrivateKey) {
	c.privateKey = key
}

func (c *Config) GetPrivateKey() *ecdsa.PrivateKey {
	return c.privateKey
}

func (c *Config) HasPrivateKey() bool {
	return c.privateKey != nil
}

func (c *Config) GetStorageEncryptionKey() string {
	return c.StorageEncryptionKey
}

func (c *Config) GetPeerID() id_tools.PeerID {
	return c.peerID
}

func (c *Config) SetPeerID(pid id_tools.PeerID) {
	c.peerID = pid
}
