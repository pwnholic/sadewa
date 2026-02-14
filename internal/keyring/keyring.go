package keyring

import (
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

type KeyRing struct {
	mu       sync.RWMutex
	keys     []*APIKey
	current  int
	strategy RotationStrategy
	logger   zerolog.Logger
}

type APIKey struct {
	ID         string
	Key        string
	Secret     string
	Passphrase string
	Disabled   bool
	LastUsed   time.Time
	ErrorCount int
}

type RotationStrategy int

const (
	RotationRoundRobin RotationStrategy = iota
	RotationOnError
	RotationOnRateLimit
)

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

func (k *KeyRing) MarkUsed() {
	k.mu.Lock()
	defer k.mu.Unlock()

	if len(k.keys) == 0 || k.keys[k.current] == nil {
		return
	}

	k.keys[k.current].LastUsed = time.Now()
}

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

func (k *APIKey) String() string {
	return fmt.Sprintf("APIKey{ID:%s, Key:%s}", k.ID, maskKey(k.Key))
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}
