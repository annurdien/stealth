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
type SessionContext struct {
	ID        string
	Page      *rod.Page
	Browser   *rod.Browser
	CreatedAt time.Time
	LastUsed  time.Time
	TTL       time.Duration // 0 means no auto-expiry
	Proxy     *models.ProxyConfig

	mu sync.Mutex // Serializes requests on the same page
}

// Lock acquires the session-level lock to prevent concurrent page access.
func (s *SessionContext) Lock() { s.mu.Lock() }

// Unlock releases the session-level lock.
func (s *SessionContext) Unlock() { s.mu.Unlock() }

// Manager stores and lifecycle-manages browser sessions.
type Manager struct {
	sessions map[string]*SessionContext
	mu       sync.RWMutex
	done     chan struct{} // Signals the reaper to stop
}

// NewManager creates a Manager and starts the TTL reaper goroutine.
func NewManager() *Manager {
	m := &Manager{
		sessions: make(map[string]*SessionContext),
		done:     make(chan struct{}),
	}
	go m.reapLoop()
	return m
}

// Create creates a new browser session. Idempotent: if the session ID already
// exists, the existing session is returned without creating a new browser.
func (m *Manager) Create(req *models.SessionCreateRequest) (*SessionContext, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := req.Session
	if id == "" {
		id = uuid.New().String()
	}

	// Return existing session (idempotent)
	if sess, exists := m.sessions[id]; exists {
		log.Printf("[session] reusing existing session %s", id)
		return sess, nil
	}

	// Launch a new browser instance
	browser, err := solver.LaunchBrowser(req.Proxy)
	if err != nil {
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	// Create a stealth page (go-rod/stealth patches applied)
	page, err := solver.CreateStealthPage(browser)
	if err != nil {
		browser.MustClose()
		return nil, fmt.Errorf("failed to create page: %w", err)
	}

	// Enable proxy auth interception if credentials provided
	if req.Proxy != nil && req.Proxy.Username != "" {
		if err := solver.EnableProxyAuth(page, req.Proxy.Username, req.Proxy.Password); err != nil {
			browser.MustClose()
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
		Browser:   browser,
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
		TTL:       ttl,
		Proxy:     req.Proxy,
	}

	m.sessions[id] = sess
	log.Printf("[session] created session %s (TTL=%v)", id, ttl)
	return sess, nil
}

// Get retrieves a session by ID and updates its LastUsed timestamp.
// Returns (session, true) if found, or (nil, false) if not found.
func (m *Manager) Get(id string) (*SessionContext, bool) {
	m.mu.RLock()
	sess, exists := m.sessions[id]
	m.mu.RUnlock()

	if exists {
		m.mu.Lock()
		sess.LastUsed = time.Now()
		m.mu.Unlock()
	}

	return sess, exists
}

// Destroy closes the browser and removes the session from the map.
// Returns true if the session existed, false if not found.
func (m *Manager) Destroy(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	sess, exists := m.sessions[id]
	if !exists {
		return false
	}

	sess.Browser.MustClose()
	delete(m.sessions, id)
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

// DestroyAll closes all sessions. Called during graceful shutdown.
func (m *Manager) DestroyAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, sess := range m.sessions {
		sess.Browser.MustClose()
		delete(m.sessions, id)
		log.Printf("[session] shutdown: destroyed session %s", id)
	}
}

// Stop signals the reaper to exit and then destroys all sessions.
func (m *Manager) Stop() {
	close(m.done)
	m.DestroyAll()
}

// reapLoop runs on a 30-second ticker and cleans up sessions that have
// exceeded their TTL based on the LastUsed timestamp.
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
		}
	}
}
