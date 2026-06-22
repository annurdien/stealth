package session

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/google/uuid"

	"github.com/annurdien/stealth/internal/models"
	"github.com/annurdien/stealth/internal/solver"
)

// SessionContext holds a live browser session.
// In the browser pool architecture, Browser is an Incognito context.
type SessionContext struct {
	ID        string
	Page      *rod.Page
	Browser   *rod.Browser
	CreatedAt time.Time
	LastUsed  time.Time
	TTL       time.Duration // 0 means no auto-expiry
	Proxy     *models.ProxyConfig
	poolKey   string // Key to decrement refcount in the pool

	mu sync.Mutex // Serializes requests on the same page
}

// Lock acquires the session-level lock to prevent concurrent page access.
func (s *SessionContext) Lock() { s.mu.Lock() }

// Unlock releases the session-level lock.
func (s *SessionContext) Unlock() { s.mu.Unlock() }

// pooledBrowser represents a physical headless Chrome process.
type pooledBrowser struct {
	browser  *rod.Browser
	refCount int
	lastUsed time.Time
	sem      chan struct{} // Semaphore limiting concurrent incognito contexts
}

type Manager struct {
	sessions       map[string]*SessionContext
	pool           map[string]*pooledBrowser
	mu             sync.RWMutex
	done           chan struct{} // Signals the reaper to stop
	maxTabs        int           // Maximum concurrent tabs per pooled browser
	ClearanceCache *ClearanceCache
}

// NewManager creates a Manager and starts the TTL reaper goroutine.
func NewManager(maxTabs int) *Manager {
	m := &Manager{
		sessions:       make(map[string]*SessionContext),
		pool:           make(map[string]*pooledBrowser),
		done:           make(chan struct{}),
		maxTabs:        maxTabs,
		ClearanceCache: NewClearanceCache(),
	}
	go m.reapLoop()
	return m
}

func (m *Manager) Create(req *models.SessionCreateRequest) (*SessionContext, error) {
	m.mu.Lock()

	id := req.Session
	if id == "" {
		id = uuid.New().String()
	}

	if sess, exists := m.sessions[id]; exists {
		log.Printf("[session] reusing existing session %s", id)
		m.mu.Unlock()
		return sess, nil
	}

	poolKey := req.Proxy.HashKey()
	pb, exists := m.pool[poolKey]
	if !exists {
		browser, err := solver.LaunchBrowser(req.Proxy)
		if err != nil {
			m.mu.Unlock()
			return nil, fmt.Errorf("failed to launch pooled browser: %w", err)
		}
		pb = &pooledBrowser{
			browser:  browser,
			refCount: 0,
			lastUsed: time.Now(),
			sem:      make(chan struct{}, m.maxTabs),
		}
		m.pool[poolKey] = pb
		log.Printf("[pool] launched new physical browser for proxy key: %q", poolKey)
	}

	pb.refCount++ // Increment early so the reaper doesn't destroy it while we wait
	m.mu.Unlock()

	// Wait for a slot to open on this physical browser (Concurrency Semaphore)
	pb.sem <- struct{}{}

	incognitoBrowser, err := pb.browser.Incognito()
	if err != nil {
		<-pb.sem
		m.mu.Lock()
		pb.refCount--
		m.mu.Unlock()
		return nil, fmt.Errorf("failed to create incognito context: %w", err)
	}

	page, err := solver.CreateStealthPage(incognitoBrowser)
	if err != nil {
		incognitoBrowser.MustClose()
		<-pb.sem
		m.mu.Lock()
		pb.refCount--
		m.mu.Unlock()
		return nil, fmt.Errorf("failed to create page: %w", err)
	}

	if req.Proxy != nil && req.Proxy.Username != "" {
		if err := solver.EnableProxyAuth(page, req.Proxy.Username, req.Proxy.Password); err != nil {
			incognitoBrowser.MustClose()
			<-pb.sem
			m.mu.Lock()
			pb.refCount--
			m.mu.Unlock()
			return nil, fmt.Errorf("failed to enable proxy auth: %w", err)
		}
	}

	ttl := time.Duration(0)
	if req.TTL > 0 {
		ttl = time.Duration(req.TTL) * time.Second
	}

	sess := &SessionContext{
		ID:        id,
		Page:      page,
		Browser:   incognitoBrowser,
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
		TTL:       ttl,
		Proxy:     req.Proxy,
		poolKey:   poolKey,
	}

	m.mu.Lock()
	pb.lastUsed = time.Now()
	m.sessions[id] = sess
	m.mu.Unlock()

	log.Printf("[session] created session %s (TTL=%v)", id, ttl)
	return sess, nil
}

// Get retrieves a session by ID and updates its LastUsed timestamp.
func (m *Manager) Get(id string) (*SessionContext, bool) {
	m.mu.RLock()
	sess, exists := m.sessions[id]
	m.mu.RUnlock()

	if exists {
		m.mu.Lock()
		sess.LastUsed = time.Now()
		if pb, ok := m.pool[sess.poolKey]; ok {
			pb.lastUsed = time.Now()
		}
		m.mu.Unlock()
	}

	return sess, exists
}

// Destroy closes the browser incognito context and removes the session from the map.
func (m *Manager) Destroy(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	sess, exists := m.sessions[id]
	if !exists {
		return false
	}

	sess.Browser.MustClose()
	delete(m.sessions, id)

	if pb, ok := m.pool[sess.poolKey]; ok {
		pb.refCount--
		pb.lastUsed = time.Now()
		<-pb.sem // Release semaphore slot
	}

	log.Printf("[session] destroyed session %s", id)
	return true
}

// List returns all active session IDs.
func (m *Manager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	return ids
}

// DestroyAll closes all sessions and pooled browsers. Called during graceful shutdown.
func (m *Manager) DestroyAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, sess := range m.sessions {
		sess.Browser.MustClose()
		delete(m.sessions, id)
		log.Printf("[session] shutdown: destroyed session %s", id)
	}

	for key, pb := range m.pool {
		pb.browser.MustClose()
		delete(m.pool, key)
		log.Printf("[pool] shutdown: destroyed physical browser %q", key)
	}
}

// Stop signals the reaper to exit and then destroys all sessions.
func (m *Manager) Stop() {
	close(m.done)
	m.DestroyAll()
}

// reapLoop runs on a 30-second ticker and cleans up sessions that have
// exceeded their TTL, as well as idle pooled browsers.
func (m *Manager) reapLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.reapExpired()
		case <-m.done:
			return
		}
	}
}

func (m *Manager) reapExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	for id, sess := range m.sessions {
		if sess.TTL > 0 && now.Sub(sess.LastUsed) > sess.TTL {
			log.Printf("[session] reaping expired session %s (idle=%v TTL=%v)",
				id, now.Sub(sess.LastUsed).Round(time.Second), sess.TTL)

			sess.Browser.MustClose()
			delete(m.sessions, id)

			if pb, ok := m.pool[sess.poolKey]; ok {
				pb.refCount--
				pb.lastUsed = now
				<-pb.sem // Release semaphore slot
			}
		}
	}

	idleTimeout := 5 * time.Minute
	for key, pb := range m.pool {
		if pb.refCount <= 0 && now.Sub(pb.lastUsed) > idleTimeout {
			log.Printf("[pool] reaping idle physical browser %q (idle=%v)", key, now.Sub(pb.lastUsed).Round(time.Second))
			pb.browser.MustClose()
			delete(m.pool, key)
		}
	}
}
