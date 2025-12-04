package config

import (
	"sync"

	"os"

	ecies "github.com/ecies/go/v2"
	"github.com/joho/godotenv"
)

// Config is a simple in-memory for runtime configuration
// (private keys, context, derived keys from env, etc).
type Config struct {
	privateKey           *ecies.PrivateKey
	StorageEncryptionKey string
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

func (c *Config) SetPrivateKey(key *ecies.PrivateKey) {
	c.privateKey = key
}

func (c *Config) GetPrivateKey() *ecies.PrivateKey {
	return c.privateKey
}

func (c *Config) HasPrivateKey() bool {
	return c.privateKey != nil
}

func (c *Config) GetStorageEncryptionKey() string {
	return c.StorageEncryptionKey
}
