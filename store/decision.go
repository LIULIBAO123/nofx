package store

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// DecisionStore decision log storage
type DecisionStore struct {
	db *gorm.DB
}

// DecisionRecordDB internal GORM model for decision_records table
type DecisionRecordDB struct {
	ID                  int64     `gorm:"primaryKey;autoIncrement"`
	TraderID            string    `gorm:"column:trader_id;not null;index:idx_decision_records_trader_time"`
	CycleNumber         int       `gorm:"column:cycle_number;not null"`
	Timestamp           time.Time `gorm:"not null;index:idx_decision_records_trader_time,sort:desc;index:idx_decision_records_timestamp,sort:desc"`
	SystemPrompt        string    `gorm:"column:system_prompt;default:''"`
	InputPrompt         string    `gorm:"column:input_prompt;default:''"`
	CoTTrace            string    `gorm:"column:cot_trace;default:''"`
	DecisionJSON        string    `gorm:"column:decision_json;default:''"`
	RawResponse         string    `gorm:"column:raw_response;default:''"`
	CandidateCoins      string    `gorm:"column:candidate_coins;default:''"`
	ExecutionLog        string    `gorm:"column:execution_log;default:''"`
	Decisions           string    `gorm:"column:decisions;default:'[]'"`
	Success             bool      `gorm:"default:false"`
	ErrorMessage        string    `gorm:"column:error_message;default:''"`
	AIRequestDurationMs int64     `gorm:"column:ai_request_duration_ms;default:0"`
	CreatedAt           time.Time `json:"created_at"`
}

func (DecisionRecordDB) TableName() string { return "decision_records" }

// DecisionRecord decision record (external API struct)
type DecisionRecord struct {
	ID                  int64              `json:"id"`
	TraderID            string             `json:"trader_id"`
	CycleNumber         int                `json:"cycle_number"`
	Timestamp           time.Time          `json:"timestamp"`
	SystemPrompt        string             `json:"system_prompt"`
	InputPrompt         string             `json:"input_prompt"`
	CoTTrace            string             `json:"cot_trace"`
	DecisionJSON        string             `json:"decision_json"`
	RawResponse         string             `json:"raw_response"` // Raw AI response for debugging
	CandidateCoins      []string           `json:"candidate_coins"`
	ExecutionLog        []string           `json:"execution_log"`
	Success             bool               `json:"success"`
	ErrorMessage        string             `json:"error_message"`
	AIRequestDurationMs int64              `json:"ai_request_duration_ms"`
	AccountState        AccountSnapshot    `json:"account_state"`
	Positions           []PositionSnapshot `json:"positions"`
	Decisions           []DecisionAction   `json:"decisions"`
}

// AccountSnapshot account state snapshot
type AccountSnapshot struct {
	TotalBalance          float64 `json:"total_balance"`
	AvailableBalance      float64 `json:"available_balance"`
	TotalUnrealizedProfit float64 `json:"total_unrealized_profit"`
	PositionCount         int     `json:"position_count"`
	MarginUsedPct         float64 `json:"margin_used_pct"`
	InitialBalance        float64 `json:"initial_balance"`
}

// PositionSnapshot position snapshot
type PositionSnapshot struct {
	Symbol           string  `json:"symbol"`
	Side             string  `json:"side"`
	PositionAmt      float64 `json:"position_amt"`
	EntryPrice       float64 `json:"entry_price"`
	MarkPrice        float64 `json:"mark_price"`
	UnrealizedProfit float64 `json:"unrealized_profit"`
	Leverage         float64 `json:"leverage"`
	LiquidationPrice float64 `json:"liquidation_price"`
}

// DecisionAction decision action
type DecisionAction struct {
	Action     string    `json:"action"`
	Symbol     string    `json:"symbol"`
	Quantity   float64   `json:"quantity"`
	Leverage   int       `json:"leverage"`
	Price      float64   `json:"price"`
	StopLoss   float64   `json:"stop_loss,omitempty"`   // Stop loss price
	TakeProfit float64   `json:"take_profit,omitempty"` // Take profit price
	Confidence int       `json:"confidence,omitempty"`  // AI confidence (0-100)
	Reasoning  string    `json:"reasoning,omitempty"`   // Brief reasoning
	OrderID    int64     `json:"order_id"`
	Timestamp  time.Time `json:"timestamp"`
	Success    bool      `json:"success"`
	Error      string    `json:"error"`
}

// TradeAnalysis AI trade analysis result
type TradeAnalysis struct {
	Rating         string    `json:"rating"`          // excellent, good, fair, poor
	Summary        string    `json:"summary"`         // one-line summary
	ProfitAnalysis string    `json:"profit_analysis"` // detailed profit/loss analysis
	Improvements   []string  `json:"improvements"`    // improvement suggestions
	RiskWarnings   []string  `json:"risk_warnings"`   // risk warnings
	AnalyzedAt     time.Time `json:"analyzed_at"`     // analysis timestamp
}

// TradeEvent trade event (for backtest)
type TradeEvent struct {
	Timestamp       int64          `json:"ts"`
	Symbol          string         `json:"symbol"`
	Action          string         `json:"action"`
	Side            string         `json:"side,omitempty"`
	Quantity        float64        `json:"qty"`
	Price           float64        `json:"price"`
	Fee             float64        `json:"fee"`
	Slippage        float64        `json:"slippage"`
	OrderValue      float64        `json:"order_value"`
	RealizedPnL     float64        `json:"realized_pnl"`
	Leverage        int            `json:"leverage,omitempty"`
	Cycle           int            `json:"cycle"`
	PositionAfter   float64        `json:"position_after"`
	LiquidationFlag bool           `json:"liquidation"`
	Note            string         `json:"note,omitempty"`
	AIAnalysis      *TradeAnalysis `json:"ai_analysis,omitempty"` // AI analysis result
}

// Statistics statistics information
type Statistics struct {
	TotalCycles         int `json:"total_cycles"`
	SuccessfulCycles    int `json:"successful_cycles"`
	FailedCycles        int `json:"failed_cycles"`
	TotalOpenPositions  int `json:"total_open_positions"`
	TotalClosePositions int `json:"total_close_positions"`
}

// NewDecisionStore creates a new DecisionStore
func NewDecisionStore(db *gorm.DB) *DecisionStore {
	return &DecisionStore{db: db}
}

// initTables initializes AI decision log tables
func (s *DecisionStore) initTables() error {
	// For PostgreSQL with existing table, skip AutoMigrate
	if s.db.Dialector.Name() == "postgres" {
		var tableExists int64
		s.db.Raw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'decision_records'`).Scan(&tableExists)
		if tableExists > 0 {
			return nil
		}
	}
	return s.db.AutoMigrate(&DecisionRecordDB{})
}

// toRecord converts DB model to API struct
func (db *DecisionRecordDB) toRecord() *DecisionRecord {
	record := &DecisionRecord{
		ID:                  db.ID,
		TraderID:            db.TraderID,
		CycleNumber:         db.CycleNumber,
		Timestamp:           db.Timestamp,
		SystemPrompt:        db.SystemPrompt,
		InputPrompt:         db.InputPrompt,
		CoTTrace:            db.CoTTrace,
		DecisionJSON:        db.DecisionJSON,
		RawResponse:         db.RawResponse,
		Success:             db.Success,
		ErrorMessage:        db.ErrorMessage,
		AIRequestDurationMs: db.AIRequestDurationMs,
	}
	json.Unmarshal([]byte(db.CandidateCoins), &record.CandidateCoins)
	json.Unmarshal([]byte(db.ExecutionLog), &record.ExecutionLog)
	json.Unmarshal([]byte(db.Decisions), &record.Decisions)
	return record
}

// LogDecision logs decision
func (s *DecisionStore) LogDecision(record *DecisionRecord) error {
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now().UTC()
	} else {
		record.Timestamp = record.Timestamp.UTC()
	}

	// Serialize arrays to JSON
	candidateCoinsJSON, _ := json.Marshal(record.CandidateCoins)
	executionLogJSON, _ := json.Marshal(record.ExecutionLog)
	decisionsJSON, _ := json.Marshal(record.Decisions)

	dbRecord := &DecisionRecordDB{
		TraderID:            record.TraderID,
		CycleNumber:         record.CycleNumber,
		Timestamp:           record.Timestamp,
		SystemPrompt:        record.SystemPrompt,
		InputPrompt:         record.InputPrompt,
		CoTTrace:            record.CoTTrace,
		DecisionJSON:        record.DecisionJSON,
		RawResponse:         record.RawResponse,
		CandidateCoins:      string(candidateCoinsJSON),
		ExecutionLog:        string(executionLogJSON),
		Decisions:           string(decisionsJSON),
		Success:             record.Success,
		ErrorMessage:        record.ErrorMessage,
		AIRequestDurationMs: record.AIRequestDurationMs,
	}

	if err := s.db.Create(dbRecord).Error; err != nil {
		return fmt.Errorf("failed to insert decision record: %w", err)
	}
	record.ID = dbRecord.ID
	return nil
}

// GetLatestRecords gets the latest N records for specified trader (sorted by time in ascending order: old to new)
func (s *DecisionStore) GetLatestRecords(traderID string, n int) ([]*DecisionRecord, error) {
	var dbRecords []*DecisionRecordDB
	err := s.db.Where("trader_id = ?", traderID).
		Order("timestamp DESC").
		Limit(n).
		Find(&dbRecords).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query decision records: %w", err)
	}

	records := make([]*DecisionRecord, len(dbRecords))
	for i, db := range dbRecords {
		records[i] = db.toRecord()
	}

	// Reverse array to sort time from old to new
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	return records, nil
}

// GetAllLatestRecords gets the latest N records for all traders
func (s *DecisionStore) GetAllLatestRecords(n int) ([]*DecisionRecord, error) {
	var dbRecords []*DecisionRecordDB
	err := s.db.Order("timestamp DESC").Limit(n).Find(&dbRecords).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query decision records: %w", err)
	}

	records := make([]*DecisionRecord, len(dbRecords))
	for i, db := range dbRecords {
		records[i] = db.toRecord()
	}

	// Reverse array
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	return records, nil
}

// GetRecordsInTimeRange returns decision records for trader in [fromMs, toMs] (Unix milliseconds UTC).
func (s *DecisionStore) GetRecordsInTimeRange(traderID string, fromMs, toMs int64) ([]*DecisionRecord, error) {
	from := time.UnixMilli(fromMs).UTC()
	to := time.UnixMilli(toMs).UTC()
	var dbRecords []*DecisionRecordDB
	err := s.db.Where("trader_id = ? AND timestamp >= ? AND timestamp <= ?", traderID, from, to).
		Order("timestamp ASC").
		Find(&dbRecords).Error
	if err != nil {
		return nil, err
	}
	records := make([]*DecisionRecord, len(dbRecords))
	for i, db := range dbRecords {
		records[i] = db.toRecord()
	}
	return records, nil
}

// GetInferredCloseReason finds a close_long/close_short decision near exitTimeMs for the same symbol/side
// and returns a short label and the decision Reasoning. Used when position close_reason is empty/sync/unknown.
// Symbol matching is normalized (VVV vs VVVUSDT both match).
func (s *DecisionStore) GetInferredCloseReason(traderID, symbol, side string, exitTimeMs int64) (reasonLabel string, reasoning string, ok bool) {
	const windowBeforeMs = 5 * 60 * 1000  // 5 min before
	const windowAfterMs = 2 * 60 * 1000   // 2 min after
	fromMs := exitTimeMs - windowBeforeMs
	toMs := exitTimeMs + windowAfterMs
	records, err := s.GetRecordsInTimeRange(traderID, fromMs, toMs)
	if err != nil || len(records) == 0 {
		return "", "", false
	}
	normSymbol := strings.ToUpper(symbol)
	if !strings.HasSuffix(normSymbol, "USDT") {
		normSymbol = normSymbol + "USDT"
	}
	expectAction := "close_long"
	if strings.ToUpper(side) == "SHORT" {
		expectAction = "close_short"
	}
	for _, rec := range records {
		for _, d := range rec.Decisions {
			if d.Action != expectAction {
				continue
			}
			ds := strings.ToUpper(strings.TrimSpace(d.Symbol))
			if ds == "" {
				continue
			}
			if !strings.HasSuffix(ds, "USDT") {
				ds = ds + "USDT"
			}
			if ds != normSymbol {
				continue
			}
			reasoning = strings.TrimSpace(d.Reasoning)
			if reasoning == "" {
				reasoning = d.Action
			}
			reasonLabel = inferCloseReasonLabel(reasoning)
			return reasonLabel, reasoning, true
		}
	}
	return "", "", false
}

func inferCloseReasonLabel(reasoning string) string {
	r := strings.ToLower(reasoning)
	if strings.Contains(r, "take profit") || strings.Contains(r, "止盈") {
		return "系统止盈"
	}
	if strings.Contains(r, "stop loss") || strings.Contains(r, "止损") {
		return "系统止损"
	}
	return "系统平仓"
}

// GetRecordsByDate gets all records for a specified trader on a specified date
func (s *DecisionStore) GetRecordsByDate(traderID string, date time.Time) ([]*DecisionRecord, error) {
	dateStr := date.Format("2006-01-02")

	var dbRecords []*DecisionRecordDB
	err := s.db.Where("trader_id = ? AND DATE(timestamp) = ?", traderID, dateStr).
		Order("timestamp ASC").
		Find(&dbRecords).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query decision records: %w", err)
	}

	records := make([]*DecisionRecord, len(dbRecords))
	for i, db := range dbRecords {
		records[i] = db.toRecord()
	}

	return records, nil
}

// CleanOldRecords cleans old records from N days ago
func (s *DecisionStore) CleanOldRecords(traderID string, days int) (int64, error) {
	cutoffTime := time.Now().AddDate(0, 0, -days)

	result := s.db.Where("trader_id = ? AND timestamp < ?", traderID, cutoffTime).
		Delete(&DecisionRecordDB{})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to clean old records: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// GetStatistics gets statistics information for specified trader
func (s *DecisionStore) GetStatistics(traderID string) (*Statistics, error) {
	stats := &Statistics{}

	var totalCount, successCount int64
	s.db.Model(&DecisionRecordDB{}).Where("trader_id = ?", traderID).Count(&totalCount)
	s.db.Model(&DecisionRecordDB{}).Where("trader_id = ? AND success = ?", traderID, true).Count(&successCount)

	stats.TotalCycles = int(totalCount)
	stats.SuccessfulCycles = int(successCount)
	stats.FailedCycles = stats.TotalCycles - stats.SuccessfulCycles

	// Count from trader_positions table using raw query for cross-table
	s.db.Raw("SELECT COUNT(*) FROM trader_positions WHERE trader_id = ?", traderID).Scan(&stats.TotalOpenPositions)
	s.db.Raw("SELECT COUNT(*) FROM trader_positions WHERE trader_id = ? AND status = 'CLOSED'", traderID).Scan(&stats.TotalClosePositions)

	return stats, nil
}

// GetAllStatistics gets statistics information for all traders
func (s *DecisionStore) GetAllStatistics() (*Statistics, error) {
	stats := &Statistics{}

	var totalCount, successCount int64
	s.db.Model(&DecisionRecordDB{}).Count(&totalCount)
	s.db.Model(&DecisionRecordDB{}).Where("success = ?", true).Count(&successCount)

	stats.TotalCycles = int(totalCount)
	stats.SuccessfulCycles = int(successCount)
	stats.FailedCycles = stats.TotalCycles - stats.SuccessfulCycles

	// Count from trader_positions table
	s.db.Raw("SELECT COUNT(*) FROM trader_positions").Scan(&stats.TotalOpenPositions)
	s.db.Raw("SELECT COUNT(*) FROM trader_positions WHERE status = 'CLOSED'").Scan(&stats.TotalClosePositions)

	return stats, nil
}

// GetLastCycleNumber gets the last cycle number for specified trader
func (s *DecisionStore) GetLastCycleNumber(traderID string) (int, error) {
	var cycleNumber *int
	err := s.db.Model(&DecisionRecordDB{}).
		Where("trader_id = ?", traderID).
		Select("MAX(cycle_number)").
		Scan(&cycleNumber).Error
	if err != nil {
		return 0, err
	}
	if cycleNumber == nil {
		return 0, nil
	}
	return *cycleNumber, nil
}
