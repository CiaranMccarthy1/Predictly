// internal/service/metrics.go
package service

import "sync/atomic"

// Metrics holds global counters updated atomically — no mutex overhead.
type Metrics struct {
	SignalsEvaluated atomic.Int64
	TradesExecuted   atomic.Int64
	FollowerFills    atomic.Int64
	RejectedSignals  atomic.Int64
}

// Global is the singleton metrics instance used across all service components.
var Global Metrics
