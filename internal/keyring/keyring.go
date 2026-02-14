package keyring

import (
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// KeyRing manages a collection of API keys with rotation support.
type KeyRing struct {
	mu       sync.RWMutex
	keys     []*APIKey
	current  int
	strategy RotationStrategy
	logger   zerolog.Logger
}

// APIKey represents credentials for authenticating with an exchange API.
type APIKey struct {
	// ID is a unique identifier for this key.
	ID string
	// Key is the public API key identifier.
	Key string
	// Secret is the private API key secret.
	Secret string
	// Passphrase is an optional passphrase required by some exchanges.
	Passphrase string
	// Disabled indicates whether this key is currently disabled.
	Disabled bool
	// LastUsed records when this key was most recently used.
	LastUsed time.Time
	// ErrorCount tracks the number of consecutive errors with this key.
	ErrorCount int
}

// RotationStrategy defines when API key rotation should occur.
type RotationStrategy int

// Available rotation strategies for API key management.
const (
	// RotationRoundRobin rotates keys sequentially after each use.
	RotationRoundRobin RotationStrategy = iota
	// RotationOnError rotates to the next key when an error occurs.
	RotationOnError
	// RotationOnRateLimit rotates to the next key when a rate limit error occurs.
	RotationOnRateLimit
)

// NewKeyRing creates a new KeyRing with the given keys and rotation strategy.
// Keys are copied to prevent external modification.
func NewKeyRing(keys []*APIKey, strategy RotationStrategy) *KeyRing {
	if len(keys) == 0 {
		return &KeyRing{
			keys:     []*APIKey{},
			strategy: strategy,
			logger:   zerolog.Nop(),
		}
	}

	keysCopy := make([]*APIKey, len(keys))
	for i, k := range keys {
		keysCopy[i] = &APIKey{
			ID:         k.ID,
			Key:        k.Key,
			Secret:     k.Secret,
			Passphrase: k.Passphrase,
			Disabled:   k.Disabled,
			LastUsed:   k.LastUsed,
			ErrorCount: k.ErrorCount,
		}
	}

	return &KeyRing{
		keys:     keysCopy,
		strategy: strategy,
		logger:   zerolog.Nop(),
	}
}

// Current returns the currently active API key, skipping any disabled keys.
// It returns nil if no enabled keys are available.
func (k *KeyRing) Current() *APIKey {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if len(k.keys) == 0 {
		return nil
	}

	for i := 0; i < len(k.keys); i++ {
		idx := (k.current + i) % len(k.keys)
		if !k.keys[idx].Disabled {
			return k.keys[idx]
		}
	}

	return nil
}

// Rotate advances to the next enabled API key in the ring.
func (k *KeyRing) Rotate() {
	k.mu.Lock()
	defer k.mu.Unlock()

	if len(k.keys) == 0 {
		return
	}

	start := k.current
	for {
		k.current = (k.current + 1) % len(k.keys)
		if !k.keys[k.current].Disabled {
			return
		}
		if k.current == start {
			return
		}
	}
}

// OnError handles an error by incrementing the error count and rotating if the strategy requires it.
func (k *KeyRing) OnError(err error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	if len(k.keys) == 0 || k.keys[k.current] == nil {
		return
	}

	k.keys[k.current].ErrorCount++

	if k.strategy == RotationOnError || k.strategy == RotationOnRateLimit {
		k.Rotate()
	}
}

// MarkUsed updates the LastUsed timestamp for the current key to the current time.
func (k *KeyRing) MarkUsed() {
	k.mu.Lock()
	defer k.mu.Unlock()

	if len(k.keys) == 0 || k.keys[k.current] == nil {
		return
	}

	k.keys[k.current].LastUsed = time.Now()
}

// Disable marks the key with the given ID as disabled.
func (k *KeyRing) Disable(id string) {
	k.mu.Lock()
	defer k.mu.Unlock()

	for _, key := range k.keys {
		if key.ID == id {
			key.Disabled = true
			return
		}
	}
}

// Enable marks the key with the given ID as enabled and resets its error count.
func (k *KeyRing) Enable(id string) {
	k.mu.Lock()
	defer k.mu.Unlock()

	for _, key := range k.keys {
		if key.ID == id {
			key.Disabled = false
			key.ErrorCount = 0
			return
		}
	}
}

// Add inserts a new API key into the ring if an key with the same ID does not already exist.
func (k *KeyRing) Add(key *APIKey) {
	k.mu.Lock()
	defer k.mu.Unlock()

	for _, existing := range k.keys {
		if existing.ID == key.ID {
			return
		}
	}

	k.keys = append(k.keys, &APIKey{
		ID:         key.ID,
		Key:        key.Key,
		Secret:     key.Secret,
		Passphrase: key.Passphrase,
	})
}

// Remove deletes the key with the given ID from the ring.
func (k *KeyRing) Remove(id string) {
	k.mu.Lock()
	defer k.mu.Unlock()

	for i, key := range k.keys {
		if key.ID == id {
			k.keys = append(k.keys[:i], k.keys[i+1:]...)
			if k.current >= len(k.keys) && len(k.keys) > 0 {
				k.current = 0
			}
			return
		}
	}
}

// String returns a safe string representation of the API key with the secret masked.
func (k *APIKey) String() string {
	return fmt.Sprintf("APIKey{ID:%s, Key:%s}", k.ID, maskKey(k.Key))
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}
