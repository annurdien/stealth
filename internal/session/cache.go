package session

import (
	"sync"
	"time"

	"github.com/annurdien/stealth/internal/models"
)

type CachedClearance struct {
	Cookies   []models.Cookie
	UserAgent string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type ClearanceCache struct {
	mu    sync.RWMutex
	store map[string]*CachedClearance
}

func NewClearanceCache() *ClearanceCache {
	return &ClearanceCache{
		store: make(map[string]*CachedClearance),
	}
}

// GenerateKey combines scheme+host with proxy credentials to isolate sessions
func (c *ClearanceCache) GenerateKey(targetURL string, proxy *models.ProxyConfig) (string, error) {
	baseURL, err := models.ExtractBaseURL(targetURL)
	if err != nil {
		return "", err
	}
	proxyKey := ""
	if proxy != nil {
		proxyKey = proxy.HashKey()
	}
	return baseURL + "|" + proxyKey, nil
}

func (c *ClearanceCache) Get(key string) (*CachedClearance, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	val, exists := c.store[key]
	if !exists || time.Now().After(val.ExpiresAt) {
		if exists {
			delete(c.store, key)
		}
		return nil, false
	}
	return val, true
}

// Delete removes an entry from the cache
func (c *ClearanceCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.store, key)
}

func (c *ClearanceCache) Set(key string, cookies []models.Cookie, ua string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.store[key] = &CachedClearance{
		Cookies:   cookies,
		UserAgent: ua,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(ttl),
	}
}
