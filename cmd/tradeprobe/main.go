package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type TraderRow struct {
	ID                         string `json:"id"`
	Name                       string `json:"name"`
	IsSimulation               bool   `json:"is_simulation"`
	ScanIntervalMinutes        int    `json:"scan_interval_minutes"`
	SystemIntervalMinutes      int    `json:"system_interval_minutes"`
	SLTPAnalysisIntervalMinutes int   `json:"sltp_analysis_interval_minutes"`
}

type PositionRow struct {
	ID                    int64   `json:"id"`
	TraderID              string  `json:"trader_id"`
	Symbol                string  `json:"symbol"`
	Side                  string  `json:"side"`
	EntryPrice            float64 `json:"entry_price"`
	EntryQuantity         float64 `json:"entry_quantity"`
	Quantity              float64 `json:"quantity"`
	EntryTime             int64   `json:"entry_time"`
	ExitPrice             float64 `json:"exit_price"`
	ExitTime              int64   `json:"exit_time"`
	RealizedPnL           float64 `json:"realized_pnl"`
	Fee                   float64 `json:"fee"`
	Status                string  `json:"status"`
	CloseReason           string  `json:"close_reason"`
	CloseReasonAIAdjusted bool    `json:"close_reason_ai_adjusted"`
	CloseReasonTriggerDetail string `json:"close_reason_trigger_detail"`
	CloseEvents           string  `json:"close_events"`
	SLTPExitSignalState   string  `json:"sltp_exit_signal_state"`
	SLTPExitSignalAt      int64   `json:"sltp_exit_signal_at"`
}

type OrderRow struct {
	ID               int64   `json:"id"`
	TraderID         string  `json:"trader_id"`
	Symbol           string  `json:"symbol"`
	Side             string  `json:"side"`
	PositionSide     string  `json:"position_side"`
	Type             string  `json:"type"`
	Quantity         float64 `json:"quantity"`
	Price            float64 `json:"price"`
	FilledQuantity   float64 `json:"filled_quantity"`
	AvgFillPrice     float64 `json:"avg_fill_price"`
	Status           string  `json:"status"`
	OrderAction      string  `json:"order_action"`
	CreatedAt        int64   `json:"created_at"`
	FilledAt         int64   `json:"filled_at"`
	ExchangeOrderID  string  `json:"exchange_order_id"`
	ClientOrderID    string  `json:"client_order_id"`
	RelatedPositionID int64  `json:"related_position_id"`
}

type DecisionRow struct {
	ID            int64  `json:"id"`
	TraderID      string `json:"trader_id"`
	CycleNumber   int    `json:"cycle_number"`
	Timestamp     string `json:"timestamp"`
	Success       bool   `json:"success"`
	ErrorMessage  string `json:"error_message"`
	RawResponse   string `json:"raw_response"`
	ExecutionLog  string `json:"execution_log"`
}

type Output struct {
	Traders   []TraderRow   `json:"traders"`
	Positions []PositionRow `json:"positions"`
	Orders    []OrderRow    `json:"orders"`
	Decisions []DecisionRow `json:"decisions"`
	Meta      map[string]any `json:"meta"`
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func normalizeSymbol(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

func main() {
	var dbPath string
	var symbol string
	var traderID string
	var limitDecisions int
	var outPath string

	flag.StringVar(&dbPath, "db", "", "path to sqlite db (data.db)")
	flag.StringVar(&symbol, "symbol", "BTCUSDT", "symbol (e.g., BTCUSDT)")
	flag.StringVar(&traderID, "trader", "", "trader id (optional)")
	flag.IntVar(&limitDecisions, "decisions", 300, "max decision_records to export (filtered by trader+time range when possible)")
	flag.StringVar(&outPath, "out", "", "output json path (default next to db)")
	flag.Parse()

	if dbPath == "" {
		must(fmt.Errorf("missing -db"))
	}
	symbol = normalizeSymbol(symbol)

	if outPath == "" {
		outPath = filepath.Join(filepath.Dir(dbPath), fmt.Sprintf("tradeprobe_%s.json", strings.ToLower(symbol)))
	}

	db, err := sql.Open("sqlite", dbPath)
	must(err)
	defer db.Close()

	out := Output{Meta: map[string]any{}}
	out.Meta["db"] = dbPath
	out.Meta["symbol"] = symbol
	out.Meta["generated_at"] = time.Now().Format(time.RFC3339)

	// traders
	{
		rows, err := db.Query(`select id,name,is_simulation,scan_interval_minutes,system_interval_minutes,sltp_analysis_interval_minutes from traders`)
		must(err)
		defer rows.Close()
		for rows.Next() {
			var r TraderRow
			must(rows.Scan(&r.ID, &r.Name, &r.IsSimulation, &r.ScanIntervalMinutes, &r.SystemIntervalMinutes, &r.SLTPAnalysisIntervalMinutes))
			out.Traders = append(out.Traders, r)
		}
		must(rows.Err())
	}

	// positions for symbol
	{
		q := `select id,trader_id,symbol,side,entry_price,entry_quantity,quantity,entry_time,exit_price,exit_time,realized_pnl,fee,status,close_reason,close_reason_ai_adjusted,close_reason_trigger_detail,close_events,sltp_exit_signal_state,sltp_exit_signal_at
from trader_positions where symbol=? order by entry_time desc limit 50`
		rows, err := db.Query(q, symbol)
		must(err)
		defer rows.Close()
		for rows.Next() {
			var r PositionRow
			must(rows.Scan(&r.ID, &r.TraderID, &r.Symbol, &r.Side, &r.EntryPrice, &r.EntryQuantity, &r.Quantity, &r.EntryTime, &r.ExitPrice, &r.ExitTime, &r.RealizedPnL, &r.Fee, &r.Status, &r.CloseReason, &r.CloseReasonAIAdjusted, &r.CloseReasonTriggerDetail, &r.CloseEvents, &r.SLTPExitSignalState, &r.SLTPExitSignalAt))
			out.Positions = append(out.Positions, r)
		}
		must(rows.Err())
	}

	// infer trader id if not provided: pick the trader with most recent position for symbol
	if traderID == "" && len(out.Positions) > 0 {
		traderID = out.Positions[0].TraderID
	}
	out.Meta["trader_id"] = traderID

	// orders for symbol + trader
	if traderID != "" {
		q := `select id,trader_id,symbol,side,position_side,type,quantity,price,filled_quantity,avg_fill_price,status,order_action,created_at,filled_at,exchange_order_id,client_order_id,related_position_id
from trader_orders where trader_id=? and symbol=? order by created_at desc limit 500`
		rows, err := db.Query(q, traderID, symbol)
		must(err)
		defer rows.Close()
		for rows.Next() {
			var r OrderRow
			must(rows.Scan(&r.ID, &r.TraderID, &r.Symbol, &r.Side, &r.PositionSide, &r.Type, &r.Quantity, &r.Price, &r.FilledQuantity, &r.AvgFillPrice, &r.Status, &r.OrderAction, &r.CreatedAt, &r.FilledAt, &r.ExchangeOrderID, &r.ClientOrderID, &r.RelatedPositionID))
			out.Orders = append(out.Orders, r)
		}
		must(rows.Err())
	}

	// decisions: try to restrict by time window around most recent position
	var tStart, tEnd time.Time
	if traderID != "" && len(out.Positions) > 0 {
		p := out.Positions[0]
		// entry/exit are unix ms UTC
		entry := time.UnixMilli(p.EntryTime).UTC()
		end := time.Now().UTC()
		if p.ExitTime > 0 {
			end = time.UnixMilli(p.ExitTime).UTC()
		}
		tStart = entry.Add(-2 * time.Hour)
		tEnd = end.Add(2 * time.Hour)
		out.Meta["window_start_utc"] = tStart.Format(time.RFC3339)
		out.Meta["window_end_utc"] = tEnd.Format(time.RFC3339)
	}

	if traderID != "" && !tStart.IsZero() {
		q := `select id,trader_id,cycle_number,timestamp,success,error_message,raw_response,execution_log
from decision_records where trader_id=? and timestamp between ? and ? order by timestamp asc limit ?`
		rows, err := db.Query(q, traderID, tStart.Format("2006-01-02 15:04:05"), tEnd.Format("2006-01-02 15:04:05"), limitDecisions)
		must(err)
		defer rows.Close()
		for rows.Next() {
			var r DecisionRow
			must(rows.Scan(&r.ID, &r.TraderID, &r.CycleNumber, &r.Timestamp, &r.Success, &r.ErrorMessage, &r.RawResponse, &r.ExecutionLog))
			out.Decisions = append(out.Decisions, r)
		}
		must(rows.Err())
	} else if traderID != "" {
		q := `select id,trader_id,cycle_number,timestamp,success,error_message,raw_response,execution_log
from decision_records where trader_id=? order by timestamp desc limit ?`
		rows, err := db.Query(q, traderID, limitDecisions)
		must(err)
		defer rows.Close()
		for rows.Next() {
			var r DecisionRow
			must(rows.Scan(&r.ID, &r.TraderID, &r.CycleNumber, &r.Timestamp, &r.Success, &r.ErrorMessage, &r.RawResponse, &r.ExecutionLog))
			out.Decisions = append(out.Decisions, r)
		}
		must(rows.Err())
	}

	b, err := json.MarshalIndent(out, "", "  ")
	must(err)
	must(os.WriteFile(outPath, b, 0o644))
	fmt.Println(outPath)
}

