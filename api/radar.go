// Package api: 多空雷达与挂单流程信息 API（与用户分享的截图功能对应）
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"nofx/kernel"
)

// RadarConfig 多空雷达配置（允许做多/做空、模式、指标阈值等）
type RadarConfig struct {
	AllowLong          bool    `json:"allow_long"`
	AllowShort         bool    `json:"allow_short"`
	AIAutoAnalysis     bool    `json:"ai_auto_analysis"`
	Mode               string  `json:"mode"` // "preset" | "manual" | "close_auto"
	HeatScore          float64 `json:"heat_score"`
	ATRPct             float64 `json:"atr_pct"`
	CapitalCriticalPct float64 `json:"capital_critical_pct"`
	ReliabilityGatePct float64 `json:"reliability_gate_pct"`
	DirectionQuantile  float64 `json:"direction_quantile"`
	DirectionQuery     int     `json:"direction_query"`
	ExcludeHeld        bool    `json:"exclude_held"`
	ExcludePending     bool    `json:"exclude_pending"`
	PendingOrderCapPct float64 `json:"pending_order_cap_pct"`
	LongPoolAutoIssue  bool    `json:"long_pool_auto_issue"`
	LongPoolAutoCancel bool    `json:"long_pool_auto_cancel"`
	ShortPoolAutoIssue bool    `json:"short_pool_auto_issue"`
	ShortPoolAutoCancel bool   `json:"short_pool_auto_cancel"`
	PoolQuantilePct    float64 `json:"pool_quantile_pct"`
}

// OrderFlowInfo 挂单流程信息（第一层/第二层/第三层管线与不达标统计）
type OrderFlowInfo struct {
	UpdatedAt           string                       `json:"updated_at"`
	ProcessStage        string                       `json:"process_stage"`
	Pipeline            OrderFlowPipeline            `json:"pipeline"`
	Layer1FailureStats  []Layer1FailureStat         `json:"layer1_failure_stats"`
	PerCoinFailures     []PerCoinFailure             `json:"per_coin_failures"`
	PipelineDescription string                       `json:"pipeline_description"`
}

type OrderFlowPipeline struct {
	PoolLong     int    `json:"pool_long"`
	PoolShort    int    `json:"pool_short"`
	AfterLayer1  int    `json:"after_layer1"`
	AfterLayer2  int    `json:"after_layer2"`
	ToSubmit     int    `json:"to_submit"`
	FlowLabel    string `json:"flow_label"`
}

type Layer1FailureStat struct {
	Condition string `json:"condition"`
	Count     int    `json:"count"`
}

type PerCoinFailure struct {
	Symbol   string   `json:"symbol"`
	Reasons  []string `json:"reasons"`
}

// DirectionPoolItem 方向池单项（多/空）
type DirectionPoolItem struct {
	Symbol          string  `json:"symbol"`
	StrengthPct     float64 `json:"strength_pct"`
	Score           float64 `json:"score"`
	MarketCondition string  `json:"market_condition"`
	ReliabilityPct  float64 `json:"reliability_pct"`
	Timing          string  `json:"timing"`
	VolumePricePct  float64 `json:"volume_price_pct"`
	FromAI          bool    `json:"from_ai,omitempty"`
}

// DirectionPoolResponse 方向池(多)+方向池(空)
type DirectionPoolResponse struct {
	Long  []DirectionPoolItem `json:"long"`
	Short []DirectionPoolItem `json:"short"`
}

// LatestAnalysisResponse 最新周期 AI 分析快照（供「实时数据+AI预测」展示）
type LatestAnalysisResponse struct {
	MarketRegime            string                         `json:"market_regime,omitempty"`
	Scenario                string                         `json:"scenario,omitempty"`
	MarketSummary           string                         `json:"market_summary,omitempty"`
	RiskAlert               bool                           `json:"risk_alert,omitempty"`
	SymbolPredictions       []SymbolPredictionItem         `json:"symbol_predictions,omitempty"`
	PositionSLTPAdjustments []kernel.PositionSLTPAdjustment `json:"position_sl_tp_adjustments,omitempty"`
	PositionSLTPBaselines   []kernel.PositionSLTPBaseline   `json:"position_sl_tp_baselines,omitempty"` // 调节前策略基线，用于展示「原始→调整后」
	UpdatedAt               string                         `json:"updated_at,omitempty"`
}

// SymbolPredictionItem 与 kernel.SymbolPrediction 一致，供 JSON 序列化
type SymbolPredictionItem struct {
	Symbol             string `json:"symbol"`
	PredictedDirection string `json:"predicted_direction,omitempty"`
	Confidence         int    `json:"confidence,omitempty"`
	SuggestExit        bool   `json:"suggest_exit,omitempty"`
	SuggestOpen        *bool  `json:"suggest_open,omitempty"`
}

func defaultRadarConfig() RadarConfig {
	return RadarConfig{
		AllowLong:           true,
		AllowShort:          true,
		AIAutoAnalysis:      true,
		Mode:                "close_auto",
		HeatScore:           0.7,
		ATRPct:              0.6,
		ReliabilityGatePct:  40,
		DirectionQuantile:   0.72,
		DirectionQuery:      3,
		ExcludeHeld:         true,
		ExcludePending:      true,
		PendingOrderCapPct:  5,
		LongPoolAutoIssue:   true,
		LongPoolAutoCancel:  true,
		ShortPoolAutoIssue:  true,
		ShortPoolAutoCancel: true,
		PoolQuantilePct:     40,
	}
}

// handleGetRadarConfig 获取多空雷达配置（优先从数据库读取，无则返回默认）
func (s *Server) handleGetRadarConfig(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	if _, err := s.store.Trader().GetFullConfig(userID, traderID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "trader not found or no access"})
		return
	}
	cfg := defaultRadarConfig()
	raw, err := s.store.Trader().GetRadarConfigBytes(traderID)
	if err == nil && len(raw) > 0 {
		_ = json.Unmarshal(raw, &cfg)
	}
	c.JSON(http.StatusOK, cfg)
}

// handlePutRadarConfig 更新多空雷达配置（持久化到数据库）
func (s *Server) handlePutRadarConfig(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	if _, err := s.store.Trader().GetFullConfig(userID, traderID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "trader not found or no access"})
		return
	}
	var req RadarConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	raw, err := json.Marshal(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal config"})
		return
	}
	if err := s.store.Trader().PutRadarConfigBytes(traderID, raw); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// handleGetOrderFlowInfo 获取挂单流程信息（管线、第一层不达标统计、按币种不达标）
// 优先返回 kernel.RunPipeline 写入的实时状态；无则返回占位数据
func (s *Server) handleGetOrderFlowInfo(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	if _, err := s.store.Trader().GetFullConfig(userID, traderID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "trader not found or no access"})
		return
	}
	st := kernel.GetPipelineState(traderID)
	if st != nil {
		info := OrderFlowInfo{
			UpdatedAt:           st.UpdatedAt.Format("15:04:05"),
			ProcessStage:        st.ProcessStage,
			PipelineDescription: st.PipelineDescription,
			Pipeline: OrderFlowPipeline{
				PoolLong:    st.PoolLongCount,
				PoolShort:   st.PoolShortCount,
				AfterLayer1: st.AfterLayer1Count,
				AfterLayer2: st.AfterLayer2Count,
				ToSubmit:    st.ToSubmitCount,
				FlowLabel:   st.FlowLabel,
			},
			Layer1FailureStats: make([]Layer1FailureStat, 0, len(st.Layer1FailureStats)),
			PerCoinFailures:    make([]PerCoinFailure, 0, len(st.PerCoinFailures)),
		}
		for _, v := range st.Layer1FailureStats {
			info.Layer1FailureStats = append(info.Layer1FailureStats, Layer1FailureStat{Condition: v.Condition, Count: v.Count})
		}
		for _, v := range st.PerCoinFailures {
			info.PerCoinFailures = append(info.PerCoinFailures, PerCoinFailure{Symbol: v.Symbol, Reasons: v.Reasons})
		}
		c.JSON(http.StatusOK, info)
		return
	}
	// 占位：交易员尚未跑过周期或未启用多层过滤
	info := OrderFlowInfo{
		UpdatedAt:    time.Now().Format("15:04:05"),
		ProcessStage: "暂无流程数据（请启动交易员并等待至少一个周期）",
		Pipeline: OrderFlowPipeline{
			PoolLong:    0,
			PoolShort:   0,
			AfterLayer1: 0,
			AfterLayer2: 0,
			ToSubmit:    0,
			FlowLabel:   "0 → 0 → 0 → 0",
		},
		Layer1FailureStats: []Layer1FailureStat{},
		PerCoinFailures:    []PerCoinFailure{},
		PipelineDescription: "池(多+空)→第一层(16项进入候选)→候选→第二层(至少6因子+可靠度≥0.67+入场信心≥31%,技策略)→待提交",
	}
	c.JSON(http.StatusOK, info)
}

// handleGetDirectionPool 获取方向池(多)/方向池(空) 列表
// 优先返回 kernel.RunPipeline 写入的实时方向池；无则返回空列表
func (s *Server) handleGetDirectionPool(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	if _, err := s.store.Trader().GetFullConfig(userID, traderID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "trader not found or no access"})
		return
	}
	st := kernel.GetPipelineState(traderID)
	if st != nil {
		resp := DirectionPoolResponse{
			Long:  make([]DirectionPoolItem, 0, len(st.DirectionLong)),
			Short: make([]DirectionPoolItem, 0, len(st.DirectionShort)),
		}
		for _, v := range st.DirectionLong {
			resp.Long = append(resp.Long, DirectionPoolItem{
				Symbol: v.Symbol, StrengthPct: v.StrengthPct, Score: v.Score,
				MarketCondition: v.MarketCondition, ReliabilityPct: v.ReliabilityPct,
				Timing: v.Timing, VolumePricePct: v.VolumePricePct, FromAI: v.FromAI,
			})
		}
		for _, v := range st.DirectionShort {
			resp.Short = append(resp.Short, DirectionPoolItem{
				Symbol: v.Symbol, StrengthPct: v.StrengthPct, Score: v.Score,
				MarketCondition: v.MarketCondition, ReliabilityPct: v.ReliabilityPct,
				Timing: v.Timing, VolumePricePct: v.VolumePricePct, FromAI: v.FromAI,
			})
		}
		c.JSON(http.StatusOK, resp)
		return
	}
	c.JSON(http.StatusOK, DirectionPoolResponse{Long: []DirectionPoolItem{}, Short: []DirectionPoolItem{}})
}

// handleGetLatestAnalysis 返回最新周期 AI 分析快照（market_regime、scenario、symbol_predictions、止盈止损调节+基线），供雷达页/看板「实时数据+AI预测」展示
// 若交易员在运行，优先从运行中实例取 position_sl_tp_adjustments 与 position_sl_tp_baselines，便于展示「原始→调整后」
func (s *Server) handleGetLatestAnalysis(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")
	if _, err := s.store.Trader().GetFullConfig(userID, traderID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "trader not found or no access"})
		return
	}
	resp := LatestAnalysisResponse{UpdatedAt: ""}
	records, err := s.store.Decision().GetLatestRecords(traderID, 1)
	if err == nil && len(records) > 0 {
		rec := records[0]
		snap := kernel.ParseAnalysisFromRawResponse(rec.RawResponse)
		// 返回 RFC3339（带时区偏移）；前端可稳定按本地时区渲染，避免“UTC 字样”与时区混用造成困惑
		resp.UpdatedAt = rec.Timestamp.Format(time.RFC3339)
		if snap != nil {
			resp.MarketRegime = snap.MarketRegime
			resp.Scenario = snap.Scenario
			resp.MarketSummary = snap.MarketSummary
			resp.RiskAlert = snap.RiskAlert
			resp.SymbolPredictions = make([]SymbolPredictionItem, 0, len(snap.SymbolPredictions))
			for _, p := range snap.SymbolPredictions {
				resp.SymbolPredictions = append(resp.SymbolPredictions, SymbolPredictionItem{
					Symbol:             p.Symbol,
					PredictedDirection: p.PredictedDirection,
					Confidence:         p.Confidence,
					SuggestExit:        p.SuggestExit,
					SuggestOpen:        p.SuggestOpen,
				})
			}
			resp.PositionSLTPAdjustments = snap.PositionSLTPAdjustments
		}
	}
	// 运行中交易员：用内存中的调节与基线覆盖，保证展示「原始→调整后」且与当前生效一致
	if at, err := s.traderManager.GetTrader(traderID); err == nil {
		status := at.GetStatus()
		if adj, ok := status["position_sl_tp_adjustments"].([]kernel.PositionSLTPAdjustment); ok && len(adj) > 0 {
			resp.PositionSLTPAdjustments = adj
		}
		if base, ok := status["position_sl_tp_baselines"].([]kernel.PositionSLTPBaseline); ok && len(base) > 0 {
			resp.PositionSLTPBaselines = base
		}
	}
	c.JSON(http.StatusOK, resp)
}
