// internal/domain/trade.go
package domain

// TradeStatus represents the lifecycle state of a trade or allocation.
type TradeStatus string

const (
	StatusPending  TradeStatus = "PENDING"
	StatusFilled   TradeStatus = "FILLED"
	StatusRejected TradeStatus = "REJECTED"
	StatusPartial  TradeStatus = "PARTIAL"
)

// MasterTrade is the platform's own execution record.
type MasterTrade struct {
	ID       string
	Signal   TradeSignal
	Status   TradeStatus
	FilledAt float64 // actual fill price
}

// UserAllocation is a single user's pro-rata slice of the master trade.
type UserAllocation struct {
	UserID     string
	ContractID string
	Side       string
	AllocUSD   float64 // Pro-rata allocation in USD
	Price      float64 // Execution price (odds)
	Status     TradeStatus
}
