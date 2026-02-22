package kernel

// ============================================================================
// Trading Data Schema - 交易数据字典
// ============================================================================
// 双语数据字典，支持中文和英文
// 确保AI能够100%理解数据格式，无论使用哪种语言
// ============================================================================

const (
	SchemaVersion = "1.0.0"
)

// Language 语言类型
type Language string

const (
	LangChinese Language = "zh-CN"
	LangEnglish Language = "en-US"
)

// ========== 双语字段定义 ==========

// BilingualFieldDef 双语字段定义
type BilingualFieldDef struct {
	NameZH    string // 中文名称
	NameEN    string // English name
	Unit      string // 单位
	FormulaZH string // 中文公式
	FormulaEN string // English formula
	DescZH    string // 中文描述
	DescEN    string // English description
}

// GetName 获取字段名称（根据语言）
func (d BilingualFieldDef) GetName(lang Language) string {
	if lang == LangChinese {
		return d.NameZH
	}
	return d.NameEN
}

// GetFormula 获取公式（根据语言）
func (d BilingualFieldDef) GetFormula(lang Language) string {
	if lang == LangChinese {
		return d.FormulaZH
	}
	return d.FormulaEN
}

// GetDesc 获取描述（根据语言）
func (d BilingualFieldDef) GetDesc(lang Language) string {
	if lang == LangChinese {
		return d.DescZH
	}
	return d.DescEN
}

// ========== 数据字典 ==========

// DataDictionary 数据字典：定义所有字段的含义
var DataDictionary = map[string]map[string]BilingualFieldDef{
	"AccountMetrics": {
		"Equity": {
			NameZH:    "总权益",
			NameEN:    "Total Equity",
			Unit:      "USDT",
			FormulaZH: "可用余额 + 未实现盈亏",
			FormulaEN: "Available Balance + Unrealized PnL",
			DescZH:    "账户的实际净值，包含所有持仓的浮动盈亏",
			DescEN:    "Actual account value including all unrealized P&L from positions",
		},
		"Balance": {
			NameZH:    "可用余额",
			NameEN:    "Available Balance",
			Unit:      "USDT",
			FormulaZH: "初始资金 + 已实现盈亏",
			FormulaEN: "Initial Capital + Realized PnL",
			DescZH:    "可用于开新仓位的资金，不包括已用保证金",
			DescEN:    "Available funds for opening new positions, excluding used margin",
		},
		"PnL": {
			NameZH:    "总盈亏百分比",
			NameEN:    "Total PnL Percentage",
			Unit:      "%",
			FormulaZH: "(总权益 - 初始资金) / 初始资金 × 100",
			FormulaEN: "(Total Equity - Initial Capital) / Initial Capital × 100",
			DescZH:    "自系统启动以来的总收益率，+15.87%表示盈利15.87%",
			DescEN:    "Total return since inception, +15.87% means 15.87% profit",
		},
		"Margin": {
			NameZH:    "保证金使用率",
			NameEN:    "Margin Usage Rate",
			Unit:      "%",
			FormulaZH: "已用保证金合计 / 总权益 × 100",
			FormulaEN: "Total Used Margin / Total Equity × 100",
			DescZH:    "该值越高，账户风险越大。安全值<30%，危险值>70%",
			DescEN:    "Higher value = higher risk. Safe <30%, Dangerous >70%",
		},
	},

	"TradeMetrics": {
		"Entry": {
			NameZH: "进场价",
			NameEN: "Entry Price",
			Unit:   "USDT",
			DescZH: "开仓时的平均价格",
			DescEN: "Average price when opening position",
		},
		"Exit": {
			NameZH: "出场价",
			NameEN: "Exit Price",
			Unit:   "USDT",
			DescZH: "平仓时的平均价格",
			DescEN: "Average price when closing position",
		},
		"Profit": {
			NameZH:    "已实现盈亏",
			NameEN:    "Realized PnL",
			Unit:      "USDT",
			FormulaZH: "(出场价 - 进场价) / 进场价 × 杠杆 × 仓位价值",
			FormulaEN: "(Exit Price - Entry Price) / Entry Price × Leverage × Position Value",
			DescZH:    "已平仓交易的实际盈亏，包含手续费。正值=盈利，负值=亏损",
			DescEN:    "Actual profit/loss of closed trades including fees. Positive=profit, Negative=loss",
		},
		"PnL%": {
			NameZH:    "盈亏百分比",
			NameEN:    "PnL Percentage",
			Unit:      "%",
			FormulaZH: "(出场价 - 进场价) / 进场价 × 杠杆 × 100",
			FormulaEN: "(Exit - Entry) / Entry × Leverage × 100",
			DescZH:    "已平仓交易的收益率，+6.71%表示盈利6.71%",
			DescEN:    "Return on closed trade, +6.71% means 6.71% profit",
		},
		"HoldDuration": {
			NameZH: "持仓时长",
			NameEN: "Holding Duration",
			Unit:   "minutes",
			DescZH: "从开仓到平仓的时间。<15分钟=超短线，15分钟-4小时=日内，>4小时=波段",
			DescEN: "Time from open to close. <15min=scalping, 15min-4h=intraday, >4h=swing",
		},
	},

	"PositionMetrics": {
		"UnrealizedPnL%": {
			NameZH:    "未实现盈亏百分比",
			NameEN:    "Unrealized PnL Percentage",
			Unit:      "%",
			FormulaZH: "(当前价 - 进场价) / 进场价 × 杠杆 × 100",
			FormulaEN: "(Current Price - Entry Price) / Entry Price × Leverage × 100",
			DescZH:    "当前持仓的浮动盈亏，未平仓前是浮动的",
			DescEN:    "Floating P&L of current position, not realized until closed",
		},
		"PeakPnL%": {
			NameZH: "峰值盈亏百分比",
			NameEN: "Peak PnL Percentage",
			Unit:   "%",
			DescZH: "该持仓曾经达到的最高未实现盈亏。用于判断是否需要止盈",
			DescEN: "Historical max unrealized PnL for this position. Used for take-profit decisions",
		},
		"Drawdown": {
			NameZH:    "从峰值回撤",
			NameEN:    "Drawdown from Peak",
			Unit:      "%",
			FormulaZH: "当前盈亏% - 峰值盈亏%",
			FormulaEN: "Current PnL% - Peak PnL%",
			DescZH:    "负值表示正在回撤。例如：峰值+5%，当前+3%，回撤=-2%",
			DescEN:    "Negative = pulling back. E.g., Peak +5%, Current +3%, Drawdown = -2%",
		},
		"Leverage": {
			NameZH: "杠杆倍数",
			NameEN: "Leverage",
			Unit:   "x",
			DescZH: "3x表示价格变动1%，持仓盈亏变动3%。杠杆越高，风险越大",
			DescEN: "3x means 1% price move = 3% position PnL. Higher leverage = higher risk",
		},
		"Margin": {
			NameZH:    "占用保证金",
			NameEN:    "Margin Used",
			Unit:      "USDT",
			FormulaZH: "仓位价值 / 杠杆",
			FormulaEN: "Position Value / Leverage",
			DescZH:    "该仓位锁定的保证金金额",
			DescEN:    "Collateral locked for this position",
		},
		"LiqPrice": {
			NameZH: "强平价格",
			NameEN: "Liquidation Price",
			Unit:   "USDT",
			DescZH: "价格触及此值时会被强制平仓。0.0000表示无爆仓风险",
			DescEN: "Price at which position will be force-closed. 0.0000 = no liquidation risk",
		},
	},

	"MarketData": {
		"Volume": {
			NameZH: "成交量",
			NameEN: "Volume",
			Unit:   "base asset",
			DescZH: "该时间段的交易量",
			DescEN: "Trading volume in this period",
		},
		"OI": {
			NameZH: "持仓量",
			NameEN: "Open Interest",
			Unit:   "USDT",
			DescZH: "未平仓合约的总价值。持仓量增加=资金流入，减少=资金流出",
			DescEN: "Total value of open contracts. Increasing OI = capital inflow, decreasing = outflow",
		},
		"OIChange": {
			NameZH: "持仓量变化",
			NameEN: "OI Change",
			Unit:   "USDT & %",
			DescZH: "1小时内持仓量的变化。用于判断市场真实资金流向",
			DescEN: "OI change in 1 hour. Used to determine real capital flow direction",
		},
	},

	"TechnicalIndicators": {
		"EMA": {
			NameZH:    "指数移动平均线",
			NameEN:    "Exponential Moving Average",
			Unit:      "USDT",
			FormulaZH: "EMA = 前一日EMA × (n-1)/(n+1) + 今日收盘价 × 2/(n+1)",
			FormulaEN: "EMA = Previous EMA × (n-1)/(n+1) + Today's Close × 2/(n+1)",
			DescZH:    "趋势指标。价格在EMA上方=多头趋势，下方=空头趋势。EMA20金叉EMA50=买入信号",
			DescEN:    "Trend indicator. Price above EMA = bullish, below = bearish. EMA20 crosses above EMA50 = buy signal",
		},
		"MACD": {
			NameZH:    "指数平滑异同移动平均线",
			NameEN:    "Moving Average Convergence Divergence",
			Unit:      "points",
			FormulaZH: "MACD = EMA12 - EMA26, Signal = EMA9(MACD)",
			FormulaEN: "MACD = EMA12 - EMA26, Signal = EMA9(MACD)",
			DescZH:    "动量指标。MACD>0=多头，<0=空头。MACD上穿信号线=金叉(买入)，下穿=死叉(卖出)",
			DescEN:    "Momentum indicator. MACD>0=bullish, <0=bearish. MACD crosses above signal=golden cross(buy), below=death cross(sell)",
		},
		"RSI": {
			NameZH:    "相对强弱指标",
			NameEN:    "Relative Strength Index",
			Unit:      "0-100",
			FormulaZH: "RSI = 100 - 100/(1 + 平均涨幅/平均跌幅)",
			FormulaEN: "RSI = 100 - 100/(1 + Average Gain/Average Loss)",
			DescZH:    "超买超卖指标。RSI>70=超买(考虑卖出)，<30=超卖(考虑买入)，40-60=中性区间",
			DescEN:    "Overbought/oversold indicator. RSI>70=overbought(consider sell), <30=oversold(consider buy), 40-60=neutral",
		},
		"ATR": {
			NameZH:    "平均真实波幅",
			NameEN:    "Average True Range",
			Unit:      "USDT",
			FormulaZH: "ATR = MA14(max(高-低, |高-昨收|, |低-昨收|))",
			FormulaEN: "ATR = MA14(max(High-Low, |High-PrevClose|, |Low-PrevClose|))",
			DescZH:    "波动率指标。ATR越大=波动越大。系统支持动态ATR倍数：止损1.5-2.5倍，止盈2.5-4.0倍，AI需根据市场波动率和币种特性选择合适倍数",
			DescEN:    "Volatility indicator. Higher ATR = higher volatility. System supports dynamic ATR multipliers: stop-loss 1.5-2.5x, take-profit 2.5-4.0x, AI should choose appropriate multiplier based on market volatility and coin characteristics",
		},
		"BOLL": {
			NameZH:    "布林带",
			NameEN:    "Bollinger Bands",
			Unit:      "USDT",
			FormulaZH: "中轨=MA20, 上轨=中轨+2×标准差, 下轨=中轨-2×标准差",
			FormulaEN: "Middle=MA20, Upper=Middle+2×StdDev, Lower=Middle-2×StdDev",
			DescZH:    "波动通道。价格触及上轨=超买，触及下轨=超卖。突破上轨=强势突破，跌破下轨=弱势破位",
			DescEN:    "Volatility channel. Price at upper band=overbought, at lower band=oversold. Break above upper=strong breakout, below lower=weak breakdown",
		},
		"FundingRate": {
			NameZH:    "资金费率",
			NameEN:    "Funding Rate",
			Unit:      "%",
			FormulaZH: "每8小时结算一次",
			FormulaEN: "Settled every 8 hours",
			DescZH:    "多空平衡指标。正值=多头支付空头(市场看多)，负值=空头支付多头(市场看空)。>0.1%=极度看多，<-0.1%=极度看空",
			DescEN:    "Long/short balance. Positive=longs pay shorts(bullish), negative=shorts pay longs(bearish). >0.1%=extremely bullish, <-0.1%=extremely bearish",
		},
	},

	"QuantData": {
		"InstitutionFlow": {
			NameZH:    "机构资金流",
			NameEN:    "Institutional Flow",
			Unit:      "USDT",
			FormulaZH: "机构账户的净买入金额",
			FormulaEN: "Net buying amount from institutional accounts",
			DescZH:    "机构资金流向。正值=机构买入，负值=机构卖出。机构买入+散户卖出=强烈看涨信号",
			DescEN:    "Institutional capital flow. Positive=institutions buying, negative=selling. Institutions buying + retail selling = strong bullish signal",
		},
		"RetailFlow": {
			NameZH:    "散户资金流",
			NameEN:    "Retail Flow",
			Unit:      "USDT",
			FormulaZH: "散户账户的净买入金额",
			FormulaEN: "Net buying amount from retail accounts",
			DescZH:    "散户资金流向。正值=散户买入，负值=散户卖出。散户买入+机构卖出=警惕信号(可能是顶部)",
			DescEN:    "Retail capital flow. Positive=retail buying, negative=selling. Retail buying + institutions selling = warning signal(possible top)",
		},
		"PriceChange": {
			NameZH:    "多周期价格变化",
			NameEN:    "Multi-period Price Change",
			Unit:      "%",
			FormulaZH: "(当前价 - N周期前价格) / N周期前价格 × 100",
			FormulaEN: "(Current Price - N-period ago Price) / N-period ago Price × 100",
			DescZH:    "不同时间周期的涨跌幅。用于判断短期动量和长期趋势的一致性",
			DescEN:    "Price changes across different timeframes. Used to assess consistency between short-term momentum and long-term trend",
		},
	},
}

// ========== 双语规则定义 ==========

// BilingualRuleDef 双语规则定义
type BilingualRuleDef struct {
	Value    interface{} // 规则值
	DescZH   string      // 中文描述
	DescEN   string      // English description
	ReasonZH string      // 中文原因
	ReasonEN string      // English reason
}

// GetDesc 获取描述（根据语言）
func (d BilingualRuleDef) GetDesc(lang Language) string {
	if lang == LangChinese {
		return d.DescZH
	}
	return d.DescEN
}

// GetReason 获取原因（根据语言）
func (d BilingualRuleDef) GetReason(lang Language) string {
	if lang == LangChinese {
		return d.ReasonZH
	}
	return d.ReasonEN
}

// ========== 交易规则 ==========

// TradingRules 交易规则定义
var TradingRules = struct {
	RiskManagement  map[string]BilingualRuleDef
	EntrySignals    map[string]BilingualRuleDef
	ExitSignals     map[string]BilingualRuleDef
	PositionControl map[string]BilingualRuleDef
}{
	RiskManagement: map[string]BilingualRuleDef{
		"MaxMarginUsage": {
			Value:    0.30,
			DescZH:   "保证金使用率不得超过30%",
			DescEN:   "Margin usage must not exceed 30%",
			ReasonZH: "保留70%的资金应对极端行情和追加保证金",
			ReasonEN: "Reserve 70% capital for extreme market conditions and margin calls",
		},
		"MaxPositionLoss": {
			Value:    -0.05,
			DescZH:   "单个持仓亏损达到-5%时必须止损",
			DescEN:   "Must stop-loss when single position loss reaches -5%",
			ReasonZH: "避免单笔交易造成过大损失",
			ReasonEN: "Prevent excessive loss from single trade",
		},
		"MaxDailyLoss": {
			Value:    -0.10,
			DescZH:   "单日亏损达到-10%时停止交易",
			DescEN:   "Stop trading when daily loss reaches -10%",
			ReasonZH: "防止情绪化交易导致连续亏损",
			ReasonEN: "Prevent emotional trading leading to consecutive losses",
		},
		"PositionSizeLimit": {
			Value:    0.15,
			DescZH:   "单个仓位不得超过总权益的15%",
			DescEN:   "Single position must not exceed 15% of total equity",
			ReasonZH: "避免过度集中风险",
			ReasonEN: "Avoid excessive risk concentration",
		},
	},

	EntrySignals: map[string]BilingualRuleDef{
		"VolumeSpike": {
			Value:    2.0,
			DescZH:   "成交量是平均值的2倍以上时考虑进场",
			DescEN:   "Consider entry when volume is 2x above average",
			ReasonZH: "放量突破通常意味着强趋势",
			ReasonEN: "Volume breakout usually indicates strong trend",
		},
		"OIChangeThreshold": {
			Value:    0.02,
			DescZH:   "持仓量1小时内变化超过2%视为显著变化",
			DescEN:   "OI change >2% in 1 hour is considered significant",
			ReasonZH: "大额资金进出会导致持仓量显著变化",
			ReasonEN: "Large capital flows cause significant OI changes",
		},
	},

	ExitSignals: map[string]BilingualRuleDef{
		"TrailingStop": {
			Value:    0.30,
			DescZH:   "当盈亏从峰值回撤30%时平仓止盈",
			DescEN:   "Close position when PnL pulls back 30% from peak",
			ReasonZH: "锁定大部分利润，避免盈利回吐。例如：峰值+5%，回撤到+3.5%时平仓",
			ReasonEN: "Lock in most profits, avoid profit giveback. E.g., Peak +5%, close at +3.5%",
		},
		"StopLoss": {
			Value:    -0.05,
			DescZH:   "硬止损设置在-5%",
			DescEN:   "Hard stop-loss at -5%",
			ReasonZH: "严格控制单笔最大损失",
			ReasonEN: "Strictly control maximum single-trade loss",
		},
	},

	PositionControl: map[string]BilingualRuleDef{
		"ScaleIn": {
			Value:    map[string]interface{}{"enabled": true, "max_additions": 2, "price_requirement": 0.01},
			DescZH:   "只在盈利仓位上加仓，最多加2次，价格需比平均成本高1%",
			DescEN:   "Only add to winning positions, max 2 additions, price must be 1% above avg cost",
			ReasonZH: "顺势加仓，不追亏损",
			ReasonEN: "Add to winners, never average down losers",
		},
		"ScaleOut": {
			Value: []map[string]interface{}{
				{"pnl": 0.03, "close_pct": 0.33},
				{"pnl": 0.05, "close_pct": 0.50},
				{"pnl": 0.08, "close_pct": 1.00},
			},
			DescZH:   "分批止盈：盈利3%时平33%，5%时平50%，8%时全平",
			DescEN:   "Scale-out: Close 33% at +3%, 50% at +5%, 100% at +8%",
			ReasonZH: "在保证利润的同时让盈利奔跑",
			ReasonEN: "Lock profits while letting winners run",
		},
	},
}

// ========== OI解读 ==========

// OIInterpretation OI变化的市场解读（双语）
type OIInterpretationType struct {
	OIUp_PriceUp struct {
		ZH string
		EN string
	}
	OIUp_PriceDown struct {
		ZH string
		EN string
	}
	OIDown_PriceUp struct {
		ZH string
		EN string
	}
	OIDown_PriceDown struct {
		ZH string
		EN string
	}
}

var OIInterpretation = OIInterpretationType{
	OIUp_PriceUp: struct {
		ZH string
		EN string
	}{
		ZH: "强多头趋势（新多单开仓，资金流入做多）",
		EN: "Strong bullish trend (new longs opening, capital flowing into long positions)",
	},
	OIUp_PriceDown: struct {
		ZH string
		EN string
	}{
		ZH: "强空头趋势（新空单开仓，资金流入做空）",
		EN: "Strong bearish trend (new shorts opening, capital flowing into short positions)",
	},
	OIDown_PriceUp: struct {
		ZH string
		EN string
	}{
		ZH: "空头平仓（空头止损离场，可能出现反转）",
		EN: "Shorts covering (shorts stopped out, potential reversal)",
	},
	OIDown_PriceDown: struct {
		ZH string
		EN string
	}{
		ZH: "多头平仓（多头止损离场，可能出现反转）",
		EN: "Longs closing (longs stopped out, potential reversal)",
	},
}

// ========== 常见错误 ==========

// CommonMistake 常见错误定义
type CommonMistake struct {
	ErrorZH   string
	ErrorEN   string
	ExampleZH string
	ExampleEN string
	CorrectZH string
	CorrectEN string
}

var CommonMistakes = []CommonMistake{
	{
		ErrorZH:   "混淆已实现盈亏和未实现盈亏",
		ErrorEN:   "Confusing realized and unrealized P&L",
		ExampleZH: "将历史交易的盈亏与当前持仓的盈亏相加",
		ExampleEN: "Adding historical trade P&L with current position P&L",
		CorrectZH: "已实现盈亏已经计入账户余额，不应重复计算",
		CorrectEN: "Realized P&L is already included in account balance, don't double count",
	},
	{
		ErrorZH:   "忽略杠杆对盈亏的影响",
		ErrorEN:   "Ignoring leverage's impact on P&L",
		ExampleZH: "价格涨1%，认为盈利1%",
		ExampleEN: "Price up 1%, thinking profit is 1%",
		CorrectZH: "3x杠杆时，价格涨1%，实际盈利约3%",
		CorrectEN: "With 3x leverage, 1% price move = ~3% P&L",
	},
	{
		ErrorZH:   "不理解Peak PnL的重要性",
		ErrorEN:   "Not understanding Peak PnL's importance",
		ExampleZH: "只关注当前PnL，不关注回撤",
		ExampleEN: "Only watching current PnL, ignoring drawdown",
		CorrectZH: "当前PnL接近Peak PnL时，应考虑止盈以锁定利润",
		CorrectEN: "When current PnL near Peak PnL, consider taking profit to lock in gains",
	},
	{
		ErrorZH:   "忽略持仓量(OI)变化",
		ErrorEN:   "Ignoring Open Interest changes",
		ExampleZH: "只看价格K线，不看资金流向",
		ExampleEN: "Only watching price candles, not capital flows",
		CorrectZH: "结合OI变化判断趋势的真实性和持续性",
		CorrectEN: "Use OI changes to validate trend authenticity and sustainability",
	},
}

// ========== Prompt生成函数 ==========

// GetSchemaPrompt 生成Schema说明文本，用于AI Prompt
func GetSchemaPrompt(lang Language) string {
	if lang == LangChinese {
		return getSchemaPromptZH()
	}
	return getSchemaPromptEN()
}

// getSchemaPromptZH 生成中文Prompt
func getSchemaPromptZH() string {
	prompt := "# 📖 数据字典与交易规则\n\n"
	prompt += "## 📊 字段含义说明\n\n"

	// 账户指标
	prompt += "### 账户指标\n"
	for key, field := range DataDictionary["AccountMetrics"] {
		prompt += formatFieldDefZH(key, field)
	}

	// 交易指标
	prompt += "\n### 交易指标\n"
	for key, field := range DataDictionary["TradeMetrics"] {
		prompt += formatFieldDefZH(key, field)
	}

	// 持仓指标
	prompt += "\n### 持仓指标\n"
	for key, field := range DataDictionary["PositionMetrics"] {
		prompt += formatFieldDefZH(key, field)
	}

	// 市场数据
	prompt += "\n### 市场数据\n"
	for key, field := range DataDictionary["MarketData"] {
		prompt += formatFieldDefZH(key, field)
	}

	// 技术指标
	prompt += "\n### 技术指标\n"
	for key, field := range DataDictionary["TechnicalIndicators"] {
		prompt += formatFieldDefZH(key, field)
	}

	// 量化数据
	prompt += "\n### 量化数据\n"
	for key, field := range DataDictionary["QuantData"] {
		prompt += formatFieldDefZH(key, field)
	}

	// OI解读
	prompt += "\n## 💹 持仓量(OI)变化解读\n\n"
	prompt += "- **OI增加 + 价格上涨**: " + OIInterpretation.OIUp_PriceUp.ZH + "\n"
	prompt += "- **OI增加 + 价格下跌**: " + OIInterpretation.OIUp_PriceDown.ZH + "\n"
	prompt += "- **OI减少 + 价格上涨**: " + OIInterpretation.OIDown_PriceUp.ZH + "\n"
	prompt += "- **OI减少 + 价格下跌**: " + OIInterpretation.OIDown_PriceDown.ZH + "\n"

	// 市场状态识别
	prompt += "\n## 📊 市场状态识别\n\n"
	prompt += getMarketRegimeGuideZH()

	// 多时间框架分析
	prompt += "\n## ⏱️ 多时间框架分析指南\n\n"
	prompt += getMultiTimeframeGuideZH()

	// 交易场景
	prompt += "\n## 🎯 交易场景示例\n\n"
	prompt += getTradingScenarioExamplesZH()

	return prompt
}

// getSchemaPromptEN 生成英文Prompt
func getSchemaPromptEN() string {
	prompt := "# 📖 Data Dictionary & Trading Rules\n\n"
	prompt += "## 📊 Field Definitions\n\n"

	// Account Metrics
	prompt += "### Account Metrics\n"
	for key, field := range DataDictionary["AccountMetrics"] {
		prompt += formatFieldDefEN(key, field)
	}

	// Trade Metrics
	prompt += "\n### Trade Metrics\n"
	for key, field := range DataDictionary["TradeMetrics"] {
		prompt += formatFieldDefEN(key, field)
	}

	// Position Metrics
	prompt += "\n### Position Metrics\n"
	for key, field := range DataDictionary["PositionMetrics"] {
		prompt += formatFieldDefEN(key, field)
	}

	// Market Data
	prompt += "\n### Market Data\n"
	for key, field := range DataDictionary["MarketData"] {
		prompt += formatFieldDefEN(key, field)
	}

	// Technical Indicators
	prompt += "\n### Technical Indicators\n"
	for key, field := range DataDictionary["TechnicalIndicators"] {
		prompt += formatFieldDefEN(key, field)
	}

	// Quant Data
	prompt += "\n### Quantitative Data\n"
	for key, field := range DataDictionary["QuantData"] {
		prompt += formatFieldDefEN(key, field)
	}

	// OI Interpretation
	prompt += "\n## 💹 Open Interest (OI) Change Interpretation\n\n"
	prompt += "- **OI Up + Price Up**: " + OIInterpretation.OIUp_PriceUp.EN + "\n"
	prompt += "- **OI Up + Price Down**: " + OIInterpretation.OIUp_PriceDown.EN + "\n"
	prompt += "- **OI Down + Price Up**: " + OIInterpretation.OIDown_PriceUp.EN + "\n"
	prompt += "- **OI Down + Price Down**: " + OIInterpretation.OIDown_PriceDown.EN + "\n"

	// Market Regime Recognition
	prompt += "\n## 📊 Market Regime Recognition\n\n"
	prompt += getMarketRegimeGuideEN()

	// Multi-Timeframe Analysis
	prompt += "\n## ⏱️ Multi-Timeframe Analysis Guide\n\n"
	prompt += getMultiTimeframeGuideEN()

	// Trading Scenarios
	prompt += "\n## 🎯 Trading Scenario Examples\n\n"
	prompt += getTradingScenarioExamplesEN()

	return prompt
}

// formatFieldDefZH 格式化中文字段定义
func formatFieldDefZH(key string, field BilingualFieldDef) string {
	result := "- **" + key + "**（" + field.NameZH + "）: " + field.DescZH
	if field.FormulaZH != "" {
		result += " | 公式: `" + field.FormulaZH + "`"
	}
	if field.Unit != "" {
		result += " | 单位: " + field.Unit
	}
	result += "\n"
	return result
}

// formatFieldDefEN 格式化英文字段定义
func formatFieldDefEN(key string, field BilingualFieldDef) string {
	result := "- **" + key + "** (" + field.NameEN + "): " + field.DescEN
	if field.FormulaEN != "" {
		result += " | Formula: `" + field.FormulaEN + "`"
	}
	if field.Unit != "" {
		result += " | Unit: " + field.Unit
	}
	result += "\n"
	return result
}

// ========== 市场状态识别指南 ==========

// getMarketRegimeGuideZH 获取中文市场状态识别指南
func getMarketRegimeGuideZH() string {
	return `### 趋势判断
- **强上升趋势**: 价格持续在EMA20上方 + MACD>0且上升 + RSI>50 + OI增加
- **上升趋势**: 价格在EMA20上方 + MACD>0 + RSI 40-70
- **横盘震荡**: 价格围绕EMA20波动 + MACD接近0 + RSI 40-60 + 1h波动率<2%
- **下降趋势**: 价格在EMA20下方 + MACD<0 + RSI 30-60
- **强下降趋势**: 价格持续在EMA20下方 + MACD<0且下降 + RSI<50 + OI增加

### 波动率判断
- **低波动**: ATR < 平均ATR的0.7倍，适合网格策略
- **正常波动**: ATR在平均ATR的0.7-1.3倍之间
- **高波动**: ATR > 平均ATR的1.3倍，降低杠杆，扩大止损
- **极端波动**: ATR > 平均ATR的2倍，暂停交易或仅平仓

### 成交量判断
- **放量**: 当前成交量 > 平均成交量的1.5倍，趋势可靠
- **缩量**: 当前成交量 < 平均成交量的0.7倍，趋势可能反转
- **天量**: 当前成交量 > 平均成交量的3倍，警惕反转

## 🎯 动态止损止盈（ATR 自适应）

系统支持基于 ATR 的动态止损止盈，你需要根据市场情况在允许的范围内选择合适的 ATR 倍数：

### 止损 ATR 倍数选择（允许范围：1.5-2.5倍）

**高波动市场（ATR > 1.5倍平均）**：
- 建议使用 2.0-2.5 倍 ATR
- 原因：避免正常波动触发止损，给趋势足够空间

**正常波动市场（ATR 0.8-1.5倍平均）**：
- 建议使用 1.5-2.0 倍 ATR
- 原因：平衡风险控制和止损空间

**低波动市场（ATR < 0.8倍平均）**：
- 建议使用 1.5-1.8 倍 ATR
- 原因：低波动时收紧止损，提高资金效率

**币种差异**：
- BTC/ETH：建议使用较大倍数（2.0-2.5倍），价格波动相对稳定
- 主流山寨币：建议使用中等倍数（1.8-2.2倍）
- 小市值山寨币：建议使用较小倍数（1.5-2.0倍），波动剧烈需严控风险

### 止盈 ATR 倍数选择（允许范围：2.5-4.0倍）

**强趋势市场（多周期共振+OI持续增加）**：
- 建议使用 3.5-4.0 倍 ATR
- 原因：让利润充分奔跑，捕捉大行情

**正常趋势市场（趋势明确但动能一般）**：
- 建议使用 2.8-3.5 倍 ATR
- 原因：平衡止盈目标和回撤风险

**弱趋势/震荡市场（方向不明确）**：
- 建议使用 2.5-3.0 倍 ATR
- 原因：快速止盈，避免利润回吐

**币种差异**：
- BTC/ETH：可以使用较大倍数（3.0-4.0倍），趋势持续性好
- 山寨币：建议使用较小倍数（2.5-3.5倍），快速止盈锁定利润

### 实战示例

**示例1：BTC 高波动突破做多**
- 当前 ATR: 450 USDT，平均 ATR: 300 USDT（1.5倍平均，高波动）
- 入场价: 95000 USDT
- 市场状态: 4h强上升趋势，OI持续增加，机构资金流入
- 止损倍数选择: 2.5倍（高波动+BTC）
- 止损价: 95000 - 2.5×450 = 93875 USDT
- 止盈倍数选择: 4.0倍（强趋势+BTC）
- 止盈价: 95000 + 4.0×450 = 96800 USDT
- 理由: BTC高波动用大倍数避免震出，强趋势让利润充分奔跑

**示例2：山寨币正常波动做多**
- 当前 ATR: 0.05 USDT，平均 ATR: 0.05 USDT（1.0倍平均，正常波动）
- 入场价: 1.50 USDT
- 市场状态: 1h上升趋势，OI增加，但4h震荡
- 止损倍数选择: 1.8倍（正常波动+山寨币）
- 止损价: 1.50 - 1.8×0.05 = 1.41 USDT
- 止盈倍数选择: 3.0倍（正常趋势+山寨币）
- 止盈价: 1.50 + 3.0×0.05 = 1.65 USDT
- 理由: 山寨币波动大用中等倍数，4h震荡所以止盈不宜过大

**示例3：小市值币低波动做多**
- 当前 ATR: 0.008 USDT，平均 ATR: 0.012 USDT（0.67倍平均，低波动）
- 入场价: 0.50 USDT
- 市场状态: 15m突破，但1h和4h方向不明
- 止损倍数选择: 1.5倍（低波动+小市值+弱趋势）
- 止损价: 0.50 - 1.5×0.008 = 0.488 USDT
- 止盈倍数选择: 2.5倍（弱趋势+快速止盈）
- 止盈价: 0.50 + 2.5×0.008 = 0.52 USDT
- 理由: 低波动收紧止损，弱趋势快速止盈避免反转

### 重要提醒

1. **必须在允许范围内选择**: 止损1.5-2.5倍，止盈2.5-4.0倍，不能超出范围
2. **综合考虑多个因素**: 波动率、币种特性、趋势强度、多周期共振
3. **动态调整**: 不同市场状态使用不同倍数，不要固定使用某个值
4. **风险优先**: 不确定时选择较小倍数，控制风险
5. **记录理由**: 在reasoning中说明为什么选择该倍数

`
}

// getMarketRegimeGuideEN 获取英文市场状态识别指南
func getMarketRegimeGuideEN() string {
	return `### Trend Identification
- **Strong Uptrend**: Price consistently above EMA20 + MACD>0 and rising + RSI>50 + OI increasing
- **Uptrend**: Price above EMA20 + MACD>0 + RSI 40-70
- **Sideways**: Price oscillating around EMA20 + MACD near 0 + RSI 40-60 + 1h volatility<2%
- **Downtrend**: Price below EMA20 + MACD<0 + RSI 30-60
- **Strong Downtrend**: Price consistently below EMA20 + MACD<0 and falling + RSI<50 + OI increasing

### Volatility Assessment
- **Low Volatility**: ATR < 0.7× average ATR, suitable for grid strategies
- **Normal Volatility**: ATR between 0.7-1.3× average ATR
- **High Volatility**: ATR > 1.3× average ATR, reduce leverage, widen stop-loss
- **Extreme Volatility**: ATR > 2× average ATR, pause trading or close only

### Volume Assessment
- **Volume Surge**: Current volume > 1.5× average, trend is reliable
- **Volume Decline**: Current volume < 0.7× average, trend may reverse
- **Climax Volume**: Current volume > 3× average, watch for reversal

## 🎯 Dynamic Stop-Loss & Take-Profit (ATR Adaptive)

The system supports ATR-based dynamic stop-loss and take-profit. You need to choose appropriate ATR multipliers within the allowed ranges based on market conditions:

### Stop-Loss ATR Multiplier Selection (Allowed Range: 1.5-2.5x)

**High Volatility Market (ATR > 1.5× average)**:
- Recommend using 2.0-2.5× ATR
- Reason: Avoid stop-out from normal volatility, give trend enough room

**Normal Volatility Market (ATR 0.8-1.5× average)**:
- Recommend using 1.5-2.0× ATR
- Reason: Balance risk control and stop-loss room

**Low Volatility Market (ATR < 0.8× average)**:
- Recommend using 1.5-1.8× ATR
- Reason: Tighten stop-loss in low volatility, improve capital efficiency

**Coin Differences**:
- BTC/ETH: Recommend larger multipliers (2.0-2.5x), relatively stable price movement
- Major altcoins: Recommend medium multipliers (1.8-2.2x)
- Small-cap altcoins: Recommend smaller multipliers (1.5-2.0x), high volatility requires strict risk control

### Take-Profit ATR Multiplier Selection (Allowed Range: 2.5-4.0x)

**Strong Trend Market (Multi-timeframe alignment + OI continuously increasing)**:
- Recommend using 3.5-4.0× ATR
- Reason: Let profits run, capture big moves

**Normal Trend Market (Clear trend but moderate momentum)**:
- Recommend using 2.8-3.5× ATR
- Reason: Balance take-profit target and pullback risk

**Weak Trend/Choppy Market (Direction unclear)**:
- Recommend using 2.5-3.0× ATR
- Reason: Quick take-profit, avoid profit giveback

**Coin Differences**:
- BTC/ETH: Can use larger multipliers (3.0-4.0x), good trend persistence
- Altcoins: Recommend smaller multipliers (2.5-3.5x), quick profit-taking to lock gains

### Practical Examples

**Example 1: BTC High Volatility Breakout Long**
- Current ATR: 450 USDT, Average ATR: 300 USDT (1.5x average, high volatility)
- Entry Price: 95000 USDT
- Market State: 4h strong uptrend, OI continuously increasing, institutional inflow
- Stop-Loss Multiplier: 2.5x (high volatility + BTC)
- Stop-Loss Price: 95000 - 2.5×450 = 93875 USDT
- Take-Profit Multiplier: 4.0x (strong trend + BTC)
- Take-Profit Price: 95000 + 4.0×450 = 96800 USDT
- Reason: BTC high volatility uses large multiplier to avoid shake-out, strong trend lets profits run

**Example 2: Altcoin Normal Volatility Long**
- Current ATR: 0.05 USDT, Average ATR: 0.05 USDT (1.0x average, normal volatility)
- Entry Price: 1.50 USDT
- Market State: 1h uptrend, OI increasing, but 4h sideways
- Stop-Loss Multiplier: 1.8x (normal volatility + altcoin)
- Stop-Loss Price: 1.50 - 1.8×0.05 = 1.41 USDT
- Take-Profit Multiplier: 3.0x (normal trend + altcoin)
- Take-Profit Price: 1.50 + 3.0×0.05 = 1.65 USDT
- Reason: Altcoin high volatility uses medium multiplier, 4h sideways so take-profit not too large

**Example 3: Small-Cap Coin Low Volatility Long**
- Current ATR: 0.008 USDT, Average ATR: 0.012 USDT (0.67x average, low volatility)
- Entry Price: 0.50 USDT
- Market State: 15m breakout, but 1h and 4h direction unclear
- Stop-Loss Multiplier: 1.5x (low volatility + small-cap + weak trend)
- Stop-Loss Price: 0.50 - 1.5×0.008 = 0.488 USDT
- Take-Profit Multiplier: 2.5x (weak trend + quick profit)
- Take-Profit Price: 0.50 + 2.5×0.008 = 0.52 USDT
- Reason: Low volatility tightens stop-loss, weak trend quick profit to avoid reversal

### Important Reminders

1. **Must choose within allowed ranges**: Stop-loss 1.5-2.5x, take-profit 2.5-4.0x, cannot exceed ranges
2. **Consider multiple factors**: Volatility, coin characteristics, trend strength, multi-timeframe alignment
3. **Dynamic adjustment**: Use different multipliers for different market states, don't fix on one value
4. **Risk first**: When uncertain, choose smaller multipliers to control risk
5. **Document reasoning**: Explain in reasoning why you chose that multiplier

`
}

// ========== 多时间框架分析指南 ==========

// getMultiTimeframeGuideZH 获取中文多时间框架分析指南
func getMultiTimeframeGuideZH() string {
	return `### 时间框架层级
- **15m**: 精确入场时机，寻找最佳入场点
- **1h**: 趋势确认，判断短期趋势方向
- **4h**: 大趋势判断，确定主要趋势方向

### 分析原则
1. **大周期定方向**: 先看4h确定大趋势（做多/做空/观望）
2. **中周期找时机**: 再看1h确认短期趋势是否与大趋势一致
3. **小周期精确入场**: 最后看15m寻找具体入场点

### 多空判断
- **强烈看多**: 4h上升 + 1h上升 + 15m上升（三周期共振）
- **看多**: 4h上升 + 1h上升 + 15m震荡/回调（等待15m转多）
- **谨慎看多**: 4h上升 + 1h震荡 + 15m上升（1h可能转多）
- **观望**: 4h震荡 或 各周期方向不一致
- **谨慎看空**: 4h下降 + 1h震荡 + 15m下降（1h可能转空）
- **看空**: 4h下降 + 1h下降 + 15m震荡/反弹（等待15m转空）
- **强烈看空**: 4h下降 + 1h下降 + 15m下降（三周期共振）

### 实战案例
**场景**: 4h强上升趋势，1h刚突破EMA20，15m回调到EMA20附近
**判断**: 大趋势向上，短期突破确认，小周期回调提供入场机会
**操作**: 在15m EMA20附近做多，止损设在15m EMA20下方

`
}

// getMultiTimeframeGuideEN 获取英文多时间框架分析指南
func getMultiTimeframeGuideEN() string {
	return `### Timeframe Hierarchy
- **15m**: Precise entry timing, find optimal entry points
- **1h**: Trend confirmation, determine short-term trend direction
- **4h**: Major trend assessment, establish primary trend direction

### Analysis Principles
1. **Higher timeframe sets direction**: Check 4h first to determine major trend (long/short/wait)
2. **Medium timeframe finds timing**: Check 1h to confirm short-term trend aligns with major trend
3. **Lower timeframe precise entry**: Check 15m last to find specific entry point

### Long/Short Assessment
- **Strong Bullish**: 4h up + 1h up + 15m up (triple timeframe alignment)
- **Bullish**: 4h up + 1h up + 15m sideways/pullback (wait for 15m to turn bullish)
- **Cautiously Bullish**: 4h up + 1h sideways + 15m up (1h may turn bullish)
- **Wait**: 4h sideways OR timeframes not aligned
- **Cautiously Bearish**: 4h down + 1h sideways + 15m down (1h may turn bearish)
- **Bearish**: 4h down + 1h down + 15m sideways/bounce (wait for 15m to turn bearish)
- **Strong Bearish**: 4h down + 1h down + 15m down (triple timeframe alignment)

### Practical Example
**Scenario**: 4h strong uptrend, 1h just broke above EMA20, 15m pulling back to EMA20
**Assessment**: Major trend up, short-term breakout confirmed, minor pullback offers entry
**Action**: Go long near 15m EMA20, stop-loss below 15m EMA20

`
}

// ========== 交易场景示例 ==========

// getTradingScenarioExamplesZH 获取中文交易场景示例
func getTradingScenarioExamplesZH() string {
	return `### 场景1: 趋势突破做多 ✅

**市场状态:**
- ETHUSDT 价格突破3200（EMA20: 3180）
- MACD金叉（当前0.0156，前值-0.0023）
- RSI: 58（健康区间，未超买）
- 机构资金流入: 1h +8.3M, 4h +25M
- 散户资金流出: 1h -2.1M
- OI增加: 1h +5.2%, 4h +12.3%
- 成交量: 当前1.8倍平均值（放量突破）
- 在OI Top榜第1名

**多时间框架:**
- 4h: 强上升趋势，价格远离EMA20
- 1h: 刚突破EMA20，MACD金叉
- 15m: 突破后回踩EMA20，形成支撑

**决策:**
` + "```json" + `
{
  "symbol": "ETHUSDT",
  "action": "open_long",
  "leverage": 5,
  "position_size_usd": 800,
  "stop_loss": 3150,
  "take_profit": 3350,
  "confidence": 85,
  "reasoning": "多信号共振：价格突破EMA20+MACD金叉+机构大额流入+散户流出+OI快速增加+放量突破+多时间框架一致，趋势突破信号强烈"
}
` + "```" + `

---

### 场景2: 假突破陷阱 ❌

**市场状态:**
- SOLUSDT 价格突破150（EMA20: 148）
- MACD金叉（0.0089）
- RSI: 78（超买警告）
- 机构资金流出: 1h -2.5M
- 散户资金流入: 1h +3.2M（散户接盘）
- OI减少: 1h -3.5%（多头平仓）
- 成交量: 当前0.6倍平均值（缩量突破）

**多时间框架:**
- 4h: 横盘震荡，无明确趋势
- 1h: 价格刚突破但MACD动能不足
- 15m: RSI严重超买

**决策:**
` + "```json" + `
{
  "symbol": "SOLUSDT",
  "action": "wait",
  "reasoning": "虽然价格突破，但RSI超买+机构资金流出+散户接盘+OI减少+缩量突破+4h无趋势，疑似假突破陷阱，等待回调或更明确信号"
}
` + "```" + `

---

### 场景3: 止盈平仓 💰

**持仓状态:**
- BTCUSDT long，入场94500，当前95800
- 盈利: +1.38% (+138 USDT)
- 峰值盈利: +1.85%（已回撤0.47%）
- 持仓时长: 1h 45m
- RSI: 72（接近超买）
- MACD开始走平，动能减弱
- 机构资金流入减弱: 15m +0.5M（前值+2.3M）
- OI增速放缓: 15m +0.8%（前值+2.1%）

**决策:**
` + "```json" + `
{
  "symbol": "BTCUSDT",
  "action": "close_long",
  "reasoning": "盈利达标+从峰值回撤25%+RSI接近超买+MACD动能减弱+机构资金流入放缓+OI增速下降，多个信号显示上涨动能衰竭，及时止盈锁定利润"
}
` + "```" + `

---

### 场景4: 止损离场 🛑

**持仓状态:**
- ETHUSDT long，入场3200，当前3140
- 亏损: -1.88% (-37.6 USDT)
- 持仓时长: 35分钟
- 价格跌破EMA20（3180）
- MACD死叉
- 机构资金突然流出: 5m -5.2M（大额卖单）
- OI快速减少: 5m -2.3%（多头止损）

**决策:**
` + "```json" + `
{
  "symbol": "ETHUSDT",
  "action": "close_long",
  "reasoning": "价格跌破EMA20+MACD死叉+机构大额流出+OI快速减少，趋势反转信号明确，及时止损避免更大损失"
}
` + "```" + `

---

### 场景5: 横盘观望 ⏸️

**市场状态:**
- BTCUSDT 价格在94000-95000区间震荡
- EMA20: 94500（价格围绕EMA20波动）
- MACD: 接近0轴，无明确方向
- RSI: 48-52之间窄幅波动
- 1h波动率: 0.8%（极低）
- OI变化: -0.3%（几乎无变化）
- 成交量: 0.5倍平均值（极度缩量）

**决策:**
` + "```json" + `
{
  "symbol": "BTCUSDT",
  "action": "wait",
  "reasoning": "横盘震荡，无明确趋势+极低波动率+缩量+OI无变化，市场处于平衡状态，等待方向选择后再入场"
}
` + "```" + `

`
}

// getTradingScenarioExamplesEN 获取英文交易场景示例
func getTradingScenarioExamplesEN() string {
	return `### Scenario 1: Trend Breakout Long ✅

**Market State:**
- ETHUSDT price breaks above 3200 (EMA20: 3180)
- MACD golden cross (current 0.0156, previous -0.0023)
- RSI: 58 (healthy zone, not overbought)
- Institutional inflow: 1h +8.3M, 4h +25M
- Retail outflow: 1h -2.1M
- OI increase: 1h +5.2%, 4h +12.3%
- Volume: 1.8× average (volume breakout)
- Ranked #1 in OI Top

**Multi-Timeframe:**
- 4h: Strong uptrend, price well above EMA20
- 1h: Just broke above EMA20, MACD golden cross
- 15m: Pullback to EMA20 after breakout, forming support

**Decision:**
` + "```json" + `
{
  "symbol": "ETHUSDT",
  "action": "open_long",
  "leverage": 5,
  "position_size_usd": 800,
  "stop_loss": 3150,
  "take_profit": 3350,
  "confidence": 85,
  "reasoning": "Multiple signal confluence: price breaks EMA20+MACD golden cross+large institutional inflow+retail outflow+rapid OI increase+volume breakout+multi-timeframe alignment, strong trend breakout signal"
}
` + "```" + `

---

### Scenario 2: False Breakout Trap ❌

**Market State:**
- SOLUSDT price breaks above 150 (EMA20: 148)
- MACD golden cross (0.0089)
- RSI: 78 (overbought warning)
- Institutional outflow: 1h -2.5M
- Retail inflow: 1h +3.2M (retail buying the top)
- OI decrease: 1h -3.5% (longs closing)
- Volume: 0.6× average (low volume breakout)

**Multi-Timeframe:**
- 4h: Sideways consolidation, no clear trend
- 1h: Price just broke out but MACD momentum weak
- 15m: RSI severely overbought

**Decision:**
` + "```json" + `
{
  "symbol": "SOLUSDT",
  "action": "wait",
  "reasoning": "Despite price breakout, RSI overbought+institutional outflow+retail buying top+OI decrease+low volume breakout+4h no trend, suspected false breakout trap, wait for pullback or clearer signal"
}
` + "```" + `

---

### Scenario 3: Take Profit Exit 💰

**Position State:**
- BTCUSDT long, entry 94500, current 95800
- Profit: +1.38% (+138 USDT)
- Peak profit: +1.85% (pulled back 0.47%)
- Holding duration: 1h 45m
- RSI: 72 (approaching overbought)
- MACD flattening, momentum weakening
- Institutional inflow weakening: 15m +0.5M (previous +2.3M)
- OI growth slowing: 15m +0.8% (previous +2.1%)

**Decision:**
` + "```json" + `
{
  "symbol": "BTCUSDT",
  "action": "close_long",
  "reasoning": "Profit target met+25% pullback from peak+RSI approaching overbought+MACD momentum weakening+institutional inflow slowing+OI growth declining, multiple signals show upward momentum exhausted, take profit to lock in gains"
}
` + "```" + `

---

### Scenario 4: Stop Loss Exit 🛑

**Position State:**
- ETHUSDT long, entry 3200, current 3140
- Loss: -1.88% (-37.6 USDT)
- Holding duration: 35 minutes
- Price broke below EMA20 (3180)
- MACD death cross
- Sudden institutional outflow: 5m -5.2M (large sell orders)
- Rapid OI decrease: 5m -2.3% (longs stopping out)

**Decision:**
` + "```json" + `
{
  "symbol": "ETHUSDT",
  "action": "close_long",
  "reasoning": "Price broke below EMA20+MACD death cross+large institutional outflow+rapid OI decrease, clear trend reversal signal, stop loss immediately to avoid larger loss"
}
` + "```" + `

---

### Scenario 5: Sideways Wait ⏸️

**Market State:**
- BTCUSDT price oscillating in 94000-95000 range
- EMA20: 94500 (price oscillating around EMA20)
- MACD: Near 0 axis, no clear direction
- RSI: Oscillating between 48-52
- 1h volatility: 0.8% (extremely low)
- OI change: -0.3% (almost no change)
- Volume: 0.5× average (extremely low volume)

**Decision:**
` + "```json" + `
{
  "symbol": "BTCUSDT",
  "action": "wait",
  "reasoning": "Sideways consolidation, no clear trend+extremely low volatility+low volume+no OI change, market in equilibrium, wait for direction breakout before entry"
}
` + "```" + `

`
}
