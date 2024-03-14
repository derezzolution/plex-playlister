package keycache

import (
	"fmt"
	"hash/fnv"
)

type KeyCache struct {
	keyToObf   map[string]string // Key string to obfuscated key string
	obfusToKey map[string]string // Obfuscated key string to key string
}

func NewKeyCache() *KeyCache {
	return &KeyCache{
		keyToObf:   map[string]string{},
		obfusToKey: map[string]string{},
	}
}

// GetObfusKey gets obfuscated key for an unobfuscated key. If one isn't found, creates one and returns it.
func (k *KeyCache) GetObfusKey(key string) string {

	if len(k.keyToObf[key]) < 1 { // Cache miss
		h := fnv.New32a()
		h.Write([]byte(key))
		k.keyToObf[key] = fmt.Sprintf("%d", h.Sum32())
		k.obfusToKey[k.keyToObf[key]] = key
	}

	return k.keyToObf[key]
}

// GetKey gets the unobfuscated key for an obfuscated key. Returns an empty string if obfuscated key is not found.
func (k *KeyCache) GetKey(obfuscatedKey string) string {
	return k.obfusToKey[obfuscatedKey]
}
