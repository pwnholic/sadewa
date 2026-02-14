package exchange

import (
	"fmt"
	"sync"
)

// Container is a thread-safe registry for managing multiple exchange instances.
// It provides dependency injection capabilities for exchange clients.
type Container struct {
	mu        sync.RWMutex
	exchanges map[string]Exchange
}

// NewContainer creates and returns a new empty exchange container.
func NewContainer() *Container {
	return &Container{
		exchanges: make(map[string]Exchange),
	}
}

// Register adds an exchange instance to the container with the given name.
// If an exchange with the same name exists, it will be overwritten.
func (c *Container) Register(name string, ex Exchange) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.exchanges[name] = ex
}

// Get retrieves an exchange instance by name.
// Returns an error if no exchange is registered with the given name.
func (c *Container) Get(name string) (Exchange, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ex, exists := c.exchanges[name]
	if !exists {
		return nil, fmt.Errorf("exchange %q not found", name)
	}
	return ex, nil
}

// Names returns a list of all registered exchange names.
func (c *Container) Names() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]string, 0, len(c.exchanges))
	for name := range c.exchanges {
		names = append(names, name)
	}
	return names
}

// Unregister removes an exchange from the container by name.
func (c *Container) Unregister(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.exchanges, name)
}

// Clear removes all exchanges from the container.
func (c *Container) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.exchanges = make(map[string]Exchange)
}

// Exists checks whether an exchange with the given name is registered.
func (c *Container) Exists(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, exists := c.exchanges[name]
	return exists
}
