//go:build linux
// +build linux

package server

import (
	"fmt"
	"sync"
)

// IPPool manages available IPs for client assignment
type IPPool struct {
	minIP    int
	maxIP    int
	assigned map[int]bool
	mu       sync.Mutex
}

// NewIPPool creates a new IP pool with the given range
func NewIPPool(minIP, maxIP int) *IPPool {
	return &IPPool{
		minIP:    minIP,
		maxIP:    maxIP,
		assigned: make(map[int]bool),
	}
}

// Allocate assigns a free IP from the pool
func (p *IPPool) Allocate() (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for ip := p.minIP; ip <= p.maxIP; ip++ {
		if !p.assigned[ip] {
			p.assigned[ip] = true
			return ip, nil
		}
	}

	return 0, fmt.Errorf("no available IPs in pool (range: 10.8.0.%d-10.8.0.%d)", p.minIP, p.maxIP)
}

// Release returns an IP back to the pool
func (p *IPPool) Release(ip int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.assigned, ip)
}

// IsAssigned checks if an IP is currently assigned
func (p *IPPool) IsAssigned(ip int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.assigned[ip]
}

// Count returns the number of assigned IPs
func (p *IPPool) Count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.assigned)
}

// Available returns the number of available IPs
func (p *IPPool) Available() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	total := p.maxIP - p.minIP + 1
	return total - len(p.assigned)
}
