// Package monitor 提供交易系统数据调用统计：按系统机制分类，机制下再按数据源分组，便于查看各机制各周期数据是否获取成功。
package monitor

import (
	"sort"
	"sync"
	"time"
)

const maxRecords = 500

// DataCallRecord 单次数据调用记录
type DataCallRecord struct {
	Source     string `json:"source"`
	DataType   string `json:"data_type"`
	Flow       string `json:"flow"` // 主周期, 系统周期, 止盈止损调整
	TraderID   string `json:"trader_id"`
	Success    bool   `json:"success"`
	ErrMsg     string `json:"err_msg"`
	DurationMs int64  `json:"duration_ms"`
	At         int64  `json:"at"`
}

// MechanismDataItem 某机制下某数据源的一项数据（参数/接口）
type MechanismDataItem struct {
	DataType string `json:"data_type"`
	Desc     string `json:"desc"`
}

// MechanismSourceGroup 某机制下的一个数据源及其数据项
type MechanismSourceGroup struct {
	Source string              `json:"source"`
	Items  []MechanismDataItem `json:"items"`
}

// CatalogByMechanism 按机制分类的目录：机制 -> 数据源 -> 数据项列表
type CatalogByMechanism struct {
	Mechanism string                 `json:"mechanism"`
	Sources   []MechanismSourceGroup `json:"sources"`
}

// ByMechanismSource 按「机制 + 数据源」的汇总，用于展示某机制下某数据源在本周期/最近是否获取成功；Items 为该机制+数据源调用的数据项（参数/接口与说明），与目录整合展示
type ByMechanismSource struct {
	Mechanism    string              `json:"mechanism"`
	Source       string              `json:"source"`
	TotalCalls   int                 `json:"total_calls"`
	SuccessCalls int                 `json:"success_calls"`
	LastCallAt   int64               `json:"last_call_at"`
	LastSuccess  bool                `json:"last_success"`
	Items        []MechanismDataItem `json:"items,omitempty"` // 该机制下该数据源调用的数据项（参数/接口 · 说明），便于在统计表中一并展示
}

// DataStatsResponse API 返回结构（按机制优先分类）
type DataStatsResponse struct {
	CatalogByMechanism []CatalogByMechanism `json:"catalog_by_mechanism"` // 各机制分别调用哪些数据（机制下再分数据源）
	ByMechanismSource  []ByMechanismSource  `json:"by_mechanism_source"`  // 各机制下各数据源的调用统计，一眼看出哪个周期哪源失败
	RecentCalls        []DataCallRecord     `json:"recent_calls"`
}

var (
	recorderMu     sync.RWMutex
	records        []DataCallRecord
	catalogByMech  []CatalogByMechanism
	flowToMechanisms map[string][]string // flow -> 所属机制列表，一条记录会计入多个机制
)

func init() {
	records = make([]DataCallRecord, 0, maxRecords)
	flowToMechanisms = map[string][]string{
		"系统周期":     {"过滤机制", "方向池", "系统周期"},
		"主周期":       {"过滤机制", "方向池", "AI实时+预测"},
		"止盈止损调整":  {"止盈止损调整"},
	}
	catalogByMech = buildCatalogByMechanism()
}

func buildCatalogByMechanism() []CatalogByMechanism {
	coinglassWSSItems := []MechanismDataItem{
		{DataType: "WSS", Desc: "WSS 实时(融资率/强平/OI/价格)"},
	}
	coinglassItems := []MechanismDataItem{
		{DataType: "GetOpenInterestExchangeList", Desc: "OI 交易所汇总"},
		{DataType: "GetFundingRateExchangeList", Desc: "资金费率多所"},
		{DataType: "GetGlobalLongShortAccountRatioHistory", Desc: "全账户多空比"},
		{DataType: "GetTopLongShortAccountRatioHistory", Desc: "大户多空比"},
		{DataType: "GetLiquidationAggregatedHistory", Desc: "强平聚合历史"},
		{DataType: "GetLiquidationAggregatedHeatmap", Desc: "强平热力图"},
		{DataType: "GetFearGreedHistory", Desc: "恐惧贪婪指数"},
		{DataType: "GetBitcoinDominance", Desc: "BTC 市值占比"},
		{DataType: "GetAltcoinSeasonHistory", Desc: "山寨季指数"},
		{DataType: "GetETFBitcoinFlowHistory", Desc: "BTC ETF 资金流"},
		{DataType: "GetETFEthereumFlowHistory", Desc: "ETH ETF 资金流"},
		{DataType: "GetWhaleIndexHistory", Desc: "鲸鱼指数"},
		{DataType: "GetCGDIIndexHistory", Desc: "CGDI 多空扩散"},
		{DataType: "GetCDRIIndexHistory", Desc: "CDRI 衍生品风险"},
	}
	marketItems := []MechanismDataItem{
		{DataType: "Get", Desc: "行情数据（K线/指标）"},
		{DataType: "GetKlines", Desc: "K 线"},
	}
	exchangeItems := []MechanismDataItem{
		{DataType: "GetPositions", Desc: "持仓"},
		{DataType: "GetAccount", Desc: "账户"},
	}
	nofxosItems := []MechanismDataItem{
		{DataType: "OIRanking", Desc: "OI 排行"},
		{DataType: "NetFlowRanking", Desc: "资金流排行"},
		{DataType: "PriceRanking", Desc: "涨跌榜"},
	}
	binancedataItems := []MechanismDataItem{
		{DataType: "LongShortRatio", Desc: "币安多空比"},
		{DataType: "FundingRate", Desc: "币安资金费率"},
		{DataType: "TakerVolume", Desc: "Taker 买卖量"},
		{DataType: "LiquidationAgg", Desc: "强平聚合"},
		{DataType: "SpotPrice", Desc: "现货价格"},
	}
	return []CatalogByMechanism{
		// WSS 仅列入「AI实时+预测」：仅主周期调 AI 时 BuildUserPrompt 会消费 CoinglassWSSSnapshot，过滤/方向池/系统周期/止盈止损均不消费
		{
			Mechanism: "过滤机制",
			Sources: []MechanismSourceGroup{
				{Source: "nofxos", Items: nofxosItems},
				{Source: "market", Items: marketItems},
				{Source: "coinglass", Items: coinglassItems},
				{Source: "binancedata", Items: binancedataItems},
				{Source: "exchange", Items: exchangeItems},
			},
		},
		{
			Mechanism: "方向池",
			Sources: []MechanismSourceGroup{
				{Source: "nofxos", Items: nofxosItems},
				{Source: "market", Items: marketItems},
				{Source: "coinglass", Items: coinglassItems},
				{Source: "binancedata", Items: binancedataItems},
				{Source: "exchange", Items: exchangeItems},
			},
		},
		{
			Mechanism: "系统周期",
			Sources: []MechanismSourceGroup{
				{Source: "nofxos", Items: nofxosItems},
				{Source: "market", Items: marketItems},
				{Source: "coinglass", Items: coinglassItems},
				{Source: "binancedata", Items: binancedataItems},
				{Source: "exchange", Items: exchangeItems},
			},
		},
		{
			Mechanism: "AI实时+预测",
			Sources: []MechanismSourceGroup{
				{Source: "nofxos", Items: nofxosItems},
				{Source: "market", Items: marketItems},
				{Source: "coinglass", Items: coinglassItems},
				{Source: "coinglass_wss", Items: coinglassWSSItems},
				{Source: "binancedata", Items: binancedataItems},
				{Source: "exchange", Items: exchangeItems},
			},
		},
		{
			Mechanism: "止盈止损调整",
			Sources: []MechanismSourceGroup{
				{Source: "exchange", Items: []MechanismDataItem{{DataType: "GetPositions", Desc: "持仓"}}},
				{Source: "market", Items: marketItems},
				{Source: "coinglass", Items: coinglassItems},
			},
		},
		{
			Mechanism: "止盈止损检查",
			Sources: []MechanismSourceGroup{
				{Source: "exchange", Items: []MechanismDataItem{{DataType: "GetPositions", Desc: "持仓"}}},
				{Source: "market", Items: marketItems},
			},
		},
	}
}

// RecordDataCall 记录一次数据调用（由 engine/trader 在每次请求外部数据后调用）
func RecordDataCall(source, dataType, flow, traderID string, success bool, errMsg string, durationMs int64) {
	recorderMu.Lock()
	defer recorderMu.Unlock()
	r := DataCallRecord{
		Source:     source,
		DataType:   dataType,
		Flow:       flow,
		TraderID:   traderID,
		Success:    success,
		ErrMsg:     errMsg,
		DurationMs: durationMs,
		At:         time.Now().UTC().UnixMilli(),
	}
	records = append(records, r)
	if len(records) > maxRecords {
		records = records[len(records)-maxRecords:]
	}
}

// GetDataStats 返回按机制分类的目录、各机制下各数据源调用统计、最近调用记录。
func GetDataStats(traderID string) DataStatsResponse {
	recorderMu.RLock()
	defer recorderMu.RUnlock()

	type key struct{ mechanism, source string }
	byKey := make(map[key]*ByMechanismSource)
	var recent []DataCallRecord
	const recentLimit = 100

	for i := len(records) - 1; i >= 0; i-- {
		r := records[i]
		if traderID != "" && r.TraderID != traderID {
			continue
		}
		if len(recent) < recentLimit {
			recent = append(recent, r)
		}
		mechs := flowToMechanisms[r.Flow]
		if len(mechs) == 0 {
			mechs = []string{r.Flow}
		}
		for _, m := range mechs {
			k := key{mechanism: m, source: r.Source}
			s := byKey[k]
			if s == nil {
				s = &ByMechanismSource{Mechanism: m, Source: r.Source}
				byKey[k] = s
			}
			s.TotalCalls++
			if r.Success {
				s.SuccessCalls++
			}
			if r.At > s.LastCallAt {
				s.LastCallAt = r.At
				s.LastSuccess = r.Success
			}
		}
	}

	// 从目录补充每个机制+数据源对应的数据项，便于前端在统计表中一并展示（整合原「一」的目录到「二」）
	catalogMap := make(map[key][]MechanismDataItem)
	for _, cat := range catalogByMech {
		for _, sg := range cat.Sources {
			k := key{mechanism: cat.Mechanism, source: sg.Source}
			catalogMap[k] = sg.Items
		}
	}

	byMechanismSource := make([]ByMechanismSource, 0, len(byKey))
	for _, v := range byKey {
		row := *v
		if items, ok := catalogMap[key{mechanism: row.Mechanism, source: row.Source}]; ok && len(items) > 0 {
			row.Items = items
		}
		byMechanismSource = append(byMechanismSource, row)
	}
	sort.Slice(byMechanismSource, func(i, j int) bool {
		if byMechanismSource[i].Mechanism != byMechanismSource[j].Mechanism {
			return byMechanismSource[i].Mechanism < byMechanismSource[j].Mechanism
		}
		return byMechanismSource[i].Source < byMechanismSource[j].Source
	})
	return DataStatsResponse{
		CatalogByMechanism: catalogByMech,
		ByMechanismSource:  byMechanismSource,
		RecentCalls:        recent,
	}
}
