package exchange

import (
	"fmt"
	"sync"
)

type Container struct {
	mu        sync.RWMutex
	exchanges map[string]Exchange
}

func NewContainer() *Container {
	return &Container{
		exchanges: make(map[string]Exchange),
	}
}

func (c *Container) Register(name string, ex Exchange) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.exchanges[name] = ex
}

func (c *Container) Get(name string) (Exchange, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ex, exists := c.exchanges[name]
	if !exists {
		return nil, fmt.Errorf("exchange %q not found", name)
	}
	return ex, nil
}

func (c *Container) Names() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]string, 0, len(c.exchanges))
	for name := range c.exchanges {
		names = append(names, name)
	}
	return names
}

func (c *Container) Unregister(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.exchanges, name)
}

func (c *Container) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.exchanges = make(map[string]Exchange)
}

func (c *Container) Exists(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, exists := c.exchanges[name]
	return exists
}
