// internal/domain/user.go
package domain

import "sync"

// RiskSettings govern how much of a signal a follower receives.
type RiskSettings struct {
	MaxPositionUSD float64
	RiskFraction   float64 // 0.01–1.0: portion of balance to deploy per signal
}

// User holds mutable portfolio state protected by the engine's RWMutex.
type User struct {
	ID           string
	BalanceUSD   float64
	RiskSettings RiskSettings
}

// UserRegistry is a thread-safe in-memory store of all follower states.
type UserRegistry struct {
	mu    sync.RWMutex
	users map[string]*User
}

func NewUserRegistry() *UserRegistry {
	return &UserRegistry{users: make(map[string]*User)}
}

func (r *UserRegistry) Get(id string) (*User, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.users[id]
	return u, ok
}

func (r *UserRegistry) Set(u *User) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.users[u.ID] = u
}

func (r *UserRegistry) All() []*User {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*User, 0, len(r.users))
	for _, u := range r.users {
		out = append(out, u)
	}
	return out
}
