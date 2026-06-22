package session

import (
	"errors"
	"net/url"
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
	baseURL, err := extractBaseURL(targetURL)
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
	c.mu.RLock()
	defer c.mu.RUnlock()

	val, exists := c.store[key]
	if !exists || time.Now().After(val.ExpiresAt) {
		return nil, false
	}
	return val, true
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

func extractBaseURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", errors.New("invalid URL: scheme or host is empty")
	}
	return u.Scheme + "://" + u.Host, nil
}
