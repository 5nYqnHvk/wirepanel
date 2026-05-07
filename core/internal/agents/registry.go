package agents

import (
	"sync"
	"time"
)

type Agent struct {
	ID         string    `json:"id"`
	Hostname   string    `json:"hostname"`
	OS         string    `json:"os"`
	Arch       string    `json:"arch"`
	Version    string    `json:"version"`
	ConnectedAt time.Time `json:"connected_at"`
	LastSeen   time.Time `json:"last_seen"`
	Online     bool      `json:"online"`
}

type Registry struct {
	mu     sync.RWMutex
	agents map[string]*Agent
}

func NewRegistry() *Registry {
	return &Registry{agents: make(map[string]*Agent)}
}

func (r *Registry) Add(a *Agent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a.ConnectedAt = time.Now()
	a.LastSeen = time.Now()
	a.Online = true
	r.agents[a.ID] = a
}

func (r *Registry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if a, ok := r.agents[id]; ok {
		a.Online = false
	}
}

func (r *Registry) Touch(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if a, ok := r.agents[id]; ok {
		a.LastSeen = time.Now()
	}
}

func (r *Registry) Get(id string) (*Agent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.agents[id]
	return a, ok
}

func (r *Registry) List() []*Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Agent, 0, len(r.agents))
	for _, a := range r.agents {
		out = append(out, a)
	}
	return out
}
