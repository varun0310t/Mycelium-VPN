//go:build linux
// +build linux

package server

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// ClientSession identified by UDP address only
// No virtual IP tracking needed - client handles that!
type ClientSession struct {
	Addr          *net.UDPAddr
	AssignedIP    int
	LastSeen      time.Time
	Authenticated bool
	BytesSent     uint64
	BytesRecv     uint64
	ConnectedAt   time.Time
}

type Manager struct {
	sessions    map[string]*ClientSession
	assignedIPs map[int]*ClientSession
	IPPool      *IPPool
	mu          sync.RWMutex
}

func NewManager() (*Manager, error) {

	return &Manager{
		sessions:    make(map[string]*ClientSession),
		assignedIPs: make(map[int]*ClientSession),
		IPPool:      NewIPPool(ServerCfg.IPPoolMin, ServerCfg.IPPoolMax),
	}, nil
}

func (m *Manager) AddClient(addr *net.UDPAddr) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := addr.String()

	if existing, ok := m.sessions[key]; ok {

		existing.LastSeen = time.Now()
		return nil
	}

	assignedIP, err := m.IPPool.Allocate()
	if err != nil {
		return fmt.Errorf("failed to allocate IP: %w", err)
	}

	session := &ClientSession{
		Addr:        addr,
		AssignedIP:  assignedIP,
		LastSeen:    time.Now(),
		ConnectedAt: time.Now(),
	}

	m.sessions[key] = session
	m.assignedIPs[assignedIP] = session

	fmt.Printf("Client connected: %s -> Assigned IP: 10.8.0.%d\n", addr.String(), assignedIP)
	return nil
}

// Get client by UDP address
func (m *Manager) GetClient(addr *net.UDPAddr) (*ClientSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[addr.String()]
	return session, exists
}

// GetClientByIP looks up client by their assigned IP
func (m *Manager) GetClientByIP(ip int) (*ClientSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.assignedIPs[ip]
	return session, exists
}

// add Client if not exists and than get Client by UDP address
func (m *Manager) GetOrAddClient(addr *net.UDPAddr) (*ClientSession, error) {
	key := addr.String()

	// Try read lock first (fast path)
	m.mu.RLock()
	if session, exists := m.sessions[key]; exists {
		session.LastSeen = time.Now()
		m.mu.RUnlock()
		return session, nil
	}
	m.mu.RUnlock()

	// Need to add - upgrade to write lock
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check (another goroutine might have added it)
	if session, exists := m.sessions[key]; exists {
		session.LastSeen = time.Now()
		return session, nil
	}

	// Allocate IP
	assignedIP, err := m.IPPool.Allocate()
	if err != nil {
		return nil, fmt.Errorf("failed to allocate IP: %w", err)
	}

	// Create new session
	session := &ClientSession{
		Addr:        addr,
		AssignedIP:  assignedIP,
		LastSeen:    time.Now(),
		ConnectedAt: time.Now(),
	}
	m.sessions[key] = session
	m.assignedIPs[assignedIP] = session

	fmt.Printf("Client connected: %s -> Assigned IP: 10.8.0.%d\n", addr.String(), assignedIP)
	return session, nil
}

// Remove client by UDP address
func (m *Manager) RemoveClient(addr *net.UDPAddr) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := addr.String()
	if session, exists := m.sessions[key]; exists {
		// Release IP
		m.IPPool.Release(session.AssignedIP)
		delete(m.assignedIPs, session.AssignedIP)
		delete(m.sessions, key)

		fmt.Printf("Client disconnected: %s (Assigned IP: 10.8.0.%d)\n", addr.String(), session.AssignedIP)
	}
}

// Update last seen timestamp
func (m *Manager) UpdateLastSeen(addr *net.UDPAddr) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, exists := m.sessions[addr.String()]; exists {
		session.LastSeen = time.Now()
	}
}

// Update byte counters
func (m *Manager) AddBytesSent(addr *net.UDPAddr, bytes uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, exists := m.sessions[addr.String()]; exists {
		session.BytesSent += bytes
	}
}

func (m *Manager) AddBytesRecv(addr *net.UDPAddr, bytes uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, exists := m.sessions[addr.String()]; exists {
		session.BytesRecv += bytes
	}
}

// Remove stale sessions (no packets for timeout duration)
func (m *Manager) CleanupStale(timeout time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	removed := 0
	now := time.Now()

	for key, session := range m.sessions {
		if now.Sub(session.LastSeen) > timeout {
			// Release IP
			m.IPPool.Release(session.AssignedIP)
			delete(m.assignedIPs, session.AssignedIP)
			delete(m.sessions, key)
			removed++
		}
	}

	return removed
}

// Get all active sessions
func (m *Manager) GetAllSessions() []*ClientSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*ClientSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessionCopy := *s
		sessions = append(sessions, &sessionCopy)
	}
	return sessions
}

// Count active sessions
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// Check if client exists
func (m *Manager) Exists(addr *net.UDPAddr) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.sessions[addr.String()]
	return exists
}

// Get session info for monitoring/stats
func (m *Manager) GetSessionInfo(addr *net.UDPAddr) map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[addr.String()]
	if !exists {
		return nil
	}

	return map[string]interface{}{
		"address":      session.Addr.String(),
		"assigned_ip":  fmt.Sprintf("10.8.0.%d", session.AssignedIP),
		"connected_at": session.ConnectedAt,
		"last_seen":    session.LastSeen,
		"bytes_sent":   session.BytesSent,
		"bytes_recv":   session.BytesRecv,
		"duration":     time.Since(session.ConnectedAt).Seconds(),
	}
}

// SetAuthenticated marks a client session as authenticated
func (m *Manager) SetAuthenticated(addr *net.UDPAddr, authenticated bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, exists := m.sessions[addr.String()]; exists {
		session.Authenticated = authenticated
		if authenticated {
			fmt.Printf("Client %s authenticated successfully\n", addr.String())
		}
	}
}
