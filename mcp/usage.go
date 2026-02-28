package mcp

import "sync"

var (
	lastUsageMu       sync.RWMutex
	lastUsage         *TokenUsage
	lastUsageByRun    = make(map[string]*TokenUsage) // per backtest run_id for GET /api/ai-usage?run_id=
	lastUsageByTrader = make(map[string]*TokenUsage) // per trader_id for GET /api/ai-usage?trader_id=
	lastUsageByScope  = make(map[string]*TokenUsage)  // per context e.g. strategy_studio for GET /api/ai-usage?context=
)

// RecordTokenUsage stores token usage globally and, when runID is non-empty, per-run for that backtest.
func RecordTokenUsage(usage TokenUsage, runID string) {
	lastUsageMu.Lock()
	defer lastUsageMu.Unlock()
	u := usage
	lastUsage = &u
	if runID != "" {
		copy := u
		lastUsageByRun[runID] = &copy
	}
}

// RecordTokenUsageForTrader stores token usage for a specific trader (for real trading / simulation).
func RecordTokenUsageForTrader(usage TokenUsage, traderID string) {
	if traderID == "" {
		return
	}
	lastUsageMu.Lock()
	defer lastUsageMu.Unlock()
	u := usage
	lastUsage = &u // also update global for fallback
	lastUsageByTrader[traderID] = &u
}

// GetLastTokenUsage returns the most recent token usage (any call), if any.
func GetLastTokenUsage() *TokenUsage {
	lastUsageMu.RLock()
	defer lastUsageMu.RUnlock()
	if lastUsage == nil {
		return nil
	}
	u := *lastUsage
	return &u
}

// GetTokenUsageForRun returns the latest token usage for the given backtest run_id, if any.
func GetTokenUsageForRun(runID string) *TokenUsage {
	if runID == "" {
		return nil
	}
	lastUsageMu.RLock()
	defer lastUsageMu.RUnlock()
	if u := lastUsageByRun[runID]; u != nil {
		cp := *u
		return &cp
	}
	return nil
}

// GetTokenUsageForTrader returns the latest token usage for the given trader_id, if any.
func GetTokenUsageForTrader(traderID string) *TokenUsage {
	if traderID == "" {
		return nil
	}
	lastUsageMu.RLock()
	defer lastUsageMu.RUnlock()
	if u := lastUsageByTrader[traderID]; u != nil {
		cp := *u
		return &cp
	}
	return nil
}

// RecordTokenUsageForScope stores token usage for a specific scope (e.g. "strategy_studio").
func RecordTokenUsageForScope(usage TokenUsage, scope string) {
	if scope == "" {
		return
	}
	lastUsageMu.Lock()
	defer lastUsageMu.Unlock()
	u := usage
	lastUsage = &u
	copy := u
	lastUsageByScope[scope] = &copy
}

// GetTokenUsageForScope returns the latest token usage for the given scope, if any.
func GetTokenUsageForScope(scope string) *TokenUsage {
	if scope == "" {
		return nil
	}
	lastUsageMu.RLock()
	defer lastUsageMu.RUnlock()
	if u := lastUsageByScope[scope]; u != nil {
		cp := *u
		return &cp
	}
	return nil
}
