package keycache

import (
	"fmt"
	"hash/fnv"
	"sync"

	"github.com/google/uuid"
)

type KeyCache struct {
	mutex sync.RWMutex

	salt       string            // Salt to use for hashing
	keyToObf   map[string]string // Key string to obfuscated key string
	obfusToKey map[string]string // Obfuscated key string to key string
}

func NewKeyCache(salt string) *KeyCache {
	if len(salt) < 1 {
		salt = uuid.New().String()
	}
	return &KeyCache{
		salt:       salt,
		keyToObf:   map[string]string{},
		obfusToKey: map[string]string{},
	}
}

// GetObfusKey gets obfuscated key for an unobfuscated key. If one isn't found, creates one and returns it.
func (k *KeyCache) GetObfusKey(key string) string {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	if len(k.keyToObf[key]) < 1 { // Cache miss
		h := fnv.New32a()
		h.Write([]byte(fmt.Sprintf("%s%s", k.salt, key)))
		k.keyToObf[key] = fmt.Sprintf("%d", h.Sum32())
		k.obfusToKey[k.keyToObf[key]] = key
	}

	return k.keyToObf[key]
}

// GetKey gets the unobfuscated key for an obfuscated key. Returns an empty string if obfuscated key is not found.
func (k *KeyCache) GetKey(obfuscatedKey string) string {
	k.mutex.RLock()
	defer k.mutex.RUnlock()
	return k.obfusToKey[obfuscatedKey]
}
