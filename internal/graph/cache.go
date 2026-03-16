package graph

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Cache wraps a Graph and reloads it from the database when the TTL expires.
type Cache struct {
	mu      sync.RWMutex
	g       *Graph
	builtAt time.Time
	ttl     time.Duration
	db      Querier
}

// NewCache creates a Cache that reloads the graph from db every ttl duration.
func NewCache(db Querier, ttl time.Duration) *Cache {
	return &Cache{db: db, ttl: ttl}
}

// Get returns the cached graph, reloading from the database if the TTL has expired.
func (c *Cache) Get(ctx context.Context) (*Graph, error) {
	c.mu.RLock()
	if c.g != nil && time.Since(c.builtAt) < c.ttl {
		g := c.g
		c.mu.RUnlock()
		return g, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	// double-check after acquiring write lock
	if c.g != nil && time.Since(c.builtAt) < c.ttl {
		return c.g, nil
	}

	slog.Info("loading waypoint graph from database")
	g, err := Load(ctx, c.db)
	if err != nil {
		return nil, err
	}
	c.g = g
	c.builtAt = time.Now()
	slog.Info("waypoint graph loaded", "nodes", len(g.nodes))
	return g, nil
}
