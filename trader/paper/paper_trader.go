package paper

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"nofx/market"
	"nofx/trader/types"
	"sync"
	"time"
)

// PaperTrader implements types.Trader for simulation.
//
// Design: Simulation must match live behaviour in every way except that funds are virtual
// and orders are not sent to an exchange. Do not simplify logic for simulation; the same
// flows (open/close, SL/TP checks, positions, orders in store, balance/margin) apply.
// SetStopLoss/SetTakeProfit/Cancel* are no-ops here because there is no exchange to send
// orders to; dynamic SL/TP is still applied by the strategy layer (checkDynamicStopLossTakeProfit).
type PaperTrader struct {
	exchangeType   string  // e.g. "binance", for market data
	initialBalance float64
	balance        float64 // current wallet balance (margin not in positions)
	positions      map[string]*paperPosition
	closedPnL      []types.ClosedPnLRecord
	leverage       map[string]int   // symbol -> leverage
	marginMode     map[string]bool  // symbol -> isCrossMargin
	orderIDGen     int
	mu             sync.RWMutex
}

type paperPosition struct {
	Symbol     string
	Side       string  // "LONG" or "SHORT"
	Quantity   float64
	EntryPrice float64
	Leverage   int
	EntryTime  time.Time
}

func posKey(symbol, side string) string {
	return symbol + "_" + side
}

// nextOrderID returns a globally-unique paper order id.
// IMPORTANT: must not repeat across process restarts, otherwise DB UNIQUE(exchange_order_id) will reject inserts
// and the UI will miss order records.
func (p *PaperTrader) nextOrderID() string {
	// Monotonic within this PaperTrader instance
	p.orderIDGen++

	// Add strong uniqueness across restarts and across multiple traders:
	// - UTC millis timestamp
	// - per-instance seq
	// - 4 random bytes
	nowMs := time.Now().UTC().UnixMilli()
	rb := make([]byte, 4)
	_, _ = rand.Read(rb)
	return fmt.Sprintf("paper_%d_%d_%s", nowMs, p.orderIDGen, hex.EncodeToString(rb))
}

// NewPaperTrader creates a paper/simulation trader with virtual balance; uses market data for prices.
func NewPaperTrader(initialBalance float64, exchangeType string) *PaperTrader {
	if exchangeType == "" {
		exchangeType = "binance"
	}
	return &PaperTrader{
		exchangeType:   exchangeType,
		initialBalance: initialBalance,
		balance:        initialBalance,
		positions:      make(map[string]*paperPosition),
		closedPnL:      nil,
		leverage:       make(map[string]int),
		marginMode:     make(map[string]bool),
		orderIDGen:     0,
	}
}

func (p *PaperTrader) getMarketPrice(symbol string) (float64, error) {
	symbol = market.Normalize(symbol)
	data, err := market.GetWithExchange(symbol, p.exchangeType)
	if err != nil {
		// Fallback: many symbols have Binance data even when paper exchange type differs
		data, err = market.Get(symbol)
		if err != nil {
			return 0, err
		}
	}
	return data.CurrentPrice, nil
}

// GetBalance returns virtual account balance (total equity = balance + unrealized PnL).
func (p *PaperTrader) GetBalance() (map[string]interface{}, error) {
	p.mu.RLock()
	balance := p.balance
	posCopy := make([]*paperPosition, 0, len(p.positions))
	for _, pos := range p.positions {
		posCopy = append(posCopy, &paperPosition{Symbol: pos.Symbol, Side: pos.Side, Quantity: pos.Quantity, EntryPrice: pos.EntryPrice})
	}
	p.mu.RUnlock()

	equity := balance
	for _, pos := range posCopy {
		price, err := p.getMarketPrice(pos.Symbol)
		if err != nil {
			continue
		}
		if pos.Side == "LONG" {
			equity += (price - pos.EntryPrice) * pos.Quantity
		} else {
			equity += (pos.EntryPrice - price) * pos.Quantity
		}
	}

	return map[string]interface{}{
		"total_equity": equity,
		"balance":      balance,
		"available":    balance,
	}, nil
}

// GetPositions returns virtual positions in exchange-like format.
func (p *PaperTrader) GetPositions() ([]map[string]interface{}, error) {
	p.mu.RLock()
	posCopy := make([]*paperPosition, 0, len(p.positions))
	for _, pos := range p.positions {
		if pos.Quantity > 0 {
			posCopy = append(posCopy, &paperPosition{Symbol: pos.Symbol, Side: pos.Side, Quantity: pos.Quantity, EntryPrice: pos.EntryPrice, Leverage: pos.Leverage, EntryTime: pos.EntryTime})
		}
	}
	p.mu.RUnlock()

	var out []map[string]interface{}
	for _, pos := range posCopy {
		price, _ := p.getMarketPrice(pos.Symbol)
		unrealized := 0.0
		if pos.Side == "LONG" {
			unrealized = (price - pos.EntryPrice) * pos.Quantity
		} else {
			unrealized = (pos.EntryPrice - price) * pos.Quantity
		}
		m := map[string]interface{}{
			"symbol":         pos.Symbol,
			"position_side":  pos.Side,
			"position_amt":   pos.Quantity,
			"entry_price":    pos.EntryPrice,
			"mark_price":     price,
			"unrealized_pnl": unrealized,
			"leverage":       pos.Leverage,
		}
		if !pos.EntryTime.IsZero() {
			m["update_time"] = pos.EntryTime.UnixMilli()
		}
		out = append(out, m)
	}
	return out, nil
}

func (p *PaperTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	symbol = market.Normalize(symbol)
	price, err := p.getMarketPrice(symbol)
	if err != nil {
		return nil, err
	}
	lev := leverage
	if lev <= 0 {
		lev = 1
	}
	margin := (quantity * price) / float64(lev)
	if p.balance < margin {
		return nil, fmt.Errorf("insufficient balance: need %.2f margin, have %.2f", margin, p.balance)
	}
	p.balance -= margin

	key := posKey(symbol, "LONG")
	if pos, ok := p.positions[key]; ok {
		// Average entry
		totalQty := pos.Quantity + quantity
		pos.EntryPrice = (pos.EntryPrice*pos.Quantity + price*quantity) / totalQty
		pos.Quantity = totalQty
		if leverage > 0 {
			pos.Leverage = leverage
		}
	} else {
		p.positions[key] = &paperPosition{
			Symbol:     symbol,
			Side:       "LONG",
			Quantity:   quantity,
			EntryPrice: price,
			Leverage:   leverage,
			EntryTime:  time.Now(),
		}
		if leverage > 0 {
			p.leverage[symbol] = leverage
		}
	}
	p.orderIDGen++
	return map[string]interface{}{"order_id": p.nextOrderID(), "status": "FILLED"}, nil
}

func (p *PaperTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	symbol = market.Normalize(symbol)
	price, err := p.getMarketPrice(symbol)
	if err != nil {
		return nil, err
	}
	lev := leverage
	if lev <= 0 {
		lev = 1
	}
	margin := (quantity * price) / float64(lev)
	if p.balance < margin {
		return nil, fmt.Errorf("insufficient balance: need %.2f margin, have %.2f", margin, p.balance)
	}
	p.balance -= margin

	key := posKey(symbol, "SHORT")
	if pos, ok := p.positions[key]; ok {
		totalQty := pos.Quantity + quantity
		pos.EntryPrice = (pos.EntryPrice*pos.Quantity + price*quantity) / totalQty
		pos.Quantity = totalQty
		if leverage > 0 {
			pos.Leverage = leverage
		}
	} else {
		p.positions[key] = &paperPosition{
			Symbol:     symbol,
			Side:       "SHORT",
			Quantity:   quantity,
			EntryPrice: price,
			Leverage:   leverage,
			EntryTime:  time.Now(),
		}
		if leverage > 0 {
			p.leverage[symbol] = leverage
		}
	}
	p.orderIDGen++
	return map[string]interface{}{"order_id": p.nextOrderID(), "status": "FILLED"}, nil
}

func (p *PaperTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	return p.closePosition(symbol, "LONG", quantity)
}

func (p *PaperTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	return p.closePosition(symbol, "SHORT", quantity)
}

func (p *PaperTrader) closePosition(symbol, side string, quantity float64) (map[string]interface{}, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	symbol = market.Normalize(symbol)
	key := posKey(symbol, side)
	pos, ok := p.positions[key]
	if !ok || pos.Quantity <= 0 {
		return nil, fmt.Errorf("no position to close")
	}
	if quantity <= 0 {
		quantity = pos.Quantity
	}
	if quantity > pos.Quantity {
		quantity = pos.Quantity
	}
	price, err := p.getMarketPrice(symbol)
	if err != nil {
		return nil, err
	}
	realized := 0.0
	if side == "LONG" {
		realized = (price - pos.EntryPrice) * quantity
	} else {
		realized = (pos.EntryPrice - price) * quantity
	}
	lev := pos.Leverage
	if lev <= 0 {
		lev = 1
	}
	marginReturned := (quantity * pos.EntryPrice) / float64(lev)
	p.balance += marginReturned + realized
	p.closedPnL = append(p.closedPnL, types.ClosedPnLRecord{
		Symbol:      symbol,
		Side:        side,
		EntryPrice:  pos.EntryPrice,
		ExitPrice:   price,
		Quantity:    quantity,
		RealizedPnL: realized,
		EntryTime:   pos.EntryTime,
		ExitTime:    time.Now(),
		CloseType:   "manual",
	})
	pos.Quantity -= quantity
	if pos.Quantity <= 0 {
		delete(p.positions, key)
	}
	return map[string]interface{}{"order_id": p.nextOrderID(), "status": "FILLED"}, nil
}

// RestoreOpenPosition 从 DB 恢复一条未平仓位（进程重启后调用），与实盘一致：扣减占用保证金。
// entryTimeMs 为 DB 中的入场时间（毫秒），传 0 则用当前时间（SL/TP 最小持仓会从恢复时刻算起）。
func (p *PaperTrader) RestoreOpenPosition(symbol, side string, quantity, entryPrice float64, leverage int, entryTimeMs int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	symbol = market.Normalize(symbol)
	lev := leverage
	if lev <= 0 {
		lev = 1
	}
	margin := (quantity * entryPrice) / float64(lev)
	if margin > p.balance {
		margin = p.balance
	}
	p.balance -= margin

	entryTime := time.Now()
	if entryTimeMs > 0 {
		entryTime = time.UnixMilli(entryTimeMs)
	}

	key := posKey(symbol, side)
	if pos, ok := p.positions[key]; ok {
		totalQty := pos.Quantity + quantity
		pos.EntryPrice = (pos.EntryPrice*pos.Quantity + entryPrice*quantity) / totalQty
		pos.Quantity = totalQty
		if leverage > 0 {
			pos.Leverage = leverage
		}
	} else {
		p.positions[key] = &paperPosition{
			Symbol:     symbol,
			Side:       side,
			Quantity:   quantity,
			EntryPrice: entryPrice,
			Leverage:   leverage,
			EntryTime:  entryTime,
		}
		if leverage > 0 {
			p.leverage[symbol] = leverage
		}
	}
}

// RestoreRealizedPnL restores cumulative realized PnL from DB (on process restart).
// This ensures balance reflects past closed trades.
func (p *PaperTrader) RestoreRealizedPnL(realizedPnL float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.balance += realizedPnL
}

func (p *PaperTrader) SetLeverage(symbol string, leverage int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.leverage[symbol] = leverage
	if pos, ok := p.positions[posKey(market.Normalize(symbol), "LONG")]; ok {
		pos.Leverage = leverage
	}
	if pos, ok := p.positions[posKey(market.Normalize(symbol), "SHORT")]; ok {
		pos.Leverage = leverage
	}
	return nil
}

func (p *PaperTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.marginMode[symbol] = isCrossMargin
	return nil
}

func (p *PaperTrader) GetMarketPrice(symbol string) (float64, error) {
	return p.getMarketPrice(symbol)
}

func (p *PaperTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	return nil
}
func (p *PaperTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	return nil
}
func (p *PaperTrader) CancelStopLossOrders(symbol string) error   { return nil }
func (p *PaperTrader) CancelTakeProfitOrders(symbol string) error  { return nil }
func (p *PaperTrader) CancelAllOrders(symbol string) error         { return nil }
func (p *PaperTrader) CancelStopOrders(symbol string) error       { return nil }

func (p *PaperTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	// Simple rounding for paper
	return fmt.Sprintf("%.4f", math.Round(quantity*10000)/10000), nil
}

func (p *PaperTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"status":       "FILLED",
		"avgPrice":     0,
		"executedQty":  0,
		"commission":   0,
	}, nil
}

func (p *PaperTrader) GetClosedPnL(startTime time.Time, limit int) ([]types.ClosedPnLRecord, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]types.ClosedPnLRecord, 0, len(p.closedPnL))
	for i := len(p.closedPnL) - 1; i >= 0 && len(out) < limit; i-- {
		if !p.closedPnL[i].ExitTime.Before(startTime) {
			out = append(out, p.closedPnL[i])
		}
	}
	return out, nil
}

func (p *PaperTrader) GetOpenOrders(symbol string) ([]types.OpenOrder, error) {
	return nil, nil
}

// Ensure PaperTrader implements types.Trader
var _ types.Trader = (*PaperTrader)(nil)
