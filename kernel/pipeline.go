// Package kernel: 多层过滤与方向池执行逻辑（挂单流程信息 + 方向池）
// 依据用户分享的截图与 docs/多层过滤与方向池策略-设计与部署.md 实现可运行版本
package kernel

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"nofx/logger"
	"nofx/market"
	"nofx/store"
	"nofx/provider/nofxos"
)

// PipelineOptions 可选参数，由调用方（如 auto_trader）传入，与多空雷达配置一致
type PipelineOptions struct {
	AllowLong  bool // 允许做多
	AllowShort bool // 允许做空
}

// PipelineState 挂单流程与方向池的当前状态，供多空雷达 API 使用
type PipelineState struct {
	UpdatedAt           time.Time
	ProcessStage        string   // e.g. "第一层16项全未过", "第二层通过", "待提交"
	PoolLongCount       int
	PoolShortCount      int
	AfterLayer1Count    int
	AfterLayer2Count    int
	ToSubmitCount       int
	FlowLabel           string   // e.g. "2 → 0 → 0 → 0"
	Layer1FailureStats  []Layer1FailureStat
	PerCoinFailures     []PerCoinFailure
	PipelineDescription string
	DirectionLong       []DirectionPoolItem
	DirectionShort      []DirectionPoolItem
	// 本周期通过各层后的 symbol 列表，用于替换 ctx.CandidateCoins
	ToSubmitSymbols []string
	// Layer3 OI 对齐：开多时需 OIAlignedLong[sym]，开空时需 OIAlignedShort[sym]（当 OIAlignedRequired 时使用）
	OIAlignedLong  map[string]bool `json:"oi_aligned_long,omitempty"`
	OIAlignedShort map[string]bool `json:"oi_aligned_short,omitempty"`
}

// Layer1FailureStat 第一层某项条件未通过的数量统计
type Layer1FailureStat struct {
	Condition string `json:"condition"`
	Count     int    `json:"count"`
}

// PerCoinFailure 某币种未通过第一层的原因列表
type PerCoinFailure struct {
	Symbol  string   `json:"symbol"`
	Reasons []string `json:"reasons"`
}

// DirectionPoolItem 方向池单项
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

// 第一层 16 项条件 ID 与中文描述（与截图一致）
var layer1ConditionNames = map[string]string{
	"entry_timing":            "入场时机非now/soon",
	"volume_ok":               "量能未达标",
	"oi_ok":                   "持仓量/OI未达标",
	"long_tf_aligned":         "长周期未对齐",
	"multi_period_aligned":    "多周期不足两周期一致",
	"flow_aligned":            "流向未同向",
	"price_ranking_aligned":    "涨跌榜未同向",
	"reliability_min":         "可靠度不足",
	"short_tf_aligned":         "短周期未对齐",
	"whale_direction_aligned":  "大户多空未同向",
	"market_direction_aligned": "全市场多空未同向",
	"trend_strength":          "趋势强度不足",
	"rsi_zone":                "RSI区间不符",
	"macd_signal":             "MACD信号不符",
	"volume_trend":            "量价趋势未同向",
	"oi_trend":                "OI趋势未同向",
	"funding_ok":              "资金费率不符",
}

// RunPipeline 执行三层过滤并生成方向池，更新 ctx.CandidateCoins 为「待提交」列表，并写入状态供 API 读取
// traderID 用于存储状态；若为空则不写入存储；opts 可选，用于允许做多/做空过滤
func RunPipeline(ctx *Context, config *store.StrategyConfig, traderID string, opts *PipelineOptions) (*PipelineState, error) {
	if config == nil || config.MultilayerFilter == nil || !config.MultilayerFilter.Enabled {
		return nil, nil
	}

	cfg := config.MultilayerFilter
	layer1 := cfg.Layer1
	layer2 := cfg.Layer2
	layer3 := cfg.Layer3

	// 默认配置
	if layer1 == nil {
		layer1 = defaultLayer1Config()
	}
	if layer2 == nil {
		layer2 = &store.Layer2Config{MinFactors: 6, ReliabilityThreshold: 0.67, EntryConfidenceThresholdPct: 31}
	}
	if layer3 == nil {
		layer3 = &store.Layer3Config{MaxSignalAgeMinutes: 5}
	}
	if opts == nil {
		opts = &PipelineOptions{AllowLong: true, AllowShort: true}
	}

	state := &PipelineState{
		UpdatedAt:           time.Now(),
		PipelineDescription: "池(多+空)→第一层(16项进入候选)→候选→第二层(至少6因子+可靠度≥0.67+入场信心≥31%,技策略)→待提交→第三层(OI+信号年龄<5分钟)",
	}

	// 池(多+空)：仅对已有行情数据的候选做过滤，避免无数据标的被误判为通过
	poolSymbols := make([]string, 0, len(ctx.CandidateCoins))
	for _, c := range ctx.CandidateCoins {
		if ctx.MarketDataMap != nil && ctx.MarketDataMap[c.Symbol] != nil {
			poolSymbols = append(poolSymbols, c.Symbol)
		}
	}
	// 方向池数量先按一半估算，后续 buildDirectionPools 会按多/空真实划分
	state.PoolLongCount = len(poolSymbols) / 2
	if state.PoolLongCount*2 < len(poolSymbols) {
		state.PoolShortCount = len(poolSymbols) - state.PoolLongCount
	} else {
		state.PoolShortCount = state.PoolLongCount
	}
	if state.PoolLongCount+state.PoolShortCount != len(poolSymbols) {
		state.PoolLongCount = len(poolSymbols)
		state.PoolShortCount = 0
	}

	// 第一层：逐币种检查；MinItemsToPass>0 时至少通过 N 项即过（放宽）
	enabledCount := 0
	for _, item := range layer1.Items {
		if item.Enabled {
			enabledCount++
		}
	}
	minPass := layer1.MinItemsToPass
	if minPass < 0 {
		minPass = 0
	}

	failCount := make(map[string]int)
	var afterLayer1 []string
	perCoinReasons := make(map[string][]string)

	for _, sym := range poolSymbols {
		reasons := runLayer1ForSymbol(sym, ctx, layer1, layer2)
		passed := len(reasons) == 0
		if !passed && minPass > 0 && enabledCount > 0 {
			passed = (enabledCount - len(reasons)) >= minPass
		}
		if passed {
			afterLayer1 = append(afterLayer1, sym)
		} else {
			for _, r := range reasons {
				failCount[r]++
			}
			perCoinReasons[sym] = reasons
		}
	}

	for cond, cnt := range failCount {
		name := layer1ConditionNames[cond]
		if name == "" {
			name = cond
		}
		state.Layer1FailureStats = append(state.Layer1FailureStats, Layer1FailureStat{Condition: name, Count: cnt})
	}
	for sym, reasons := range perCoinReasons {
		names := make([]string, 0, len(reasons))
		for _, r := range reasons {
			n := layer1ConditionNames[r]
			if n == "" {
				n = r
			}
			names = append(names, n)
		}
		state.PerCoinFailures = append(state.PerCoinFailures, PerCoinFailure{Symbol: sym, Reasons: names})
	}
	state.AfterLayer1Count = len(afterLayer1)

	// 第二层：至少 M 因子 + 可靠度 + 入场信心（无 AI 时用规则估算）
	var afterLayer2 []string
	for _, sym := range afterLayer1 {
		factorsPassed, reliability, entryConfPct := evaluateLayer2(sym, ctx, layer2)
		if factorsPassed >= layer2.MinFactors && reliability >= layer2.ReliabilityThreshold && entryConfPct >= layer2.EntryConfidenceThresholdPct {
			afterLayer2 = append(afterLayer2, sym)
		}
	}
	state.AfterLayer2Count = len(afterLayer2)

	// 方向池(多)/(空)：按截图逻辑「趋势、动能、多周期至少两周期一致」划分；traderID 用于上周期强度缓存（动能同向加强加分）
	state.DirectionLong, state.DirectionShort = buildDirectionPools(ctx, afterLayer1, config, traderID)
	if cfg.DirectionPool != nil && cfg.DirectionPool.EnableStrengthSmoothing && traderID != "" {
		applyStrengthSmoothing(traderID, state.DirectionLong, state.DirectionShort, cfg.DirectionPool.StrengthSmoothingWeight)
	}
	if traderID != "" {
		saveLastCycleStrength(traderID, state.DirectionLong, state.DirectionShort)
	}
	state.PoolLongCount = len(state.DirectionLong)
	state.PoolShortCount = len(state.DirectionShort)

	// 第三层预过滤：仅做数量；真实「OI 对齐 + 信号年龄<5分钟」在 auto_trader 开单前校验
	toSubmit := afterLayer2
	if layer3.MaxSignalAgeMinutes > 0 {
		_ = layer3.MaxSignalAgeMinutes
	}
	// 按多空雷达「允许做多/做空」过滤待提交：只保留允许方向上的标的
	toSubmit = filterToSubmitByDirection(toSubmit, state.DirectionLong, state.DirectionShort, opts)
	if cfg.DirectionPool != nil && cfg.DirectionPool.SortByStrength {
		toSubmit = sortToSubmitByStrength(toSubmit, state.DirectionLong, state.DirectionShort)
	}
	state.ToSubmitCount = len(toSubmit)
	state.ToSubmitSymbols = toSubmit

	// Layer3：为待提交与方向池标的计算 OI 对齐（开多：在 OI 增榜或不在减榜；开空：在 OI 减榜或不在增榜）
	state.OIAlignedLong = make(map[string]bool)
	state.OIAlignedShort = make(map[string]bool)
	for _, sym := range toSubmit {
		n := normalizeSymbol(sym)
		state.OIAlignedLong[n], state.OIAlignedShort[n] = computeOIAligned(sym, ctx)
	}
	for _, i := range state.DirectionLong {
		n := normalizeSymbol(i.Symbol)
		if _, ok := state.OIAlignedLong[n]; !ok {
			state.OIAlignedLong[n], state.OIAlignedShort[n] = computeOIAligned(i.Symbol, ctx)
		}
	}
	for _, i := range state.DirectionShort {
		n := normalizeSymbol(i.Symbol)
		if _, ok := state.OIAlignedShort[n]; !ok {
			state.OIAlignedLong[n], state.OIAlignedShort[n] = computeOIAligned(i.Symbol, ctx)
		}
	}

	// 流程阶段文案
	if state.AfterLayer1Count == 0 {
		state.ProcessStage = "第一层16项全未过"
	} else if state.AfterLayer2Count == 0 {
		state.ProcessStage = "第二层未过"
	} else if state.ToSubmitCount == 0 {
		state.ProcessStage = "第三层未过或方向未允许"
	} else {
		state.ProcessStage = "待提交"
	}
	state.FlowLabel = formatFlowLabel(state.PoolLongCount+state.PoolShortCount, state.AfterLayer1Count, state.AfterLayer2Count, state.ToSubmitCount)

	if traderID != "" {
		SetPipelineState(traderID, state)
	}
	logger.Infof("📊 Pipeline: pool=%d L1=%d L2=%d toSubmit=%d [%s]",
		len(poolSymbols), state.AfterLayer1Count, state.AfterLayer2Count, state.ToSubmitCount, state.FlowLabel)
	return state, nil
}

func defaultLayer1Config() *store.Layer1Config {
	return &store.Layer1Config{
		RequiredAll:       true,
		MinPeriodsAligned: 2, // 三周期(4h/1h/短)至少两周期一致
		Items: []store.Layer1Item{
			{ID: "entry_timing", Enabled: true, Allowed: []string{"now", "soon"}},
			{ID: "volume_ok", Enabled: true},
			{ID: "oi_ok", Enabled: true},
			{ID: "long_tf_aligned", Enabled: true},
			{ID: "multi_period_aligned", Enabled: true},
			{ID: "flow_aligned", Enabled: true},
			{ID: "price_ranking_aligned", Enabled: true},
			{ID: "reliability_min", Enabled: true, Value: 0.4},
			{ID: "short_tf_aligned", Enabled: true},
			{ID: "whale_direction_aligned", Enabled: true},
			{ID: "market_direction_aligned", Enabled: false},
			{ID: "trend_strength", Enabled: true},
			{ID: "rsi_zone", Enabled: true},
			{ID: "macd_signal", Enabled: true},
			{ID: "volume_trend", Enabled: false},
			{ID: "oi_trend", Enabled: false},
			{ID: "funding_ok", Enabled: true},
		},
		MaxSignalAgeMinutes: 5,
	}
}

// runLayer1ForSymbol 返回该币种未通过的第一层条件 ID 列表
func runLayer1ForSymbol(symbol string, ctx *Context, cfg *store.Layer1Config, layer2 *store.Layer2Config) []string {
	if cfg == nil || len(cfg.Items) == 0 {
		return nil
	}
	var failed []string
	md := ctx.MarketDataMap[symbol]
	for _, item := range cfg.Items {
		if !item.Enabled {
			continue
		}
		ok := checkLayer1Item(symbol, item, md, ctx, cfg, layer2)
		if !ok {
			failed = append(failed, item.ID)
		}
	}
	if cfg.RequiredAll && len(failed) > 0 {
		return failed
	}
	return failed
}

// checkLayer1Item 单条件检查；按截图设计：长/短周期对齐、流向、涨跌榜、大户/全市场多空等有数据则严格判断
func checkLayer1Item(symbol string, item store.Layer1Item, md *market.Data, ctx *Context, layer1 *store.Layer1Config, layer2 *store.Layer2Config) bool {
	failClosed := layer1 != nil && layer1.FailClosedWhenDataMissing
	switch item.ID {
	case "entry_timing":
		// 入场时机 now/soon：无信号时间戳时通过，第三层开单前再校验信号年龄<5分钟
		return true
	case "volume_ok":
		if md == nil || md.IntradaySeries == nil {
			return true
		}
		vol := md.IntradaySeries.Volume
		if len(vol) < 2 {
			return true
		}
		avg := average(vol[:len(vol)-1])
		if avg <= 0 {
			return true
		}
		return vol[len(vol)-1] >= avg*0.5
	case "oi_ok":
		if md == nil || md.OpenInterest == nil {
			return true
		}
		return md.OpenInterest.Latest > 0
	case "long_tf_aligned":
		// 长周期方向对齐：4h 与 1h 同向（同涨或同跌）
		if md == nil {
			return true
		}
		return sameSign(md.PriceChange4h, md.PriceChange1h)
	case "multi_period_aligned":
		// 多周期至少 N 周期一致：4h、1h、短周期(价格 vs EMA20) 至少 minPeriods 个同向（经验：三周期至少两周期一致）
		if md == nil {
			return !failClosed
		}
		minPeriods := 2
		if layer1 != nil && layer1.MinPeriodsAligned > 0 {
			minPeriods = layer1.MinPeriodsAligned
			if minPeriods > 3 {
				minPeriods = 3
			}
		}
		up4h := md.PriceChange4h >= 0
		up1h := md.PriceChange1h >= 0
		upShort := md.CurrentPrice > 0 && md.CurrentEMA20 > 0 && md.CurrentPrice >= md.CurrentEMA20
		upCount := 0
		if up4h {
			upCount++
		}
		if up1h {
			upCount++
		}
		if upShort {
			upCount++
		}
		downCount := 3 - upCount
		return upCount >= minPeriods || downCount >= minPeriods
	case "short_tf_aligned":
		// 短周期方向对齐：1h 与近期趋势(价格 vs EMA20) 同向
		if md == nil {
			return true
		}
		if md.CurrentPrice <= 0 || md.CurrentEMA20 <= 0 {
			return true
		}
		up := md.CurrentPrice >= md.CurrentEMA20
		up1h := md.PriceChange1h >= 0
		return up == up1h
	case "flow_aligned":
		// 流向与价格同向：机构净流入+涨 或 净流出+跌
		if md == nil {
			return !failClosed
		}
		if ctx.NetFlowRankingData == nil {
			return !failClosed
		}
		inTop := symbolInNetFlowList(symbol, ctx.NetFlowRankingData.InstitutionFutureTop)
		inLow := symbolInNetFlowList(symbol, ctx.NetFlowRankingData.InstitutionFutureLow)
		if !inTop && !inLow {
			return true
		}
		priceUp := md.PriceChange1h >= 0
		return (inTop && priceUp) || (inLow && !priceUp)
	case "price_ranking_aligned":
		// 涨跌榜与方向一致：在涨幅榜且涨 或 在跌幅榜且跌
		if md == nil {
			return !failClosed
		}
		if ctx.PriceRankingData == nil || len(ctx.PriceRankingData.Durations) == 0 {
			return !failClosed
		}
		inTop, inLow := symbolInPriceRanking(symbol, ctx.PriceRankingData)
		if !inTop && !inLow {
			return true
		}
		priceUp := md.PriceChange1h >= 0
		return (inTop && priceUp) || (inLow && !priceUp)
	case "reliability_min":
		// 可靠度门槛：若配置了 Value，用 Layer2 可靠度预算，低于则不通过
		if item.Value <= 0 {
			return true
		}
		if layer2 == nil {
			return true
		}
		_, reliability, _ := evaluateLayer2(symbol, ctx, layer2)
		return reliability >= item.Value
	case "whale_direction_aligned":
		// 大户 OI 与价格同向：OI 增+涨 或 OI 减+跌
		if md == nil {
			return !failClosed
		}
		if ctx.OIRankingData == nil {
			return !failClosed
		}
		inTop := symbolInOIPositions(symbol, ctx.OIRankingData.TopPositions)
		inLow := symbolInOIPositions(symbol, ctx.OIRankingData.LowPositions)
		if !inTop && !inLow {
			return true
		}
		priceUp := md.PriceChange1h >= 0
		return (inTop && priceUp) || (inLow && !priceUp)
	case "market_direction_aligned":
		// 全市场多空与方向一致：用涨跌榜代表市场方向，与 price_ranking_aligned 同逻辑
		if md == nil {
			return !failClosed
		}
		if ctx.PriceRankingData == nil || len(ctx.PriceRankingData.Durations) == 0 {
			return !failClosed
		}
		inTop, inLow := symbolInPriceRanking(symbol, ctx.PriceRankingData)
		if !inTop && !inLow {
			return true
		}
		priceUp := md.PriceChange1h >= 0
		return (inTop && priceUp) || (inLow && !priceUp)
	case "trend_strength":
		// 趋势强度：4h 涨跌幅有一定幅度（避免震荡）
		if md == nil {
			return true
		}
		abs4h := absPct(md.PriceChange4h)
		return abs4h >= 0.2 || (md.CurrentPrice > 0 && md.CurrentEMA20 > 0)
	case "rsi_zone":
		// RSI 不在极端区：可交易区间 30~70
		if md == nil {
			return true
		}
		rsi := md.CurrentRSI7
		if rsi < 0 {
			return true
		}
		return rsi >= 28 && rsi <= 72
	case "macd_signal":
		// MACD 与价格趋势同向
		if md == nil {
			return true
		}
		if md.CurrentMACD == 0 {
			return true
		}
		priceUp := md.PriceChange1h >= 0
		macdUp := md.CurrentMACD > 0
		return priceUp == macdUp
	case "volume_trend":
		// 量价同向：近期量能不低于均值
		if md == nil || md.IntradaySeries == nil || len(md.IntradaySeries.Volume) < 2 {
			return true
		}
		vol := md.IntradaySeries.Volume
		avg := average(vol[:len(vol)-1])
		return avg <= 0 || vol[len(vol)-1] >= avg*0.5
	case "oi_trend":
		// OI 有参考：Latest 与 Average 可比较
		if md == nil || md.OpenInterest == nil {
			return true
		}
		return md.OpenInterest.Latest > 0
	case "funding_ok":
		// 资金费率不过于极端（|费率| < 0.01）；启用 Coinglass 时多所费率也需在合理区间（补强）
		if md == nil {
			return true
		}
		fr := md.FundingRate
		if fr < -0.01 || fr > 0.01 {
			return false
		}
		if ctx.CoinglassFundingMap != nil {
			cgSym := symbolToCoinglass(symbol)
			if cgSym != "" {
				if r, ok := ctx.CoinglassFundingMap[cgSym]; ok && (r < -0.01 || r > 0.01) {
					return false
				}
			}
		}
		return true
	default:
		return true
	}
}

func sameSign(a, b float64) bool {
	return (a >= 0 && b >= 0) || (a < 0 && b < 0)
}

func absPct(pct float64) float64 {
	if pct < 0 {
		return -pct
	}
	return pct
}

func symbolInNetFlowList(symbol string, list []nofxos.NetFlowPosition) bool {
	norm := normalizeSymbol(symbol)
	for _, p := range list {
		if normalizeSymbol(p.Symbol) == norm {
			return true
		}
	}
	return false
}

func symbolInOIPositions(symbol string, list []nofxos.OIPosition) bool {
	norm := normalizeSymbol(symbol)
	for _, p := range list {
		if normalizeSymbol(p.Symbol) == norm {
			return true
		}
	}
	return false
}

func symbolInPriceRanking(symbol string, data *nofxos.PriceRankingData) (inTop, inLow bool) {
	if data == nil {
		return false, false
	}
	norm := normalizeSymbol(symbol)
	for _, d := range data.Durations {
		if d == nil {
			continue
		}
		for _, t := range d.Top {
			if normalizeSymbol(t.Symbol) == norm {
				inTop = true
				break
			}
		}
		for _, l := range d.Low {
			if normalizeSymbol(l.Symbol) == norm {
				inLow = true
				break
			}
		}
	}
	return inTop, inLow
}

func normalizeSymbol(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToUpper(s)
	return s
}

// symbolToCoinglass 将交易对转为 Coinglass 币种符号（BTC/ETH）；仅 BTC/ETH 有 Coinglass 多所数据时使用
func symbolToCoinglass(symbol string) string {
	n := normalizeSymbol(symbol)
	if strings.HasSuffix(n, "USDT") {
		base := n[:len(n)-4]
		if base == "BTC" || base == "ETH" {
			return base
		}
	}
	if n == "BTC" || n == "ETH" {
		return n
	}
	return ""
}

// computeOIAligned 返回 (做多时 OI 是否对齐, 做空时 OI 是否对齐)。无 OI 排行数据时均视为对齐。
func computeOIAligned(symbol string, ctx *Context) (alignedLong, alignedShort bool) {
	if ctx.OIRankingData == nil {
		return true, true
	}
	inTop := symbolInOIPositions(symbol, ctx.OIRankingData.TopPositions)
	inLow := symbolInOIPositions(symbol, ctx.OIRankingData.LowPositions)
	// 做多对齐：OI 增（在增榜）或至少不在减榜
	alignedLong = inTop || !inLow
	// 做空对齐：OI 减（在减榜）或至少不在增榜
	alignedShort = inLow || !inTop
	return alignedLong, alignedShort
}

func average(x []float64) float64 {
	if len(x) == 0 {
		return 0
	}
	s := 0.0
	for _, v := range x {
		s += v
	}
	return s / float64(len(x))
}

// evaluateLayer2 返回 (通过因子数, 可靠度, 入场信心%)；纳入排行与补强数据
func evaluateLayer2(symbol string, ctx *Context, cfg *store.Layer2Config) (factorsPassed int, reliability, entryConfPct float64) {
	md := ctx.MarketDataMap[symbol]
	if md == nil {
		return 0, 0, 0
	}
	priceUp := md.PriceChange1h >= 0
	// 因子：价格趋势、量能、RSI 合理、MACD、OI、多周期同向
	if md.CurrentPrice > 0 && md.CurrentEMA20 > 0 {
		factorsPassed++
	}
	if md.IntradaySeries != nil && len(md.IntradaySeries.Volume) > 0 {
		factorsPassed++
	}
	if md.CurrentRSI7 >= 28 && md.CurrentRSI7 <= 72 {
		factorsPassed++
	}
	if md.CurrentMACD != 0 {
		factorsPassed++
	}
	if md.OpenInterest != nil && md.OpenInterest.Latest > 0 {
		factorsPassed++
	}
	if sameSign(md.PriceChange1h, md.PriceChange4h) {
		factorsPassed++
	}
	if md.PriceChange1h != 0 || md.PriceChange4h != 0 {
		factorsPassed++
	}
	// 排行与补强因子
	if ctx.OIRankingData != nil {
		inTop := symbolInOIPositions(symbol, ctx.OIRankingData.TopPositions)
		inLow := symbolInOIPositions(symbol, ctx.OIRankingData.LowPositions)
		if (inTop && priceUp) || (inLow && !priceUp) {
			factorsPassed++
		}
	}
	if ctx.NetFlowRankingData != nil {
		inTop := symbolInNetFlowList(symbol, ctx.NetFlowRankingData.InstitutionFutureTop)
		inLow := symbolInNetFlowList(symbol, ctx.NetFlowRankingData.InstitutionFutureLow)
		if (inTop && priceUp) || (inLow && !priceUp) {
			factorsPassed++
		}
	}
	if ctx.PriceRankingData != nil && len(ctx.PriceRankingData.Durations) > 0 {
		inTop, inLow := symbolInPriceRanking(symbol, ctx.PriceRankingData)
		if (inTop && priceUp) || (inLow && !priceUp) {
			factorsPassed++
		}
	}
	if ctx.BinanceFundingRateAvg8h != nil {
		if avg, ok := ctx.BinanceFundingRateAvg8h[symbol]; ok && avg >= -0.01 && avg <= 0.01 {
			factorsPassed++
		}
	}
	// 可靠度：多周期一致 + 因子数 + 排行对齐加成
	aligned := 0
	if sameSign(md.PriceChange4h, md.PriceChange1h) {
		aligned++
	}
	if md.CurrentPrice > 0 && md.CurrentEMA20 > 0 && (md.CurrentPrice >= md.CurrentEMA20) == (md.PriceChange1h >= 0) {
		aligned++
	}
	reliability = 0.5 + float64(aligned)*0.15 + float64(factorsPassed)*0.04
	if reliability > 1 {
		reliability = 1
	}
	// 入场信心：RSI、MACD、排行对齐
	entryConfPct = 35
	if md.CurrentRSI7 >= 30 && md.CurrentRSI7 <= 70 {
		entryConfPct += 10
	}
	if md.CurrentMACD != 0 && sameSign(md.PriceChange1h, md.CurrentMACD) {
		entryConfPct += 10
	}
	if ctx.OIRankingData != nil {
		inTop := symbolInOIPositions(symbol, ctx.OIRankingData.TopPositions)
		inLow := symbolInOIPositions(symbol, ctx.OIRankingData.LowPositions)
		if (inTop && priceUp) || (inLow && !priceUp) {
			entryConfPct += 5
		}
	}
	if ctx.PriceRankingData != nil {
		inTop, inLow := symbolInPriceRanking(symbol, ctx.PriceRankingData)
		if (inTop && priceUp) || (inLow && !priceUp) {
			entryConfPct += 5
		}
	}
	if entryConfPct > 100 {
		entryConfPct = 100
	}
	return factorsPassed, reliability, entryConfPct
}

func formatFlowLabel(pool, l1, l2, submit int) string {
	return fmt.Sprintf("%d → %d → %d → %d", pool, l1, l2, submit)
}

// sortToSubmitByStrength 按方向池中的 StrengthPct 降序排列 toSubmit，便于优先开高强度标的
func sortToSubmitByStrength(toSubmit []string, directionLong, directionShort []DirectionPoolItem) []string {
	score := make(map[string]float64)
	for _, i := range directionLong {
		n := normalizeSymbol(i.Symbol)
		if i.StrengthPct > score[n] {
			score[n] = i.StrengthPct
		}
	}
	for _, i := range directionShort {
		n := normalizeSymbol(i.Symbol)
		if i.StrengthPct > score[n] {
			score[n] = i.StrengthPct
		}
	}
	sort.Slice(toSubmit, func(i, j int) bool {
		si := score[normalizeSymbol(toSubmit[i])]
		sj := score[normalizeSymbol(toSubmit[j])]
		return si > sj
	})
	return toSubmit
}

// filterToSubmitByDirection 按允许做多/做空过滤待提交列表，只保留在允许方向池中的标的
func filterToSubmitByDirection(toSubmit []string, directionLong, directionShort []DirectionPoolItem, opts *PipelineOptions) []string {
	if opts == nil || (opts.AllowLong && opts.AllowShort) {
		return toSubmit
	}
	longSet := make(map[string]bool)
	for _, i := range directionLong {
		longSet[normalizeSymbol(i.Symbol)] = true
	}
	shortSet := make(map[string]bool)
	for _, i := range directionShort {
		shortSet[normalizeSymbol(i.Symbol)] = true
	}
	var out []string
	for _, sym := range toSubmit {
		n := normalizeSymbol(sym)
		if (opts.AllowLong && longSet[n]) || (opts.AllowShort && shortSet[n]) {
			out = append(out, sym)
		}
	}
	return out
}

// applyDirectionBonus 用 Binance 多空、OI/净流入/涨跌榜、强平聚合对方向池单项做强度与可靠度加成；dp 为 nil 时不设上限
func applyDirectionBonus(ctx *Context, symbol string, isLong bool, item *DirectionPoolItem, dp *store.DirectionPoolConfig) {
	strengthBonus := 0.0
	reliBonus := 0.0
	if ctx.BinanceLongShortMap != nil {
		if ls, ok := ctx.BinanceLongShortMap[symbol]; ok {
			if isLong && ls.LongAccount > 0.5 {
				strengthBonus += 5
				reliBonus += 3
			}
			if !isLong && ls.ShortAccount > 0.5 {
				strengthBonus += 5
				reliBonus += 3
			}
		}
	}
	if ctx.OIRankingData != nil {
		inTop := symbolInOIPositions(symbol, ctx.OIRankingData.TopPositions)
		inLow := symbolInOIPositions(symbol, ctx.OIRankingData.LowPositions)
		if (isLong && inTop) || (!isLong && inLow) {
			strengthBonus += 5
			reliBonus += 3
		}
	}
	if ctx.NetFlowRankingData != nil {
		inTop := symbolInNetFlowList(symbol, ctx.NetFlowRankingData.InstitutionFutureTop)
		inLow := symbolInNetFlowList(symbol, ctx.NetFlowRankingData.InstitutionFutureLow)
		if (isLong && inTop) || (!isLong && inLow) {
			strengthBonus += 5
			reliBonus += 3
		}
	}
	if ctx.PriceRankingData != nil && len(ctx.PriceRankingData.Durations) > 0 {
		inTop, inLow := symbolInPriceRanking(symbol, ctx.PriceRankingData)
		if (isLong && inTop) || (!isLong && inLow) {
			strengthBonus += 5
			reliBonus += 3
		}
	}
	if ctx.LiquidationAgg != nil {
		// 近期空头强平多→偏多信号，多头强平多→偏空信号；仅做小幅加成
		longLiq := ctx.LiquidationAgg.Long1hUSD
		shortLiq := ctx.LiquidationAgg.Short1hUSD
		if longLiq+shortLiq > 0 {
			shortRatio := shortLiq / (longLiq + shortLiq)
			if isLong && shortRatio > 0.55 {
				strengthBonus += 2
			}
			if !isLong && shortRatio < 0.45 {
				strengthBonus += 2
			}
		}
	}
	// 资金费率 8h 均值：|费率| 在合理区间时小幅加成，极端时不加分
	if ctx.BinanceFundingRateAvg8h != nil {
		if avg, ok := ctx.BinanceFundingRateAvg8h[symbol]; ok && avg >= -0.01 && avg <= 0.01 {
			strengthBonus += 2
			reliBonus += 1
		}
	}
	// Basis（永续-现货价差）：与方向一致时小幅加成（正 basis 偏多、负偏空）
	if ctx.BasisMap != nil {
		if basis, ok := ctx.BasisMap[symbol]; ok {
			if isLong && basis > 0 || !isLong && basis < 0 {
				strengthBonus += 2
				reliBonus += 1
			}
		}
	}
	// Coinglass CGDI（多空扩散）：与方向一致时小幅加成；鲸鱼指数同向时略加
	if ctx.CoinglassCGDI != 0 {
		if isLong && ctx.CoinglassCGDI > 0.1 || !isLong && ctx.CoinglassCGDI < -0.1 {
			strengthBonus += 2
			reliBonus += 1
		}
	}
	if ctx.CoinglassWhaleIndex != 0 {
		if isLong && ctx.CoinglassWhaleIndex > 0 || !isLong && ctx.CoinglassWhaleIndex < 0 {
			strengthBonus += 1
		}
	}
	if dp != nil {
		if dp.StrengthBonusCap > 0 && strengthBonus > dp.StrengthBonusCap {
			strengthBonus = dp.StrengthBonusCap
		}
		if dp.ReliBonusCap > 0 && reliBonus > dp.ReliBonusCap {
			reliBonus = dp.ReliBonusCap
		}
	}
	item.StrengthPct += strengthBonus
	if item.StrengthPct > 100 {
		item.StrengthPct = 100
	}
	item.ReliabilityPct += reliBonus
	if item.ReliabilityPct > 100 {
		item.ReliabilityPct = 100
	}
	item.Score = item.StrengthPct / 100
}

// volumePricePctFromMD 用主周期量能计算 VolumePricePct：近期量/均量×10，上限 100；无数据时返回 10
func volumePricePctFromMD(md *market.Data) float64 {
	if md == nil || md.IntradaySeries == nil || len(md.IntradaySeries.Volume) < 2 {
		return 10
	}
	vol := md.IntradaySeries.Volume
	avg := average(vol[:len(vol)-1])
	if avg <= 0 {
		return 10
	}
	ratio := vol[len(vol)-1] / avg
	pct := ratio * 10
	if pct > 100 {
		pct = 100
	}
	if pct < 0 {
		pct = 0
	}
	return pct
}

// buildOneDirectionItem 为指定标的与方向（多/空）计算方向池单项；prevStrength 为上周期该侧强度，用于动能「同向加强」加分
func buildOneDirectionItem(sym string, md *market.Data, ctx *Context, isLong bool, dp *store.DirectionPoolConfig, prevStrength float64) DirectionPoolItem {
	item := DirectionPoolItem{
		Symbol:          sym,
		StrengthPct:     80,
		Score:           0.8,
		MarketCondition: "trend",
		ReliabilityPct:  50,
		Timing:          "now",
		VolumePricePct:  10,
	}
	if md == nil {
		return item
	}
	item.VolumePricePct = volumePricePctFromMD(md)
	aligned4h1h := sameSign(md.PriceChange4h, md.PriceChange1h)
	shortTrendUp := md.CurrentPrice > 0 && md.CurrentEMA20 > 0 && md.CurrentPrice >= md.CurrentEMA20
	shortTrendDown := md.CurrentPrice > 0 && md.CurrentEMA20 > 0 && md.CurrentPrice < md.CurrentEMA20
	alignedCount := 0
	if aligned4h1h {
		alignedCount++
	}
	if md.PriceChange1h >= 0 && shortTrendUp || md.PriceChange1h < 0 && shortTrendDown {
		alignedCount++
	}
	momentumOk := md.CurrentMACD == 0 || sameSign(md.PriceChange1h, md.CurrentMACD)
	if alignedCount >= 1 && momentumOk {
		item.StrengthPct = 70 + float64(alignedCount)*15
		if item.StrengthPct > 100 {
			item.StrengthPct = 100
		}
		item.Score = item.StrengthPct / 100
		item.ReliabilityPct = 40 + float64(alignedCount)*15
		if md.CurrentRSI7 >= 30 && md.CurrentRSI7 <= 70 {
			item.ReliabilityPct += 10
		}
		switch alignedCount {
		case 2:
			item.MarketCondition = "strong_trend"
		case 1:
			item.MarketCondition = "trend"
		default:
			item.MarketCondition = "weak_trend"
		}
		item.Timing = "now"
	} else {
		item.StrengthPct = 50
		item.Score = 0.5
		item.ReliabilityPct = 35
		item.MarketCondition = "weak_trend"
		item.Timing = "soon"
	}
	// 动能同向加强：本周期强度高于上周期时加分（上限 +5%，总强度不超 100）
	if prevStrength > 0 && item.StrengthPct > prevStrength && dp != nil && dp.StrengthBonusCap > 0 {
		bonus := 5.0
		if bonus > dp.StrengthBonusCap {
			bonus = dp.StrengthBonusCap
		}
		item.StrengthPct += bonus
		if item.StrengthPct > 100 {
			item.StrengthPct = 100
		}
		item.Score = item.StrengthPct / 100
	}
	applyDirectionBonus(ctx, sym, isLong, &item, dp)
	return item
}

// buildDirectionPools 按「趋势、动能、多周期一致」划分多/空方向池；traderID 用于读取上周期强度做动能同向加强加分
func buildDirectionPools(ctx *Context, symbols []string, config *store.StrategyConfig, traderID string) (long, short []DirectionPoolItem) {
	var dp *store.DirectionPoolConfig
	layer2 := (*store.Layer2Config)(nil)
	if config != nil && config.MultilayerFilter != nil {
		dp = config.MultilayerFilter.DirectionPool
		layer2 = config.MultilayerFilter.Layer2
	}
	if layer2 == nil {
		layer2 = &store.Layer2Config{MinFactors: 6, ReliabilityThreshold: 0.67, EntryConfidenceThresholdPct: 31}
	}
	minStrength := 0.0
	singleSideOnly := false
	sortByStrength := false
	filterPoolByLayer2 := false
	if dp != nil {
		minStrength = dp.MinStrengthPct
		singleSideOnly = dp.SingleSideOnly
		sortByStrength = dp.SortByStrength
		filterPoolByLayer2 = dp.FilterPoolByLayer2
	}
	prevStrength := loadLastCycleStrength(traderID)
	getPrev := func(sym string, isLong bool) float64 {
		if prevStrength == nil {
			return 0
		}
		k := normalizeSymbol(sym) + "_long"
		if !isLong {
			k = normalizeSymbol(sym) + "_short"
		}
		return prevStrength[k]
	}
	for _, sym := range symbols {
		if filterPoolByLayer2 {
			factorsPassed, reliability, entryConfPct := evaluateLayer2(sym, ctx, layer2)
			if factorsPassed < layer2.MinFactors || reliability < layer2.ReliabilityThreshold || entryConfPct < layer2.EntryConfidenceThresholdPct {
				continue
			}
		}
		md := ctx.MarketDataMap[sym]
		if md == nil {
			item := DirectionPoolItem{Symbol: sym, StrengthPct: 80, Score: 0.8, MarketCondition: "trend", ReliabilityPct: 50, Timing: "now", VolumePricePct: 10}
			if minStrength <= 0 || item.StrengthPct >= minStrength {
				long = append(long, item)
			}
			continue
		}
		if singleSideOnly {
			itemLong := buildOneDirectionItem(sym, md, ctx, true, dp, getPrev(sym, true))
			itemShort := buildOneDirectionItem(sym, md, ctx, false, dp, getPrev(sym, false))
			if itemLong.StrengthPct >= itemShort.StrengthPct {
				if minStrength <= 0 || itemLong.StrengthPct >= minStrength {
					long = append(long, itemLong)
				}
			} else {
				if minStrength <= 0 || itemShort.StrengthPct >= minStrength {
					short = append(short, itemShort)
				}
			}
			continue
		}
		isLong := md.PriceChange1h >= 0
		item := buildOneDirectionItem(sym, md, ctx, isLong, dp, getPrev(sym, isLong))
		if minStrength > 0 && item.StrengthPct < minStrength {
			continue
		}
		if isLong {
			long = append(long, item)
		} else {
			short = append(short, item)
		}
	}
	if sortByStrength {
		sort.Slice(long, func(i, j int) bool { return long[i].StrengthPct > long[j].StrengthPct })
		sort.Slice(short, func(i, j int) bool { return short[i].StrengthPct > short[j].StrengthPct })
	}
	return long, short
}

// pipelineStateStore 按 traderID 存储最近一次 PipelineState
var (
	pipelineStateMu sync.RWMutex
	pipelineState   = make(map[string]*PipelineState)
)

// directionStrengthCache 用于强度平滑：traderID -> symbol -> 上一周期 StrengthPct
var (
	directionStrengthCacheMu sync.Mutex
	directionStrengthCache   = make(map[string]map[string]float64)
)

// lastCycleStrengthCache 上周期方向池强度，用于动能「同向加强」加分：traderID -> (symbol_normalized+"_long"|"_short") -> StrengthPct
var (
	lastCycleStrengthCacheMu sync.Mutex
	lastCycleStrengthCache   = make(map[string]map[string]float64)
)

func loadLastCycleStrength(traderID string) map[string]float64 {
	lastCycleStrengthCacheMu.Lock()
	defer lastCycleStrengthCacheMu.Unlock()
	if m := lastCycleStrengthCache[traderID]; m != nil {
		out := make(map[string]float64, len(m))
		for k, v := range m {
			out[k] = v
		}
		return out
	}
	return nil
}

func saveLastCycleStrength(traderID string, long, short []DirectionPoolItem) {
	lastCycleStrengthCacheMu.Lock()
	defer lastCycleStrengthCacheMu.Unlock()
	if lastCycleStrengthCache[traderID] == nil {
		lastCycleStrengthCache[traderID] = make(map[string]float64)
	}
	m := lastCycleStrengthCache[traderID]
	for k := range m {
		delete(m, k)
	}
	for _, i := range long {
		k := normalizeSymbol(i.Symbol) + "_long"
		if i.StrengthPct > m[k] {
			m[k] = i.StrengthPct
		}
	}
	for _, i := range short {
		k := normalizeSymbol(i.Symbol) + "_short"
		if i.StrengthPct > m[k] {
			m[k] = i.StrengthPct
		}
	}
}

// applyStrengthSmoothing 对方向池单项的 StrengthPct 做指数平滑，减少周期间抖动
func applyStrengthSmoothing(traderID string, long, short []DirectionPoolItem, weight float64) {
	if weight <= 0 || weight >= 1 {
		weight = 0.7
	}
	directionStrengthCacheMu.Lock()
	defer directionStrengthCacheMu.Unlock()
	if directionStrengthCache[traderID] == nil {
		directionStrengthCache[traderID] = make(map[string]float64)
	}
	cache := directionStrengthCache[traderID]
	for i := range long {
		sym := normalizeSymbol(long[i].Symbol)
		cur := long[i].StrengthPct
		last := cache[sym]
		smoothed := weight*last + (1-weight)*cur
		if last == 0 {
			smoothed = cur
		}
		long[i].StrengthPct = smoothed
		long[i].Score = smoothed / 100
		cache[sym] = smoothed
	}
	for i := range short {
		sym := normalizeSymbol(short[i].Symbol)
		cur := short[i].StrengthPct
		last := cache[sym]
		smoothed := weight*last + (1-weight)*cur
		if last == 0 {
			smoothed = cur
		}
		short[i].StrengthPct = smoothed
		short[i].Score = smoothed / 100
		cache[sym] = smoothed
	}
}

// SetPipelineState 写入指定交易员的 pipeline 状态
func SetPipelineState(traderID string, state *PipelineState) {
	pipelineStateMu.Lock()
	defer pipelineStateMu.Unlock()
	pipelineState[traderID] = state
}

// GetPipelineState 读取指定交易员的 pipeline 状态
func GetPipelineState(traderID string) *PipelineState {
	pipelineStateMu.RLock()
	defer pipelineStateMu.RUnlock()
	return pipelineState[traderID]
}

// SystemEntry 系统执行开仓单项（由 pipeline 方向池得出，非 AI 输出）
type SystemEntry struct {
	Symbol string // 标的
	Action string // "open_long" | "open_short"
}

// GetSystemEntryList 根据 pipeline 状态与允许方向，返回本周期应由系统执行的开仓列表（每标的至多一个方向；若同时在多空池则取 StrengthPct 更高的一侧）
func GetSystemEntryList(state *PipelineState, opts *PipelineOptions) []SystemEntry {
	if state == nil || len(state.ToSubmitSymbols) == 0 {
		return nil
	}
	if opts == nil {
		opts = &PipelineOptions{AllowLong: true, AllowShort: true}
	}
	longScores := make(map[string]float64)
	for _, i := range state.DirectionLong {
		n := normalizeSymbol(i.Symbol)
		if i.StrengthPct > longScores[n] {
			longScores[n] = i.StrengthPct
		}
	}
	shortScores := make(map[string]float64)
	for _, i := range state.DirectionShort {
		n := normalizeSymbol(i.Symbol)
		if i.StrengthPct > shortScores[n] {
			shortScores[n] = i.StrengthPct
		}
	}
	var out []SystemEntry
	for _, sym := range state.ToSubmitSymbols {
		n := normalizeSymbol(sym)
		inLong := opts.AllowLong && longScores[n] > 0
		inShort := opts.AllowShort && shortScores[n] > 0
		if inLong && inShort {
			if longScores[n] >= shortScores[n] {
				out = append(out, SystemEntry{Symbol: sym, Action: "open_long"})
			} else {
				out = append(out, SystemEntry{Symbol: sym, Action: "open_short"})
			}
		} else if inLong {
			out = append(out, SystemEntry{Symbol: sym, Action: "open_long"})
		} else if inShort {
			out = append(out, SystemEntry{Symbol: sym, Action: "open_short"})
		}
	}
	return out
}

// GetSystemEntryListFromPrediction 预测定多空：三条件共振才开仓（见下）。仅对同时满足三条件的标的生成开仓项；强度与是否执行由调用方用实时数据再校验。
//
// 开仓逻辑梳理：
//  1. 过滤机制：币种通过多层过滤后，系统根据实时数据判定进入方向池（多池或空池），即 state.DirectionLong / DirectionShort、ToSubmitSymbols。
//  2. 实时数据传给 AI：AI 根据实时数据（含方向池、行情等）输出 symbol_predictions，包含 predicted_direction 与 suggest_open。
//  3. 系统再根据实时数据判定方向：仅当标的在「多池」且 AI 预测 up 时允许 open_long，在「空池」且 AI 预测 down 时允许 open_short。
//  4. 三条件共振：① 实时方向（该币在对应方向池）② AI 预测方向（up/down 与开多/开空一致）③ AI 建议开仓（suggest_open=true）。缺一不开仓。
//
// 对通过过滤的标的，同时满足 suggest_open=true、confidence>=minConfidence、predicted_direction 与方向池一侧一致时才加入列表。
// minStrengthToOpen：开仓最低强度，仅当方向池中该标的 StrengthPct >= 此值才加入；<=0 表示不按强度再滤。
// 返回列表按实时方向池强度降序排列，便于优先开高强度的仓。
func GetSystemEntryListFromPrediction(state *PipelineState, opts *PipelineOptions, predictions []SymbolPrediction, minConfidence int, minStrengthToOpen float64) []SystemEntry {
	if state == nil || len(state.ToSubmitSymbols) == 0 {
		return nil
	}
	if opts == nil {
		opts = &PipelineOptions{AllowLong: true, AllowShort: true}
	}
	if len(predictions) == 0 {
		return nil
	}
	predMap := make(map[string]SymbolPrediction)
	for _, p := range predictions {
		n := normalizeSymbol(p.Symbol)
		predMap[n] = p
	}
	longStrength := make(map[string]float64)
	for _, i := range state.DirectionLong {
		n := normalizeSymbol(i.Symbol)
		if i.StrengthPct > longStrength[n] {
			longStrength[n] = i.StrengthPct
		}
	}
	shortStrength := make(map[string]float64)
	for _, i := range state.DirectionShort {
		n := normalizeSymbol(i.Symbol)
		if i.StrengthPct > shortStrength[n] {
			shortStrength[n] = i.StrengthPct
		}
	}
	type entWithStrength struct {
		ent     SystemEntry
		strength float64
	}
	var list []entWithStrength
	for _, sym := range state.ToSubmitSymbols {
		n := normalizeSymbol(sym)
		pred, ok := predMap[n]
		if !ok {
			continue
		}
		// 三条件共振：实时方向 + AI预测方向 + AI建议开仓；仅当 AI 显式 suggest_open=true 时才参与开仓
		if pred.SuggestOpen == nil || !*pred.SuggestOpen {
			continue
		}
		dir := strings.TrimSpace(strings.ToLower(pred.PredictedDirection))
		if pred.Confidence < minConfidence {
			continue
		}
		switch dir {
		case "up":
			// 实时方向：仅当该币在多池（longStrength>0）且满足开仓最低强度时才允许开多
			if opts.AllowLong && longStrength[n] > 0 && (minStrengthToOpen <= 0 || longStrength[n] >= minStrengthToOpen) {
				list = append(list, entWithStrength{SystemEntry{Symbol: sym, Action: "open_long"}, longStrength[n]})
			}
		case "down":
			// 实时方向：仅当该币在空池（shortStrength>0）且满足开仓最低强度时才允许开空
			if opts.AllowShort && shortStrength[n] > 0 && (minStrengthToOpen <= 0 || shortStrength[n] >= minStrengthToOpen) {
				list = append(list, entWithStrength{SystemEntry{Symbol: sym, Action: "open_short"}, shortStrength[n]})
			}
		default:
			// neutral 或未知：不加入
		}
	}
	// 按实时强度降序，优先开高强度的仓
	sort.Slice(list, func(i, j int) bool { return list[i].strength > list[j].strength })
	out := make([]SystemEntry, 0, len(list))
	for _, x := range list {
		out = append(out, x.ent)
	}
	return out
}

// ValidateLayer3 开单前第三层校验：信号年龄、待提交列表、OI 对齐（可选）、方向池强度（可选）
// layer3 为 nil 时仅校验信号年龄与 ToSubmitSymbols，使用默认 5 分钟
func ValidateLayer3(symbol, side, traderID string, layer3 *store.Layer3Config) bool {
	maxMin := 5
	if layer3 != nil && layer3.MaxSignalAgeMinutes > 0 {
		maxMin = layer3.MaxSignalAgeMinutes
	}
	st := GetPipelineState(traderID)
	if st == nil {
		logger.Warnf("ValidateLayer3: no pipeline state for trader %s, skip layer3", traderID)
		return true
	}
	age := time.Since(st.UpdatedAt)
	if age > time.Duration(maxMin)*time.Minute {
		logger.Infof("ValidateLayer3: signal age %v > %d min, reject %s %s", age.Round(time.Second), maxMin, symbol, side)
		return false
	}
	norm := normalizeSymbol(symbol)
	inSubmit := false
	for _, s := range st.ToSubmitSymbols {
		if normalizeSymbol(s) == norm {
			inSubmit = true
			break
		}
	}
	if !inSubmit {
		logger.Infof("ValidateLayer3: %s not in ToSubmitSymbols, reject %s %s", symbol, symbol, side)
		return false
	}
	if layer3 != nil && layer3.OIAlignedRequired {
		if st.OIAlignedLong == nil {
			st.OIAlignedLong = make(map[string]bool)
		}
		if st.OIAlignedShort == nil {
			st.OIAlignedShort = make(map[string]bool)
		}
		if side == "long" && !st.OIAlignedLong[norm] {
			logger.Infof("ValidateLayer3: OI not aligned for long, reject %s", symbol)
			return false
		}
		if side == "short" && !st.OIAlignedShort[norm] {
			logger.Infof("ValidateLayer3: OI not aligned for short, reject %s", symbol)
			return false
		}
	}
	if layer3 != nil && layer3.EntryTimingStrengthMin > 0 {
		minStr := layer3.EntryTimingStrengthMin
		if side == "long" {
			for _, i := range st.DirectionLong {
				if normalizeSymbol(i.Symbol) == norm && i.StrengthPct < minStr {
					logger.Infof("ValidateLayer3: strength %.1f < %.1f for long %s, reject", i.StrengthPct, minStr, symbol)
					return false
				}
			}
		} else {
			for _, i := range st.DirectionShort {
				if normalizeSymbol(i.Symbol) == norm && i.StrengthPct < minStr {
					logger.Infof("ValidateLayer3: strength %.1f < %.1f for short %s, reject", i.StrengthPct, minStr, symbol)
					return false
				}
			}
		}
	}
	return true
}
