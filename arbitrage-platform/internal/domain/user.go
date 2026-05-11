// internal/domain/user.go
package domain

import "sync"

// RiskSettings govern how much of a signal a user receives.
type RiskSettings struct {
	MaxPositionUSD float64
	RiskFraction   float64 // 0.01–1.0: portion of balance to deploy per signal
}

// User holds mutable portfolio state protected by the engine's RWMutex.
type User struct {
	ID           string
	BalanceUSD   float64
	TotalPnL     float64 // Realized Profit and Loss
	RiskSettings RiskSettings
}

// UserRegistry is a thread-safe in-memory store of all user states.
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

// DeductBalance safely deducts the specified amount from the user's balance.
// It returns the new balance and a boolean indicating if the user was found.
func (r *UserRegistry) DeductBalance(id string, amount float64) (float64, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.users[id]
	if !ok {
		return 0, false
	}

	// Ensure balance doesn't go below 0
	if u.BalanceUSD >= amount {
		u.BalanceUSD -= amount
	} else {
		u.BalanceUSD = 0
	}

	return u.BalanceUSD, true
}

// CreditBalance safely adds the specified amount to the user's balance and updates PnL.
func (r *UserRegistry) CreditBalance(id string, amount float64, pnl float64) (float64, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.users[id]
	if !ok {
		return 0, false
	}

	u.BalanceUSD += amount
	u.TotalPnL += pnl

	return u.BalanceUSD, true
}
