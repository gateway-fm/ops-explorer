package indexer

import (
	"context"
	"strings"
	"sync"

	"explorer/internal/db"
)

// TokenCache is a thread-safe cache for tracking indexed tokens
type TokenCache struct {
	mu     sync.RWMutex
	tokens map[string]struct{} // lowercase address -> exists
}

// NewTokenCache creates a new token cache
func NewTokenCache() *TokenCache {
	return &TokenCache{
		tokens: make(map[string]struct{}),
	}
}

// Has checks if a token address is already in the cache
func (c *TokenCache) Has(address string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, exists := c.tokens[strings.ToLower(address)]
	return exists
}

// Add marks a token address as indexed
func (c *TokenCache) Add(address string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tokens[strings.ToLower(address)] = struct{}{}
}

// AddBatch adds multiple token addresses to the cache
func (c *TokenCache) AddBatch(addresses []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, addr := range addresses {
		c.tokens[strings.ToLower(addr)] = struct{}{}
	}
}

// LoadFromDB pre-populates the cache with all tokens from the database
func (c *TokenCache) LoadFromDB(ctx context.Context, database *db.DB) error {
	addresses, err := database.GetAllTokenAddresses(ctx)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	for _, addr := range addresses {
		c.tokens[strings.ToLower(addr)] = struct{}{}
	}
	return nil
}

// Size returns the number of tokens in the cache
func (c *TokenCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.tokens)
}

// ContractCache is a thread-safe cache for tracking known contracts
type ContractCache struct {
	mu        sync.RWMutex
	contracts map[string]struct{} // lowercase address -> exists
}

// NewContractCache creates a new contract cache
func NewContractCache() *ContractCache {
	return &ContractCache{
		contracts: make(map[string]struct{}),
	}
}

// Has checks if an address is a known contract
func (c *ContractCache) Has(address string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, exists := c.contracts[strings.ToLower(address)]
	return exists
}

// Add marks an address as a contract
func (c *ContractCache) Add(address string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.contracts[strings.ToLower(address)] = struct{}{}
}

// Size returns the number of contracts in the cache
func (c *ContractCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.contracts)
}
