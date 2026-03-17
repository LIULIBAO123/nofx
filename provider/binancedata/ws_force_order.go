// Package binancedata 强平流 WebSocket：订阅币安 @forceOrder，在本地按 1h/4h 窗口聚合多空强平金额，供 nofx Context 使用。
package binancedata

import (
	"encoding/json"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"nofx/kernel"
)

const (
	wsForceOrderURL = "wss://fstream.binance.com/stream?streams=!forceOrder@arr"
	window1h        = 1 * time.Hour
	window4h        = 4 * time.Hour
	window24h       = 24 * time.Hour
	maxEvents       = 50000 // 单窗口最多保留事件数，防止内存膨胀
)

type forceOrderEvent struct {
	EventType string `json:"e"`
	EventTime int64  `json:"E"`
	Order     struct {
		Symbol     string `json:"s"`
		Side       string `json:"S"` // SELL = 多单强平, BUY = 空单强平
		Quantity   string `json:"q"`
		Price      string `json:"p"`
		TradeTime  int64  `json:"T"`
	} `json:"o"`
}

var (
	forceOrderMu     sync.RWMutex
	forceOrderEvents []forceOrderItem
	forceOrderSnap   *kernel.LiquidationAggSnapshot
)

func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

func dialBinanceForceOrderWS() (*websocket.Conn, error) {
	conn, _, err := websocket.DefaultDialer.Dial(wsForceOrderURL, nil)
	return conn, err
}

type forceOrderItem struct {
	Ts       int64
	LongUSD  float64 // 若为多单强平则填金额
	ShortUSD float64 // 若为空单强平则填金额
}

func init() {
	forceOrderSnap = &kernel.LiquidationAggSnapshot{Source: "binance_ws"}
}

// RunForceOrderWS 在后台运行币安强平流 WebSocket，持续写入内存聚合；可多次调用，仅维护单连接（会重连）。
func RunForceOrderWS() {
	go runForceOrderWSLoop()
}

func runForceOrderWSLoop() {
	for {
		if err := runForceOrderWSOnce(); err != nil {
			log.Printf("binance forceOrder ws: %v, reconnect in 30s", err)
		}
		time.Sleep(30 * time.Second)
	}
}

func runForceOrderWSOnce() error {
	// 使用标准库 websocket 建立连接（兼容 go 1.22+）
	conn, err := dialBinanceForceOrderWS()
	if err != nil {
		return err
	}
	defer conn.Close()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		var raw struct {
			Stream string          `json:"stream"`
			Data  json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(msg, &raw); err != nil {
			continue
		}
		var ev forceOrderEvent
		if err := json.Unmarshal(raw.Data, &ev); err != nil {
			continue
		}
		if ev.EventType != "forceOrder" {
			continue
		}
		qty, _ := parseFloat(ev.Order.Quantity)
		price, _ := parseFloat(ev.Order.Price)
		usd := qty * price
		ts := ev.Order.TradeTime
		if ts == 0 {
			ts = ev.EventTime
		}
		item := forceOrderItem{Ts: ts}
		switch ev.Order.Side {
		case "SELL":
			item.LongUSD = usd
		case "BUY":
			item.ShortUSD = usd
		default:
			continue
		}
		forceOrderMu.Lock()
		forceOrderEvents = append(forceOrderEvents, item)
		if len(forceOrderEvents) > maxEvents*2 {
			forceOrderEvents = forceOrderEvents[len(forceOrderEvents)-maxEvents:]
		}
		forceOrderSnap = aggregateForceOrderLocked()
		forceOrderMu.Unlock()
	}
}

func aggregateForceOrderLocked() *kernel.LiquidationAggSnapshot {
	now := time.Now().UnixMilli()
	cut1h := now - window1h.Milliseconds()
	cut4h := now - window4h.Milliseconds()
	cut24h := now - window24h.Milliseconds()
	var long1h, short1h, long4h, short4h, long24h, short24h float64
	for i := len(forceOrderEvents) - 1; i >= 0; i-- {
		it := forceOrderEvents[i]
		if it.Ts >= cut24h {
			long24h += it.LongUSD
			short24h += it.ShortUSD
		}
		if it.Ts >= cut4h {
			long4h += it.LongUSD
			short4h += it.ShortUSD
		}
		if it.Ts >= cut1h {
			long1h += it.LongUSD
			short1h += it.ShortUSD
		}
	}
	return &kernel.LiquidationAggSnapshot{
		Long1hUSD:   long1h,
		Short1hUSD:  short1h,
		Long4hUSD:   long4h,
		Short4hUSD:  short4h,
		Long24hUSD:  long24h,
		Short24hUSD: short24h,
		Source:      "binance_ws",
		UpdatedAt:   now,
	}
}

// GetLiquidationAgg 返回当前 1h/4h 强平聚合快照（只读）；若未启动 WS 或尚无数据则返回 nil。
func GetLiquidationAgg() *kernel.LiquidationAggSnapshot {
	forceOrderMu.RLock()
	defer forceOrderMu.RUnlock()
	if forceOrderSnap == nil {
		return nil
	}
	// 返回副本，避免调用方修改
	s := *forceOrderSnap
	return &s
}

// RefreshLiquidationAgg 根据当前时间重新聚合窗口并更新快照（供周期逻辑在读取前调用一次，保证窗口边界正确）
func RefreshLiquidationAgg() {
	forceOrderMu.Lock()
	defer forceOrderMu.Unlock()
	forceOrderSnap = aggregateForceOrderLocked()
}
