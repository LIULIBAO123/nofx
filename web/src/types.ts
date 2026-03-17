// 数据统计（按系统机制分类，机制下再按数据源）
export interface DataCallRecord {
  source: string
  data_type: string
  flow: string
  trader_id: string
  success: boolean
  err_msg: string
  duration_ms: number
  at: number
}
export interface MechanismDataItem {
  data_type: string
  desc: string
}
export interface MechanismSourceGroup {
  source: string
  items: MechanismDataItem[]
}
export interface CatalogByMechanism {
  mechanism: string
  sources: MechanismSourceGroup[]
}
export interface ByMechanismSource {
  mechanism: string
  source: string
  total_calls: number
  success_calls: number
  last_call_at: number
  last_success: boolean
  /** 该机制下该数据源调用的数据项（参数/接口 · 说明），与目录整合在统计表中展示 */
  items?: MechanismDataItem[]
}
export interface DataStatsResponse {
  catalog_by_mechanism: CatalogByMechanism[]
  by_mechanism_source: ByMechanismSource[]
  recent_calls: DataCallRecord[]
}

/** Latest AI token usage from backend (includes prompt cache when supported) */
export interface AIUsage {
  provider: string
  model: string
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  cache_read_input_tokens: number
  cache_creation_input_tokens: number
}

export interface SystemStatus {
  trader_id: string
  trader_name: string
  ai_model: string
  is_running: boolean
  start_time: string
  runtime_minutes: number
  call_count: number
  /** 本会话系统周期执行次数（仅用缓存+实时执行，不调 AI） */
  system_cycle_count?: number
  /** 本会话止盈止损分析周期执行次数 */
  sltp_analysis_cycle_count?: number
  /** 上次系统周期输出摘要（可查看） */
  last_system_cycle_summary?: string
  /** 上次止盈止损分析周期输出（可查看） */
  last_sltp_cycle_output?: PositionSLTPAdjustmentItem[]
  /** 系统周期输出历史（思维链形式，最近 N 条） */
  system_cycle_output_history?: SystemCycleOutputEntry[]
  /** 止盈止损周期输出历史（思维链形式，最近 N 条） */
  sltp_cycle_output_history?: SLTPCycleOutputEntry[]
  initial_balance: number
  scan_interval: string
  stop_until: string
  last_reset_time: string
  ai_provider: string
  strategy_type?: 'ai_trading' | 'grid_trading'
  grid_symbol?: string
}

export interface AccountInfo {
  total_equity: number
  wallet_balance: number
  unrealized_profit: number // 未实现盈亏（交易所API官方值）
  available_balance: number
  total_pnl: number       // 总盈亏 = 总资产 - 初始（含持仓浮盈/浮亏）
  total_pnl_pct: number
  realized_pnl?: number   // 已实现盈亏（仅平仓后的盈亏，不含持仓）
  initial_balance: number
  daily_pnl: number
  position_count: number
  margin_used: number
  margin_used_pct: number
}

export interface Position {
  symbol: string
  side: string
  entry_price: number
  mark_price: number
  quantity: number
  leverage: number
  unrealized_pnl: number
  unrealized_pnl_pct: number
  /** 价格相对入场价的变动百分比，与分层止盈触发条件一致（非保证金收益率） */
  price_change_pct?: number
  liquidation_price: number
  margin_used: number
  stop_loss?: number
  take_profit?: number
  atr_at_open?: number
  atr_multiple_sl?: number
  atr_multiple_tp?: number
  distance_to_sl_pct?: number
  distance_to_tp_pct?: number
  trailing_enabled?: boolean
  scaled_tp_enabled?: boolean
  scaled_tp_level?: number
  scaled_tp_closed_pct?: number
  trailing_tier_activated?: number
  trailing_allowed_drawdown?: number
  atr_period?: number
  support_resistance_enabled?: boolean
  support_resistance_buffer?: number
  resistance_enabled?: boolean
  resistance_buffer?: number
}

export interface DecisionAction {
  action: string
  symbol: string
  quantity: number
  leverage: number
  price: number
  stop_loss?: number      // Stop loss price
  take_profit?: number    // Take profit price
  confidence?: number     // AI confidence (0-100)
  reasoning?: string      // Brief reasoning
  order_id: number
  timestamp: string
  success: boolean
  error?: string
}

export interface AccountSnapshot {
  total_balance: number
  available_balance: number
  total_unrealized_profit: number
  position_count: number
  margin_used_pct: number
}

export interface DecisionRecord {
  timestamp: string
  cycle_number: number
  system_prompt: string
  input_prompt: string
  cot_trace: string
  decision_json: string
  account_state: AccountSnapshot
  positions: any[]
  candidate_coins: string[]
  decisions: DecisionAction[]
  execution_log: string[]
  success: boolean
  error_message?: string
}

export interface Statistics {
  total_cycles: number
  successful_cycles: number
  failed_cycles: number
  total_open_positions: number
  total_close_positions: number
}

// AI Trading相关类型
export interface TraderInfo {
  trader_id: string
  trader_name: string
  ai_model: string
  exchange_id?: string
  is_running?: boolean
  show_in_competition?: boolean
  strategy_id?: string
  strategy_name?: string
  custom_prompt?: string
  use_ai500?: boolean
  use_oi_top?: boolean
  system_prompt_template?: string
  /** 实盘模拟：虚拟资金，不发出真实订单 */
  is_simulation?: boolean
}

export interface AIModel {
  id: string
  name: string
  provider: string
  enabled: boolean
  apiKey?: string
  customApiUrl?: string
  customModelName?: string
}

export interface Exchange {
  id: string                     // UUID (empty for supported exchange templates)
  exchange_type: string          // "binance", "bybit", "okx", "hyperliquid", "aster", "lighter"
  account_name: string           // User-defined account name
  name: string                   // Display name
  type: 'cex' | 'dex'
  enabled: boolean
  apiKey?: string
  secretKey?: string
  passphrase?: string            // OKX specific
  testnet?: boolean
  // Hyperliquid specific
  hyperliquidWalletAddr?: string
  // Aster specific
  asterUser?: string
  asterSigner?: string
  asterPrivateKey?: string
  // LIGHTER specific
  lighterWalletAddr?: string
  lighterPrivateKey?: string
  lighterApiKeyPrivateKey?: string
  lighterApiKeyIndex?: number
  /** true=仅用于实盘模拟，与实盘交易分离 */
  is_simulation?: boolean
}

export interface CreateExchangeRequest {
  exchange_type: string          // "binance", "bybit", "okx", "hyperliquid", "aster", "lighter"
  account_name: string           // User-defined account name
  enabled: boolean
  /** true=仅用于实盘模拟，与实盘交易分离 */
  is_simulation?: boolean
  api_key?: string
  secret_key?: string
  passphrase?: string
  testnet?: boolean
  hyperliquid_wallet_addr?: string
  aster_user?: string
  aster_signer?: string
  aster_private_key?: string
  lighter_wallet_addr?: string
  lighter_private_key?: string
  lighter_api_key_private_key?: string
  lighter_api_key_index?: number
}

export interface CreateTraderRequest {
  name: string
  ai_model_id: string
  exchange_id: string
  strategy_id?: string // 策略ID（新版，使用保存的策略配置）
  initial_balance?: number // 可选：创建时由后端自动获取，编辑时可手动更新；实盘模拟时为必填虚拟初始资金
  scan_interval_minutes?: number
  /** 系统周期(分钟)；0=与 AI 一致，>0 且 <scan_interval 时分离：系统更频繁、AI 按 scan_interval 省 token */
  system_interval_minutes?: number
  /** 持仓止盈止损专用分析周期(分钟)；0=不启用，1~且<scan_interval 时单独跑 SL/TP 轻量分析 */
  sltp_analysis_interval_minutes?: number
  is_cross_margin?: boolean
  show_in_competition?: boolean // 是否在竞技场显示
  /** 实盘模拟：为 true 时 initial_balance 为自定义虚拟初始资金，不查询交易所 */
  is_simulation?: boolean
  // 以下字段为向后兼容保留，新版使用策略配置
  btc_eth_leverage?: number
  altcoin_leverage?: number
  trading_symbols?: string
  custom_prompt?: string
  override_base_prompt?: boolean
  system_prompt_template?: string
  use_ai500?: boolean
  use_oi_top?: boolean
}

export interface UpdateModelConfigRequest {
  models: {
    [key: string]: {
      enabled: boolean
      api_key: string
      custom_api_url?: string
      custom_model_name?: string
    }
  }
}

export interface UpdateExchangeConfigRequest {
  exchanges: {
    [key: string]: {
      enabled: boolean
      api_key: string
      secret_key: string
      passphrase?: string
      testnet?: boolean
      // Hyperliquid 特定字段
      hyperliquid_wallet_addr?: string
      // Aster 特定字段
      aster_user?: string
      aster_signer?: string
      aster_private_key?: string
      // LIGHTER 特定字段
      lighter_wallet_addr?: string
      lighter_private_key?: string
      lighter_api_key_private_key?: string
      lighter_api_key_index?: number
    }
  }
}

// Competition related types
export interface CompetitionTraderData {
  trader_id: string
  trader_name: string
  ai_model: string
  exchange: string
  total_equity: number
  total_pnl: number
  total_pnl_pct: number
  position_count: number
  margin_used_pct: number
  is_running: boolean
}

export interface CompetitionData {
  traders: CompetitionTraderData[]
  count: number
}

// Trader Configuration Data for View Modal
export interface TraderConfigData {
  trader_id?: string
  trader_name: string
  ai_model: string
  exchange_id: string
  strategy_id?: string  // 策略ID
  strategy_name?: string  // 策略名称
  is_cross_margin: boolean
  show_in_competition: boolean  // 是否在竞技场显示
  scan_interval_minutes: number
  system_interval_minutes?: number
  sltp_analysis_interval_minutes?: number
  initial_balance: number
  is_running: boolean
  // 以下为旧版字段（向后兼容）
  btc_eth_leverage?: number
  altcoin_leverage?: number
  trading_symbols?: string
  custom_prompt?: string
  override_base_prompt?: boolean
  system_prompt_template?: string
  use_ai500?: boolean
  use_oi_top?: boolean
}

// Backtest types
export interface BacktestRunSummary {
  symbol_count: number;
  decision_tf: string;
  processed_bars: number;
  progress_pct: number;
  equity_last: number;
  max_drawdown_pct: number;
  liquidated: boolean;
  liquidation_note?: string;
}

export interface BacktestRunMetadata {
  run_id: string;
  label?: string;
  user_id?: string;
  last_error?: string;
  version: number;
  state: string;
  created_at: string;
  updated_at: string;
  summary: BacktestRunSummary;
}

export interface BacktestRunsResponse {
  total: number;
  items: BacktestRunMetadata[];
}

// Position status for real-time display during backtest
export interface BacktestPositionStatus {
  symbol: string;
  side: string;
  quantity: number;
  entry_price: number;
  mark_price: number;
  leverage: number;
  unrealized_pnl: number;
  unrealized_pnl_pct: number;
  margin_used: number;
  stop_loss?: number;
  take_profit?: number;
  atr_at_open?: number;
  /** ATR as multiples (e.g. 1.6 = 1.6× ATR) for display */
  atr_multiple_sl?: number;
  atr_multiple_tp?: number;
  /** Distance from current price to SL/TP as % (positive = room before hit) */
  distance_to_sl_pct?: number;
  distance_to_tp_pct?: number;
  trailing_enabled?: boolean;
  scaled_tp_enabled?: boolean;
  scaled_tp_level?: number;
  /** Cumulative % of position closed by scaled TP (0–100) */
  scaled_tp_closed_pct?: number;
  /** Trailing stop: which tier activated by profit (0=none, 1=first…) */
  trailing_tier_activated?: number;
  /** Trailing stop: allowed drawdown % at current tier */
  trailing_allowed_drawdown?: number;
  /** ATR period used (e.g. 14) */
  atr_period?: number;
}

export interface BacktestStatusPayload {
  run_id: string;
  state: string;
  progress_pct: number;
  processed_bars: number;
  current_time: number;
  decision_cycle: number;
  equity: number;
  unrealized_pnl: number;
  realized_pnl: number;
  positions?: BacktestPositionStatus[];
  note?: string;
  last_error?: string;
  last_updated_iso: string;
}

export interface BacktestEquityPoint {
  ts: number;
  equity: number;
  available: number;
  pnl: number;
  pnl_pct: number;
  dd_pct: number;
  cycle: number;
}

export interface BacktestTradeEvent {
  ts: number;
  symbol: string;
  action: string;
  side?: string;
  qty: number;
  price: number;
  fee: number;
  slippage: number;
  order_value: number;
  realized_pnl: number;
  leverage?: number;
  cycle: number;
  position_after: number;
  liquidation: boolean;
  note?: string;
  ai_analysis?: TradeAnalysis;
  /** 开仓时间（仅平仓事件有），用于展示持仓时间 */
  open_time?: number;
  stop_loss?: number;
  take_profit?: number;
  atr_at_open?: number;
  /** 平仓原因：策略触发时为 initial_stop | trailing_stop | scaled_tp | fixed_tp 等 */
  close_reason?: string;
  /** 平仓时最终固定值（与当时持仓一致，仅 close 事件有） */
  atr_multiple_sl?: number;
  atr_multiple_tp?: number;
  atr_period?: number;
  scaled_tp_level?: number;
  scaled_tp_closed_pct?: number;
  trailing_tier_activated?: number;
  trailing_allowed_drawdown?: number;
}

export interface TradeAnalysis {
  trade_id: number;
  rating: 'excellent' | 'good' | 'fair' | 'poor';
  summary: string;
  profit_analysis: string;
  improvements: string[];
  risk_warnings: string[];
  analyzed_at: string;
}

export interface BacktestMetrics {
  total_return_pct: number;
  max_drawdown_pct: number;
  sharpe_ratio: number;
  profit_factor: number;
  win_rate: number;
  trades: number;
  avg_win: number;
  avg_loss: number;
  best_symbol: string;
  worst_symbol: string;
  liquidated: boolean;
  symbol_stats?: Record<
    string,
    {
      total_trades: number;
      winning_trades: number;
      losing_trades: number;
      total_pnl: number;
      avg_pnl: number;
      win_rate: number;
    }
  >;
}

export interface BacktestStartConfig {
  run_id?: string;
  ai_model_id?: string;
  strategy_id?: string; // Optional: use saved strategy from Strategy Studio
  symbols: string[];
  timeframes: string[];
  decision_timeframe: string;
  decision_cadence_nbars: number;
  /** 与实盘一致：每 N 分钟一次决策（0=按 K 线节奏） */
  decision_interval_minutes?: number;
  start_ts: number;
  end_ts: number;
  initial_balance: number;
  fee_bps: number;
  slippage_bps: number;
  fill_policy: string;
  prompt_variant?: string;
  prompt_template?: string;
  custom_prompt?: string;
  override_prompt?: boolean;
  cache_ai?: boolean;
  replay_only?: boolean;
  checkpoint_interval_bars?: number;
  checkpoint_interval_seconds?: number;
  replay_decision_dir?: string;
  shared_ai_cache_path?: string;
  ai?: {
    provider?: string;
    model?: string;
    key?: string;
    secret_key?: string;
    base_url?: string;
  };
  leverage?: {
    btc_eth_leverage?: number;
    altcoin_leverage?: number;
  };
}

// Kline data for backtest chart
export interface BacktestKline {
  time: number;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
}

export interface BacktestKlinesResponse {
  symbol: string;
  timeframe: string;
  start_ts: number;
  end_ts: number;
  count: number;
  klines: BacktestKline[];
  run_id: string;
}

// Strategy Studio Types
export interface Strategy {
  id: string;
  name: string;
  description: string;
  is_active: boolean;
  is_default: boolean;
  is_public: boolean;           // 是否在策略市场公开
  config_visible: boolean;      // 配置参数是否公开可见
  config: StrategyConfig;
  created_at: string;
  updated_at: string;
}

// 策略使用统计
export interface StrategyStats {
  clone_count: number;          // 被克隆次数
  active_users: number;         // 当前使用人数
  top_performers?: StrategyPerformer[];  // 收益排行
}

// 策略使用者收益排行
export interface StrategyPerformer {
  user_id: string;
  user_name: string;            // 脱敏后的用户名
  total_pnl_pct: number;        // 总收益率
  total_pnl: number;            // 总收益金额
  win_rate: number;             // 胜率
  trade_count: number;          // 交易次数
  using_since: string;          // 使用开始时间
  rank: number;                 // 排名
}

export interface PromptSectionsConfig {
  role_definition?: string;
  trading_frequency?: string;
  entry_standards?: string;
  decision_process?: string;
}

export interface StrategyConfig {
  // Strategy type: "ai_trading" (default) or "grid_trading"
  strategy_type?: 'ai_trading' | 'grid_trading';
  // Strategy mode: "classic" | "multilayer_filter"
  strategy_mode?: 'classic' | 'multilayer_filter';
  // Language setting: "zh" for Chinese, "en" for English
  language?: 'zh' | 'en';
  coin_source: CoinSourceConfig;
  indicators: IndicatorConfig;
  custom_prompt?: string;
  risk_control: RiskControlConfig;
  prompt_sections?: PromptSectionsConfig;
  // Grid trading configuration (only used when strategy_type is 'grid_trading')
  grid_config?: GridStrategyConfig;
  // 多层过滤与方向池（挂单流程）
  multilayer_filter?: MultilayerFilterConfig;
}

export interface MultilayerFilterConfig {
  enabled?: boolean;
  direction_pool?: DirectionPoolConfig;
  layer1?: Layer1Config;
  layer2?: Layer2Config;
  layer3?: Layer3Config;
}

/** 方向池可选配置 */
export interface DirectionPoolConfig {
  /** 仅将 StrengthPct >= 此值的标的放入方向池；0 表示不过滤 */
  min_strength_pct?: number;
  /** 每标的只归入多空池中强度更高的一侧 */
  single_side_only?: boolean;
  /** 多/空池按 StrengthPct 降序排列，开仓与展示优先高强度 */
  sort_by_strength?: boolean;
  /** 仅 Layer2 达标的标的进入方向池 */
  filter_pool_by_layer2?: boolean;
  /** 对 StrengthPct 做短时平滑，减少抖动 */
  enable_strength_smoothing?: boolean;
  /** 平滑权重：smoothed = weight*last + (1-weight)*current，建议 0.6~0.8 */
  strength_smoothing_weight?: number;
  /** 补强加成对 StrengthPct 的上限（0 表示不设限） */
  strength_bonus_cap?: number;
  /** 补强加成对 ReliabilityPct 的上限（0 表示不设限） */
  reli_bonus_cap?: number;
  /** 开仓最低强度：三条件共振后仅当方向池中该标的 StrengthPct ≥ 此值才允许开仓；0 表示不额外过滤 */
  min_strength_pct_to_open?: number;
}

export interface Layer1Config {
  required_all?: boolean;
  /** 0=全部通过才过；>0 时至少通过 N 项即过 Layer1（放宽过滤） */
  min_items_to_pass?: number;
  /** 多周期至少一致数：2=三周期(4h/1h/短)至少两周期同向；0=不启用 */
  min_periods_aligned?: number;
  items?: Layer1Item[];
  max_signal_age_minutes?: number;
  /** 依赖数据缺失时视为不通过（默认 false 为原行为） */
  fail_closed_when_data_missing?: boolean;
}

export interface Layer1Item {
  id: string;
  enabled: boolean;
  allowed?: string[];
  value?: number;
}

export interface Layer2Config {
  min_factors?: number;
  reliability_threshold?: number;
  entry_confidence_threshold_pct?: number;
}

export interface Layer3Config {
  max_signal_age_minutes?: number;
  oi_aligned_required?: boolean;
  entry_timing_strength_min?: number;
  custom_factors_required?: boolean;
}

// Grid trading specific configuration
export interface GridStrategyConfig {
  // Trading pair (e.g., "BTCUSDT")
  symbol: string;
  // Number of grid levels (5-50)
  grid_count: number;
  // Total investment in USDT
  total_investment: number;
  // Leverage (1-20)
  leverage: number;
  // Upper price boundary (0 = auto-calculate from ATR)
  upper_price: number;
  // Lower price boundary (0 = auto-calculate from ATR)
  lower_price: number;
  // Use ATR to auto-calculate bounds
  use_atr_bounds: boolean;
  // ATR multiplier for bound calculation (default 2.0)
  atr_multiplier: number;
  // Position distribution: "uniform" | "gaussian" | "pyramid"
  distribution: 'uniform' | 'gaussian' | 'pyramid';
  // Maximum drawdown percentage before emergency exit
  max_drawdown_pct: number;
  // Stop loss percentage per position
  stop_loss_pct: number;
  // Daily loss limit percentage
  daily_loss_limit_pct: number;
  // Use maker-only orders for lower fees
  use_maker_only: boolean;
  // Enable automatic grid direction adjustment based on box breakouts
  enable_direction_adjust?: boolean;
  // Direction bias ratio for long_bias/short_bias modes (default 0.7 = 70%/30%)
  direction_bias_ratio?: number;
}

export interface CoinSourceConfig {
  source_type: 'static' | 'ai500' | 'oi_top' | 'oi_low' | 'mixed';
  static_coins?: string[];
  excluded_coins?: string[];   // 排除的币种列表
  use_ai500: boolean;
  ai500_limit?: number;
  use_oi_top: boolean;
  oi_top_limit?: number;
  use_oi_low: boolean;
  oi_low_limit?: number;
  // Note: API URLs are now built automatically using nofxos_api_key from IndicatorConfig
}

export interface IndicatorConfig {
  klines: KlineConfig;
  // Raw OHLCV kline data - required for AI analysis
  enable_raw_klines: boolean;
  // Technical indicators (optional)
  enable_ema: boolean;
  enable_macd: boolean;
  enable_rsi: boolean;
  enable_atr: boolean;
  enable_boll: boolean;
  enable_volume: boolean;
  enable_oi: boolean;
  enable_funding_rate: boolean;
  ema_periods?: number[];
  rsi_periods?: number[];
  atr_periods?: number[];
  boll_periods?: number[];
  external_data_sources?: ExternalDataSource[];

  // ========== NofxOS 数据源统一配置 ==========
  // Unified NofxOS API Key - used for all NofxOS data sources
  nofxos_api_key?: string;

  // 量化数据源（资金流向、持仓变化、价格变化）
  enable_quant_data?: boolean;
  enable_quant_oi?: boolean;
  enable_quant_netflow?: boolean;

  // OI 排行数据（市场持仓量增减排行）
  enable_oi_ranking?: boolean;
  oi_ranking_duration?: string;  // "1h", "4h", "24h"
  oi_ranking_limit?: number;

  // NetFlow 排行数据（机构/散户资金流向排行）
  enable_netflow_ranking?: boolean;
  netflow_ranking_duration?: string;  // "1h", "4h", "24h"
  netflow_ranking_limit?: number;

  // Price 排行数据（涨跌幅排行）
  enable_price_ranking?: boolean;
  price_ranking_duration?: string;  // "1h", "4h", "24h" or "1h,4h,24h"
  price_ranking_limit?: number;

  // 币安衍生数据（多空比、资金费率、Taker）— 以币安为主增强市场判断，无需 API Key
  enable_binance_long_short_ratio?: boolean;
  binance_long_short_period?: string;   // 5m, 15m, 1h, 4h
  enable_binance_funding_history?: boolean;
  enable_binance_taker_volume?: boolean;
  binance_taker_period?: string;        // 5m, 15m, 1h

  // 数据补强：资金费率 8h 均值、Basis、BTC 占比、强平聚合、CoinAnk 清算
  enable_binance_funding_rate_history?: boolean;
  enable_basis?: boolean;
  enable_btc_dominance?: boolean;
  enable_binance_ws_force_order?: boolean;
  enable_coinank_liquidation?: boolean;
  coinank_api_key?: string;
  coinank_url?: string;

  /** Coinglass 中转站（KeyStore）：通过代理获取 OI/资金费率/强平等市场数据 */
  enable_coinglass_data?: boolean;
  coinglass_proxy_url?: string;
  coinglass_api_key?: string;
  /** 启用 Coinglass WSS 实时推送（融资率/清算/OI/价格），补强 AI 实时与预测 */
  enable_coinglass_wss?: boolean;

  /** 非主周期在 Prompt 中仅输出一行摘要（Close/EMA20/EMA50/ATR14），可显著减少 Token */
  compact_non_primary_timeframe?: boolean;
}

export interface KlineConfig {
  primary_timeframe: string;
  primary_count: number;
  longer_timeframe?: string;
  longer_count?: number;
  enable_multi_timeframe: boolean;
  // 新增：支持选择多个时间周期
  selected_timeframes?: string[];
  /** 写入 Prompt 的候选币数上限（不含持仓），0=默认8，减小可省 Token */
  max_coins_in_prompt?: number;
}

export interface ExternalDataSource {
  name: string;
  type: 'api' | 'webhook';
  url: string;
  method: string;
  headers?: Record<string, string>;
  data_path?: string;
  refresh_secs?: number;
}

export interface RiskControlConfig {
  // Max number of coins held simultaneously (CODE ENFORCED)
  max_positions: number;

  // Trading Leverage - exchange leverage for opening positions (AI guided)
  btc_eth_max_leverage: number;    // BTC/ETH max exchange leverage
  altcoin_max_leverage: number;    // Altcoin max exchange leverage

  // Position Value Ratio - single position notional value / account equity (CODE ENFORCED)
  // Max position value = equity × this ratio
  btc_eth_max_position_value_ratio?: number;     // default: 5 (BTC/ETH max position = 5x equity)
  altcoin_max_position_value_ratio?: number;     // default: 1 (Altcoin max position = 1x equity)

  // Risk Parameters
  max_margin_usage: number;        // Max margin utilization, e.g. 0.9 = 90% (CODE ENFORCED)
  min_position_size: number;       // Min position size in USDT (CODE ENFORCED)
  min_risk_reward_ratio: number;   // Min take_profit / stop_loss ratio (AI guided)
  min_confidence: number;          // Min AI confidence to open position (AI guided)

  // AI 仅开仓：true 时不执行 AI 的平仓建议，平仓完全由策略动态 SL/TP 执行（适应震荡市）
  ai_only_entry?: boolean;

  // 系统执行开仓：true 时 AI 仅作辅助分析量化数据，不输出开仓动作；开仓由系统根据多层过滤/方向池执行（需启用 multilayer_filter）
  system_executes_entry?: boolean;

  /** 允许执行 AI 的平仓建议（需 AIOnlyEntry=false）；false 时仅由动态 SL/TP 平仓 */
  /** AI 仅预测：为 true 时禁止 AI 输出开平仓，只输出预测信息，系统根据预测+pipeline 执行 */
  ai_predict_only?: boolean;
  allow_ai_close?: boolean;
  /** AI 平仓时最低置信度；0=不额外要求 */
  min_confidence_for_ai_close?: number;
  /** 为 true 时仅当 exit_reason 为 take_profit|stop_loss|prediction_mismatch 才执行 AI 平仓 */
  require_exit_reason_for_ai_close?: boolean;

  /** 额外补强：按 market_regime 提高开仓置信度要求（ranging/high_volatility/reversal） */
  regime_adjust_enabled?: boolean;
  regime_min_confidence_map?: Record<string, number>;  // 如 { "ranging": 75, "high_volatility": 78, "reversal": 80 }

  /** 额外补强：极端资金费率/多空比时的开仓约束 */
  extreme_funding_rule?: ExtremeFundingRule;

  // Dynamic Stop Loss & Take Profit
  dynamic_stop_loss?: DynamicStopLossConfig;
  dynamic_take_profit?: DynamicTakeProfitConfig;

  /** AI 参与仓位与分层止盈止损：离散档位与模板（系统兜底裁剪） */
  position_size_buckets?: PositionSizeBucketsConfig;
  tp_profiles?: Record<string, DynamicTakeProfitConfig>;
  sl_profiles?: Record<string, DynamicStopLossConfig>;
}

export interface PositionSizeBucketsConfig {
  enabled: boolean;
  /** fallback bucket when AI missing/invalid */
  default_bucket?: string; // low/medium/high
  /** when AI confidence < this, force default bucket */
  min_bucket_confidence?: number;
  /** equity ratio map, e.g. { low:0.003, medium:0.007, high:0.012 } */
  buckets?: Record<string, number>;
  /** optional cap bucket, e.g. medium */
  max_bucket?: string;
}

/** 极端资金费率与多空比时的开仓约束（补强） */
export interface ExtremeFundingRule {
  enabled: boolean;
  funding_threshold_pct?: number;   // 资金费率绝对值阈值，如 0.001 = 0.1%
  long_short_ratio_high?: number;  // 多空比 > 此值视为多头过热，如 1.4
  long_short_ratio_low?: number;   // 多空比 < 此值视为空头过热，如 0.714
  block_open_long_when_excessive_longs?: boolean;
  block_open_short_when_excessive_shorts?: boolean;
  raise_confidence_by?: number;    // 不禁止时提高的置信度，如 10
  min_confidence_when_extreme?: number;  // 极端时最低置信度，如 80
}

// 动态止损配置
export interface DynamicStopLossConfig {
  enabled: boolean;                // 是否启用动态止损
  trigger_logic: 'any' | 'all';    // 触发逻辑：'any' = 任一条件触发即平仓，'all' = 所有启用的条件都触发才平仓
  
  // 最小持仓时间（分钟），未满不触发动态止损，避免开仓即止损。0=不限制
  min_hold_minutes?: number;
  
  // 初始固定止损（必需，作为保底）
  initial_stop_percent: number;    // 初始固定止损百分比 (例如: 3 = 3%)
  
  // 追踪止损 (Trailing Stop) - 分层模式
  trailing_enabled?: boolean;      // 是否启用追踪止损
  trailing_levels?: TrailingStopLevel[];  // 追踪止损层级
  /** 为 true 时仅当已触发过至少一档分层止盈后才启用追踪止损，避免尚未止盈就被追踪平仓 */
  trailing_stop_only_after_first_scaled_tp?: boolean;
  
  // ATR 止损 - 动态区间模式
  atr_enabled?: boolean;           // 是否启用 ATR 止损
  atr_multiplier_min?: number;     // ATR 倍数最小值 (AI 可选范围下限)
  atr_multiplier_max?: number;     // ATR 倍数最大值 (AI 可选范围上限)
  atr_period_btc_eth?: number;     // BTC/ETH 的 ATR 周期
  atr_period_altcoin?: number;     // 山寨币的 ATR 周期
  
  // 支撑阻力止损
  support_resistance_enabled?: boolean;  // 是否启用支撑阻力止损
  support_resistance_buffer?: number;    // 支撑/阻力位缓冲百分比 (例如: 0.5 = 0.5%)

  // 连续确认再止损：减少单K线假跌破
  confirm_cycles?: number;               // 连续 N 周期满足条件才执行止损，1=立即，2+=延迟确认
  confirm_minutes?: number;              // >0 时按真实时间确认：条件需持续满足此分钟数；0=用 confirm_cycles

  // 高波动宽容：ATR 高时更宽容
  atr_tolerance_enabled?: boolean;      // 高波动时多要求 1 个确认周期 / 放宽 ATR 止损
  atr_high_multiplier?: number;         // 当前 ATR > 长期 ATR * 此倍数视为高波动，默认 1.2

  /** scenario=reversal 时是否再减 1 个 SL 确认周期（加快真反转止损） */
  scenario_adjust_enabled?: boolean;

  // 止损用 K 线周期（ATR、支撑阻力、逆势早退等）"15m" | "1h"，默认 15m
  klines_timeframe?: string;
  // 支撑/阻力用 EMA20 作为结构位（多=支撑，空=阻力）
  support_resistance_use_ema20?: boolean;

  // 从未浮盈+反向过大早退：从未出现过浮盈且价格反向移动≥此倍数×ATR 时提前止损（0=关闭）
  adverse_exit_when_never_profit_atr?: number;
  // 山寨币单独倍数（非 BTC/ETH 使用）
  adverse_exit_when_never_profit_atr_altcoin?: number;
  // 逆势早退需 ATR 骤升（当前 ATR ≥ 长期 ATR×阈值）才触发，避免温和震荡早退
  adverse_exit_require_atr_spike?: boolean;
  adverse_exit_atr_spike_threshold?: number;  // 默认 1.2
}

// 追踪止损层级
export interface TrailingStopLevel {
  profit_threshold: number;        // 盈利阈值 (例如: 3 = 盈利3%时激活此层级)
  trailing_percent: number;        // 该层级的追踪止损百分比 (例如: 2 = 允许2%回撤)
}

// 动态止盈配置
export interface DynamicTakeProfitConfig {
  enabled: boolean;                // 是否启用动态止盈
  
  // 最小持仓时间（分钟），未满不触发动态止盈，避免开仓即止盈。0=不限制
  min_hold_minutes?: number;
  // 止盈侧最低盈利过滤（价格%）。当前浮盈低于此值时不触发任何止盈；0=不限制
  min_profit_percent_to_allow_tp?: number;

  // 固定止盈
  fixed_enabled?: boolean;         // 是否启用固定止盈
  fixed_percent?: number;          // 固定止盈百分比 (例如: 8 = 8%)
  
  // 分批止盈 (Scaled Take Profit)
  scaled_enabled?: boolean;        // 是否启用分批止盈
  scaled_levels?: ScaledTakeProfitLevel[];  // 分批止盈层级
  
  // ATR 止盈 - 动态区间模式
  atr_enabled?: boolean;           // 是否启用 ATR 止盈
  atr_multiplier_min?: number;     // ATR 倍数最小值 (AI 可选范围下限)
  atr_multiplier_max?: number;     // ATR 倍数最大值 (AI 可选范围上限)
  atr_use_max_in_high_volatility?: boolean;  // 高波动时用 Max 倍数
  atr_high_volatility_threshold?: number;   // 默认 1.2
  atr_period_btc_eth?: number;     // BTC/ETH 的 ATR 周期
  atr_period_altcoin?: number;     // 山寨币的 ATR 周期

  // 阻力位止盈
  resistance_enabled?: boolean;    // 是否启用阻力位止盈
  resistance_buffer?: number;      // 阻力位缓冲百分比 (例如: 0.5 = 0.5%)
  
  // 回撤止盈：有盈利后从峰值回撤一定幅度即止盈，用 ATR/确认防震荡
  trailing_tp_enabled?: boolean;
  trailing_tp_activate_profit_pct?: number;  // 至少达到此盈利%才考虑回撤止盈 (如 2)
  trailing_tp_retrace_pct?: number;         // 从峰值回撤超过此%即满足 (如 1.5)
  trailing_tp_retrace_atr_mult?: number;     // 回撤需≥此倍数×ATR/价格，取 max(固定%, ATR%) 防震荡 (如 0.5)
  trailing_tp_confirm_minutes?: number;      // 0=不确认；>0=条件持续 N 分钟再触发
  trailing_tp_close_percent?: number;       // 触发时平仓比例 (50 或 100)
  trailing_tp_activate_profit_pct_altcoin?: number; // 山寨币激活阈值%（非 BTC/ETH）
  trailing_tp_retrace_pct_altcoin?: number;         // 山寨币回撤%（非 BTC/ETH）

  // 通用设置
  lock_profit_percent?: number;    // 锁定利润百分比 (达到后移动止损到盈亏平衡点)
}

// 分批止盈层级
export interface ScaledTakeProfitLevel {
  profit_percent: number;          // 盈利百分比触发点 (例如: 3 = 3%)
  close_percent: number;           // 平仓百分比 (例如: 30 = 平仓30%)
  move_stop_to_breakeven?: boolean; // 是否移动止损到盈亏平衡点
}

// 单笔平仓记录（部分平仓或全平），与后端 CloseEvent 一致
export interface CloseEventItem {
  exit_time_ms: number;
  closed_qty: number;
  exit_price: number;
  realized_pnl: number;
  close_reason?: string;
  is_partial: boolean;
  profit_percent?: number;
}

// Position History Types
export interface HistoricalPosition {
  id: number;
  trader_id: string;
  exchange_id: string;
  exchange_type: string;
  symbol: string;
  side: string;
  quantity: number;
  entry_quantity: number;
  entry_price: number;
  entry_order_id: string;
  entry_time: string;
  exit_price: number;
  exit_order_id: string;
  exit_time: string;
  realized_pnl: number;
  fee: number;
  leverage: number;
  status: string;
  close_reason: string;
  /** 平仓是否由触发了 AI 调节后的止盈/止损参数导致 */
  close_reason_ai_adjusted?: boolean;
  /** 触发时的详情 JSON：trigger, type, trigger_price, trail_aggressiveness, atr_mult_sl, lock_profit_pct, advice 等 */
  close_reason_trigger_detail?: string;
  /** 平仓明细（分层止盈等）：JSON 字符串，解析为 CloseEventItem[] 展示 */
  close_events?: string;
  /** 当 close_reason 为空/sync/unknown 时，由同期决策记录推断的平仓原因（系统止盈/系统止损/系统平仓） */
  inferred_close_reason?: string;
  /** 推断依据：同期决策的 Reasoning 文案 */
  inferred_close_reason_detail?: string;
  // Fixed params at open (same as backtest trade history)
  stop_loss?: number;
  take_profit?: number;
  atr_at_open?: number;
  atr_multiple_sl?: number;
  atr_multiple_tp?: number;
  atr_period?: number;
  created_at: string;
  updated_at: string;
}

// Matches Go TraderStats struct exactly
export interface TraderStats {
  total_trades: number;
  win_trades: number;
  loss_trades: number;
  win_rate: number;
  profit_factor: number;
  sharpe_ratio: number;
  total_pnl: number;
  total_fee: number;
  avg_win: number;
  avg_loss: number;
  max_drawdown_pct: number;
}

// Matches Go SymbolStats struct exactly
export interface SymbolStats {
  symbol: string;
  total_trades: number;
  win_trades: number;
  win_rate: number;
  total_pnl: number;
  avg_pnl: number;
  avg_hold_mins: number;
}

// Matches Go DirectionStats struct exactly
export interface DirectionStats {
  side: string;
  trade_count: number;
  win_rate: number;
  total_pnl: number;
  avg_pnl: number;
}

export interface PositionHistoryResponse {
  positions: HistoricalPosition[];
  stats: TraderStats | null;
  symbol_stats: SymbolStats[];
  direction_stats: DirectionStats[];
}

// Grid Risk Information for frontend display
export interface GridRiskInfo {
  // Leverage info
  current_leverage: number
  effective_leverage: number
  recommended_leverage: number

  // Position info
  current_position: number
  max_position: number
  position_percent: number

  // Liquidation info
  liquidation_price: number
  liquidation_distance: number

  // Market state
  regime_level: string

  // Box state
  short_box_upper: number
  short_box_lower: number
  mid_box_upper: number
  mid_box_lower: number
  long_box_upper: number
  long_box_lower: number
  current_price: number

  // Breakout state
  breakout_level: string
  breakout_direction: string
}

// --- 多空雷达与挂单流程（与用户分享截图功能对应）---

export interface RadarConfig {
  allow_long: boolean
  allow_short: boolean
  ai_auto_analysis: boolean
  mode: 'preset' | 'manual' | 'close_auto'
  heat_score: number
  atr_pct: number
  capital_critical_pct: number
  reliability_gate_pct: number
  direction_quantile: number
  direction_query: number
  exclude_held: boolean
  exclude_pending: boolean
  pending_order_cap_pct: number
  long_pool_auto_issue: boolean
  long_pool_auto_cancel: boolean
  short_pool_auto_issue: boolean
  short_pool_auto_cancel: boolean
  pool_quantile_pct: number
}

export interface OrderFlowPipeline {
  pool_long: number
  pool_short: number
  after_layer1: number
  after_layer2: number
  to_submit: number
  flow_label: string
}

export interface Layer1FailureStat {
  condition: string
  count: number
}

export interface PerCoinFailure {
  symbol: string
  reasons: string[]
}

export interface OrderFlowInfo {
  updated_at: string
  process_stage: string
  pipeline: OrderFlowPipeline
  layer1_failure_stats: Layer1FailureStat[]
  per_coin_failures: PerCoinFailure[]
  pipeline_description: string
}

export interface DirectionPoolItem {
  symbol: string
  strength_pct: number
  score: number
  market_condition: string
  reliability_pct: number
  timing: string
  volume_price_pct: number
  from_ai?: boolean
}

export interface DirectionPoolResponse {
  long: DirectionPoolItem[]
  short: DirectionPoolItem[]
}

/** 最新周期 AI 分析快照（雷达页「实时数据+AI预测」展示） */
/** AI 对单笔持仓的止盈/止损参数调节建议（展示用）；含结构化退场信号 */
export interface PositionSLTPAdjustmentItem {
  symbol: string
  side: string
  advice?: string
  trail_aggressiveness?: string
  atr_mult_sl?: number
  lock_profit_pct?: number
  confirm_cycles_delta?: number
  /** 阶段标签（如 trend_exhaustion） */
  phase_label?: string
  /** 证伪价位/级别 */
  invalidation_level?: string
  /** 证伪强度 0–100 */
  invalidation_strength?: number
  /** 退场倾向：tighten / scale_out / exit */
  exit_bias?: string
  /** 简要理由 */
  rationale?: string
}

/** 单条系统周期输出（思维链一条） */
export interface SystemCycleOutputEntry {
  at: string
  cycle_number: number
  summary: string
}

/** 单条止盈止损周期输出（思维链一条） */
export interface SLTPCycleOutputEntry {
  at: string
  scheduled_at?: string
  cycle_number: number
  adjustments: PositionSLTPAdjustmentItem[]
  raw_output?: string
}

/** 止盈止损调节前策略基线（用于展示「原始→调整后」） */
export interface PositionSLTPBaselineItem {
  symbol: string
  side: string
  trail_aggressiveness?: string
  atr_mult_sl?: number
  lock_profit_pct?: number
  confirm_cycles?: number
}

export interface LatestAnalysisResponse {
  market_regime?: string
  scenario?: string
  market_summary?: string
  risk_alert?: boolean
  symbol_predictions?: SymbolPredictionItem[]
  position_sl_tp_adjustments?: PositionSLTPAdjustmentItem[]
  position_sl_tp_baselines?: PositionSLTPBaselineItem[]
  updated_at?: string
}

export interface SymbolPredictionItem {
  symbol: string
  predicted_direction?: string
  confidence?: number
  suggest_exit?: boolean
  /** 是否建议本周期开仓；与实时方向、AI预测方向三条件共振才开仓 */
  suggest_open?: boolean
}
