import { Shield, AlertTriangle, TrendingDown, TrendingUp } from 'lucide-react'
import type {
  DynamicStopLossConfig,
  DynamicTakeProfitConfig,
  PositionSizeBucketsConfig,
  RiskControlConfig,
  ScaledTakeProfitLevel,
  TrailingStopLevel,
} from '../../types'
import { DynamicStopLossEditor } from './DynamicStopLossEditor'
import { DynamicTakeProfitEditor } from './DynamicTakeProfitEditor'

interface RiskControlEditorProps {
  config: RiskControlConfig
  onChange: (config: RiskControlConfig) => void
  disabled?: boolean
  language: string
}

export function RiskControlEditor({
  config,
  onChange,
  disabled,
  language,
}: RiskControlEditorProps) {
  const t = (key: string) => {
    const translations: Record<string, Record<string, string>> = {
      positionLimits: { zh: '仓位限制', en: 'Position Limits' },
      maxPositions: { zh: '最大持仓数量', en: 'Max Positions' },
      maxPositionsDesc: { zh: '同时持有的最大币种数量', en: 'Maximum coins held simultaneously' },
      // Trading leverage (exchange leverage)
      tradingLeverage: { zh: '交易杠杆（交易所杠杆）', en: 'Trading Leverage (Exchange)' },
      btcEthLeverage: { zh: 'BTC/ETH 交易杠杆', en: 'BTC/ETH Trading Leverage' },
      btcEthLeverageDesc: { zh: '交易所开仓使用的杠杆倍数', en: 'Exchange leverage for opening positions' },
      altcoinLeverage: { zh: '山寨币交易杠杆', en: 'Altcoin Trading Leverage' },
      altcoinLeverageDesc: { zh: '交易所开仓使用的杠杆倍数', en: 'Exchange leverage for opening positions' },
      // Position value ratio (risk control) - CODE ENFORCED
      positionValueRatio: { zh: '仓位价值比例（代码强制）', en: 'Position Value Ratio (CODE ENFORCED)' },
      positionValueRatioDesc: { zh: '单仓位名义价值 / 账户净值，由代码强制执行', en: 'Position notional value / equity, enforced by code' },
      btcEthPositionValueRatio: { zh: 'BTC/ETH 仓位价值比例', en: 'BTC/ETH Position Value Ratio' },
      btcEthPositionValueRatioDesc: { zh: '单仓最大名义价值 = 净值 × 此值（代码强制）', en: 'Max position value = equity × this ratio (CODE ENFORCED)' },
      altcoinPositionValueRatio: { zh: '山寨币仓位价值比例', en: 'Altcoin Position Value Ratio' },
      altcoinPositionValueRatioDesc: { zh: '单仓最大名义价值 = 净值 × 此值（代码强制）', en: 'Max position value = equity × this ratio (CODE ENFORCED)' },
      riskParameters: { zh: '风险参数', en: 'Risk Parameters' },
      minRiskReward: { zh: '最小风险回报比', en: 'Min Risk/Reward Ratio' },
      minRiskRewardDesc: { zh: '开仓要求的最低盈亏比', en: 'Minimum profit ratio for opening' },
      maxMarginUsage: { zh: '最大保证金使用率（代码强制）', en: 'Max Margin Usage (CODE ENFORCED)' },
      maxMarginUsageDesc: { zh: '保证金使用率上限，由代码强制执行', en: 'Maximum margin utilization, enforced by code' },
      entryRequirements: { zh: '开仓要求', en: 'Entry Requirements' },
      minPositionSize: { zh: '最小开仓金额', en: 'Min Position Size' },
      minPositionSizeDesc: { zh: 'USDT 最小名义价值', en: 'Minimum notional value in USDT' },
      minConfidence: { zh: '最小信心度', en: 'Min Confidence' },
      minConfidenceDesc: { zh: 'AI 开仓信心度阈值', en: 'AI confidence threshold for entry' },
      regimeAdjust: { zh: '按市场状态提高开仓置信度', en: 'Raise min confidence by market regime' },
      regimeAdjustDesc: { zh: '开启后：当 AI 判定为震荡(ranging)/高波(high_volatility)/反转(reversal) 时，开仓要求的最低置信度将提高为下方设定值，减少在不利市况下开仓。', en: 'When on: in ranging/high_volatility/reversal, required min confidence is raised to values below.' },
      regimeRanging: { zh: '震荡 (ranging)', en: 'Ranging' },
      regimeHighVol: { zh: '高波动 (high_volatility)', en: 'High volatility' },
      regimeReversal: { zh: '反转 (reversal)', en: 'Reversal' },
      aiOnlyEntry: { zh: 'AI 仅开仓', en: 'AI Only Entry' },
      aiOnlyEntryDesc: { zh: '开启后：AI 只负责预测与开仓；平仓完全由策略（动态止损/追踪/分层止盈）执行，不执行 AI 的 close 建议。适合震荡市拿住仓、盈利后平仓。', en: 'When on: AI only predicts and opens; strategy handles all exits (dynamic SL/TP, trailing, scaled TP). AI close suggestions are ignored. Suited for ranging markets.' },
      systemExecutesEntry: { zh: '系统执行开仓', en: 'System Executes Entry' },
      systemExecutesEntryDesc: { zh: '开启后：AI 仅作辅助，分析量化数据并输出 trend_view；开仓动作由系统根据多层过滤与方向池执行。需同时启用「多层过滤」策略模式。', en: 'When on: AI only assists (analyzes data, trend_view); system executes opens from pipeline. Requires multilayer_filter strategy mode.' },
      aiPredictOnly: { zh: 'AI 仅预测（系统决策）', en: 'AI Predict Only (System Decides)' },
      aiPredictOnlyDesc: { zh: '开启后：禁止 AI 输出开平仓，AI 只输出预测（market_regime、scenario、symbol_predictions 等）；开平仓完全由系统根据预测 + 多层过滤/方向池/止盈止损 执行。需在 <analysis> 中输出 symbol_predictions。', en: 'When on: AI must not output open/close; only prediction. System decides all entry/exit from prediction + pipeline + TP/SL. AI must output symbol_predictions in <analysis>.' },
      allowAiClose: { zh: '允许 AI 平仓/止盈止损', en: 'Allow AI Close / TP-SL' },
      allowAiCloseDesc: { zh: '开启后：AI 可建议 close_long/close_short；仅当结合历史+实时+预测认为交易与预测不符时，且满足置信度与 exit_reason 约束时才执行。与系统动态止盈止损并存。', en: 'When on: AI can suggest close; only executed when prediction mismatch + confidence and exit_reason constraints met. Coexists with system TP/SL.' },
      minConfidenceForAiClose: { zh: 'AI 平仓最低置信度', en: 'Min confidence for AI close' },
      minConfidenceForAiCloseDesc: { zh: 'AI 建议平仓时仅当 confidence ≥ 此值才执行；0 表示不额外要求', en: 'Execute AI close only when confidence ≥ this; 0 = no extra requirement' },
      requireExitReasonForAiClose: { zh: '要求填写退出原因', en: 'Require exit_reason' },
      requireExitReasonForAiCloseDesc: { zh: '仅当 exit_reason 为 take_profit | stop_loss | prediction_mismatch 之一时才执行 AI 平仓', en: 'Execute AI close only when exit_reason is take_profit | stop_loss | prediction_mismatch' },
      dynamicStopLoss: { zh: '动态止损', en: 'Dynamic Stop Loss' },
      dynamicTakeProfit: { zh: '动态止盈', en: 'Dynamic Take Profit' },
      aiSizingAndProfiles: { zh: 'AI 仓位档位与止盈止损模板', en: 'AI Sizing Buckets & SL/TP Profiles' },
      aiSizingAndProfilesDesc: { zh: 'AI 只选择档位/模板，系统会根据硬风控裁剪并执行。用于让 AI 参与仓位、分层止盈止损与追踪松紧。', en: 'AI selects buckets/profiles; system enforces hard caps. Lets AI participate in sizing, scaled TP/SL, and trailing aggressiveness.' },
      posBuckets: { zh: '仓位档位（risk_bucket）', en: 'Position Size Buckets (risk_bucket)' },
      posBucketsEnabled: { zh: '启用仓位档位', en: 'Enable buckets' },
      defaultBucket: { zh: '默认档位', en: 'Default bucket' },
      minBucketConf: { zh: '低信心强制默认档位', en: 'Force default when confidence below' },
      maxBucket: { zh: '最大允许档位封顶', en: 'Max bucket cap' },
      bucketRatios: { zh: '档位 → 净值比例', en: 'Bucket → equity ratio' },
      tpProfiles: { zh: '止盈模板（tp_profile）', en: 'TP Profiles (tp_profile)' },
      slProfiles: { zh: '止损模板（sl_profile）', en: 'SL Profiles (sl_profile)' },
      resetProfiles: { zh: '恢复默认模板', en: 'Reset to defaults' },
      levelProfit: { zh: '盈利阈值(%)', en: 'Profit(%)' },
      levelClose: { zh: '平仓比例(%)', en: 'Close(%)' },
      levelMoveBE: { zh: '移动止损到保本', en: 'Move SL to BE' },
      addLevel: { zh: '新增一档', en: 'Add level' },
      remove: { zh: '删除', en: 'Remove' },
      confirmCycles: { zh: '止损确认周期', en: 'SL confirm cycles' },
      atrMultMin: { zh: 'ATR 倍数最小', en: 'ATR mult min' },
      atrMultMax: { zh: 'ATR 倍数最大', en: 'ATR mult max' },
      trailingLevels: { zh: '追踪止损档位', en: 'Trailing levels' },
      trailingProfit: { zh: '触发盈利(%)', en: 'Profit threshold(%)' },
      trailingPct: { zh: '追踪回撤(%)', en: 'Trailing percent(%)' },
      partialCloseCooldown: { zh: '部分平仓冷却(秒)', en: 'Partial close cooldown (sec)' },
      partialCloseCooldownDesc: { zh: '避免短时间重复部分平仓（信号减仓/分层止盈等）。0/留空=默认 45 秒；范围 5~600 秒。', en: 'Avoid duplicate partial closes in short window (signal scale-out / scaled TP). 0/empty = default 45s; range 5~600.' },
      partialCloseMinPercent: { zh: '部分平仓最小比例(%)', en: 'Min partial close (%)' },
      partialCloseMinPercentDesc: { zh: '部分平仓比例过小会跳过，避免噪声或最小下单量问题。0/留空=默认 2%。', en: 'Skip too-small partial closes to avoid noise/min-order issues. 0/empty = default 2%.' },
      sltpExitStateTtl: { zh: '结构化退场状态TTL(分钟)', en: 'Exit state TTL (min)' },
      sltpExitStateTtlDesc: { zh: '结构化退场状态机超过该时间未更新则过期忽略。0/留空=默认 60。', en: 'Expire stale structural exit state after this time. 0/empty = default 60.' },
      signalExitMinHold: { zh: '信号exit最小持仓(秒)', en: 'Signal exit min-hold (sec)' },
      signalExitMinHoldDesc: { zh: '结构化信号 exit 绕过 MinHold 的最低持仓时间；0/留空=立刻允许。', en: 'Minimum hold time before structural exit can bypass MinHold. 0/empty = immediate.' },
      sltpPriority: { zh: 'SL/TP 优先级（先止损）', en: 'SL/TP priority (SL first)' },
      sltpPriorityDesc: { zh: '默认先止盈再止损；开启后先止损再止盈。', en: 'Default TP-first; enable to check SL first.' },
      scaleoutBlocksScaledtp: { zh: 'scale_out 阻断 scaled TP(秒)', en: 'scale_out blocks scaled TP (sec)' },
      scaledtpBlocksScaleout: { zh: 'scaled TP 阻断 scale_out(秒)', en: 'scaled TP blocks scale_out (sec)' },
      structExitEscalation: { zh: '结构化退场升级（阶段+强度）', en: 'Structural exit escalation (phase+strength)' },
      structExitScaleOutStrength: { zh: '升级 scale_out 强度阈值', en: 'Scale_out strength threshold' },
      structExitExitStrength: { zh: '升级 exit 强度阈值', en: 'Exit strength threshold' },
      structExitScaleOutConfirm: { zh: '升级 scale_out 确认次数', en: 'Scale_out confirm samples' },
      structExitExitConfirm: { zh: '升级 exit 确认次数', en: 'Exit confirm samples' },
      structExitEscalationDesc: { zh: '当 phase=late_trend/reversal_risk 且强度持续满足时，即使 AI 只给 tighten/hold，也可升级到 scale_out/exit。0/留空=默认：70/85 + 3/2。', en: 'When phase=late_trend/reversal_risk and strength persists, system can escalate to scale_out/exit even if AI says tighten/hold. 0/empty defaults: 70/85 + 3/2.' },
    }
    return translations[key]?.[language] || key
  }

  const updateField = <K extends keyof RiskControlConfig>(
    key: K,
    value: RiskControlConfig[K]
  ) => {
    if (!disabled) {
      onChange({ ...config, [key]: value })
    }
  }

  const defaultBuckets = (): PositionSizeBucketsConfig => ({
    enabled: true,
    default_bucket: 'medium',
    min_bucket_confidence: 70,
    buckets: { low: 0.003, medium: 0.007, high: 0.012 },
    max_bucket: 'high',
  })

  const defaultTPProfiles = (): Record<string, DynamicTakeProfitConfig> => ({
    tp_conservative: {
      enabled: true,
      min_hold_minutes: 10,
      scaled_enabled: true,
      scaled_profit_percent_mode: 'roe',
      scaled_levels: [
        { profit_percent: 3.0, close_percent: 50, move_stop_to_breakeven: true },
        { profit_percent: 6.0, close_percent: 100, move_stop_to_breakeven: false },
      ],
    },
    tp_balanced: {
      enabled: true,
      min_hold_minutes: 10,
      scaled_enabled: true,
      scaled_profit_percent_mode: 'roe',
      scaled_levels: [
        { profit_percent: 2.5, close_percent: 25, move_stop_to_breakeven: false },
        { profit_percent: 6.0, close_percent: 25, move_stop_to_breakeven: true },
        { profit_percent: 10.0, close_percent: 100, move_stop_to_breakeven: false },
      ],
    },
    tp_aggressive: {
      enabled: true,
      min_hold_minutes: 10,
      scaled_enabled: true,
      scaled_profit_percent_mode: 'roe',
      scaled_levels: [
        { profit_percent: 1.5, close_percent: 30, move_stop_to_breakeven: false },
        { profit_percent: 3.5, close_percent: 30, move_stop_to_breakeven: true },
        { profit_percent: 10.0, close_percent: 100, move_stop_to_breakeven: false },
      ],
    },
  })

  const defaultSLProfiles = (): Record<string, DynamicStopLossConfig> => ({
    sl_tight: {
      enabled: true,
      trigger_logic: 'any',
      min_hold_minutes: 10,
      initial_stop_percent: 0,
      trailing_enabled: true,
      trailing_levels: [
        { profit_threshold: 2.0, trailing_percent: 1.2 },
        { profit_threshold: 5.0, trailing_percent: 2.0 },
      ],
      atr_enabled: true,
      atr_multiplier_min: 1.2,
      atr_multiplier_max: 2.0,
      confirm_cycles: 1,
      atr_tolerance_enabled: true,
      atr_high_multiplier: 1.2,
      klines_timeframe: '15m',
      trailing_stop_only_after_first_scaled_tp: true,
      adverse_exit_when_never_profit_atr: 1.2,
    },
    sl_normal: {
      enabled: true,
      trigger_logic: 'any',
      min_hold_minutes: 10,
      initial_stop_percent: 0,
      trailing_enabled: true,
      trailing_levels: [
        { profit_threshold: 2.5, trailing_percent: 1.5 },
        { profit_threshold: 6.0, trailing_percent: 2.5 },
      ],
      atr_enabled: true,
      atr_multiplier_min: 1.5,
      atr_multiplier_max: 2.5,
      confirm_cycles: 2,
      atr_tolerance_enabled: true,
      atr_high_multiplier: 1.2,
      klines_timeframe: '15m',
      trailing_stop_only_after_first_scaled_tp: true,
      adverse_exit_when_never_profit_atr: 1.5,
    },
    sl_loose: {
      enabled: true,
      trigger_logic: 'any',
      min_hold_minutes: 10,
      initial_stop_percent: 0,
      trailing_enabled: true,
      trailing_levels: [
        { profit_threshold: 3.0, trailing_percent: 2.0 },
        { profit_threshold: 7.0, trailing_percent: 3.0 },
      ],
      atr_enabled: true,
      atr_multiplier_min: 2.0,
      atr_multiplier_max: 3.2,
      confirm_cycles: 2,
      atr_tolerance_enabled: true,
      atr_high_multiplier: 1.25,
      klines_timeframe: '15m',
      trailing_stop_only_after_first_scaled_tp: true,
      adverse_exit_when_never_profit_atr: 1.8,
    },
  })

  const bucketOptions = ['low', 'medium', 'high'] as const
  const tpProfileKeys = ['tp_conservative', 'tp_balanced', 'tp_aggressive'] as const
  const slProfileKeys = ['sl_tight', 'sl_normal', 'sl_loose'] as const

  const getBuckets = (): PositionSizeBucketsConfig =>
    config.position_size_buckets || defaultBuckets()

  const setBuckets = (next: PositionSizeBucketsConfig) =>
    updateField('position_size_buckets', next)

  const getTPProfiles = (): Record<string, DynamicTakeProfitConfig> =>
    config.tp_profiles || defaultTPProfiles()
  const setTPProfiles = (next: Record<string, DynamicTakeProfitConfig>) =>
    updateField('tp_profiles', next)

  const getSLProfiles = (): Record<string, DynamicStopLossConfig> =>
    config.sl_profiles || defaultSLProfiles()
  const setSLProfiles = (next: Record<string, DynamicStopLossConfig>) =>
    updateField('sl_profiles', next)

  const resetAiProfiles = () => {
    if (disabled) return
    updateField('position_size_buckets', defaultBuckets())
    updateField('tp_profiles', defaultTPProfiles())
    updateField('sl_profiles', defaultSLProfiles())
  }

  const updateTPLevel = (
    key: string,
    idx: number,
    patch: Partial<ScaledTakeProfitLevel>
  ) => {
    const all = getTPProfiles()
    const current = all[key] || { enabled: true }
    const levels = (current.scaled_levels || []).slice()
    levels[idx] = { ...levels[idx], ...patch }
    setTPProfiles({ ...all, [key]: { ...current, scaled_levels: levels } })
  }

  const addTPLevel = (key: string) => {
    const all = getTPProfiles()
    const current = all[key] || { enabled: true }
    const levels = (current.scaled_levels || []).slice()
    levels.push({ profit_percent: 3, close_percent: 25, move_stop_to_breakeven: false })
    setTPProfiles({ ...all, [key]: { ...current, scaled_levels: levels, scaled_enabled: true } })
  }

  const removeTPLevel = (key: string, idx: number) => {
    const all = getTPProfiles()
    const current = all[key]
    if (!current?.scaled_levels) return
    const levels = current.scaled_levels.slice()
    levels.splice(idx, 1)
    setTPProfiles({ ...all, [key]: { ...current, scaled_levels: levels } })
  }

  const updateTrailingLevel = (
    key: string,
    idx: number,
    patch: Partial<TrailingStopLevel>
  ) => {
    const all = getSLProfiles()
    const current = all[key] || { enabled: true, trigger_logic: 'any', initial_stop_percent: 0 }
    const levels = (current.trailing_levels || []).slice()
    levels[idx] = { ...levels[idx], ...patch }
    setSLProfiles({ ...all, [key]: { ...current, trailing_levels: levels } })
  }

  const addTrailingLevel = (key: string) => {
    const all = getSLProfiles()
    const current = all[key] || { enabled: true, trigger_logic: 'any', initial_stop_percent: 0 }
    const levels = (current.trailing_levels || []).slice()
    levels.push({ profit_threshold: 3, trailing_percent: 2 })
    setSLProfiles({ ...all, [key]: { ...current, trailing_levels: levels, trailing_enabled: true } })
  }

  const removeTrailingLevel = (key: string, idx: number) => {
    const all = getSLProfiles()
    const current = all[key]
    if (!current?.trailing_levels) return
    const levels = current.trailing_levels.slice()
    levels.splice(idx, 1)
    setSLProfiles({ ...all, [key]: { ...current, trailing_levels: levels } })
  }

  return (
    <div className="space-y-6">
      {/* Position Limits */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Shield className="w-5 h-5" style={{ color: '#F0B90B' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {t('positionLimits')}
          </h3>
        </div>

        <div className="grid grid-cols-1 gap-4 mb-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('maxPositions')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('maxPositionsDesc')}
            </p>
            <input
              type="number"
              value={config.max_positions ?? 3}
              onChange={(e) =>
                updateField('max_positions', parseInt(e.target.value) || 3)
              }
              disabled={disabled}
              min={1}
              max={10}
              className="w-32 px-3 py-2 rounded"
              style={{
                background: '#1E2329',
                border: '1px solid #2B3139',
                color: '#EAECEF',
              }}
            />
          </div>

          {/* AI 仅开仓 */}
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <div className="flex items-center justify-between gap-2">
              <div>
                <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
                  {t('aiOnlyEntry')}
                </label>
                <p className="text-xs" style={{ color: '#848E9C' }}>
                  {t('aiOnlyEntryDesc')}
                </p>
              </div>
              <input
                type="checkbox"
                checked={!!config.ai_only_entry}
                onChange={(e) => updateField('ai_only_entry', e.target.checked)}
                disabled={disabled}
                className="w-5 h-5 rounded accent-yellow-500"
              />
            </div>
          </div>

          {/* 系统执行开仓 */}
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <div className="flex items-center justify-between gap-2">
              <div>
                <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
                  {t('systemExecutesEntry')}
                </label>
                <p className="text-xs" style={{ color: '#848E9C' }}>
                  {t('systemExecutesEntryDesc')}
                </p>
              </div>
              <input
                type="checkbox"
                checked={!!config.system_executes_entry}
                onChange={(e) => updateField('system_executes_entry', e.target.checked)}
                disabled={disabled}
                className="w-5 h-5 rounded accent-yellow-500"
              />
            </div>
          </div>

          {/* AI 仅预测（系统决策） */}
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <div className="flex items-center justify-between gap-2">
              <div>
                <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
                  {t('aiPredictOnly')}
                </label>
                <p className="text-xs" style={{ color: '#848E9C' }}>
                  {t('aiPredictOnlyDesc')}
                </p>
              </div>
              <input
                type="checkbox"
                checked={!!config.ai_predict_only}
                onChange={(e) => updateField('ai_predict_only', e.target.checked)}
                disabled={disabled}
                className="w-5 h-5 rounded accent-yellow-500"
              />
            </div>
          </div>

          {/* 允许 AI 平仓/止盈止损 */}
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <div className="flex items-center justify-between gap-2">
              <div>
                <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
                  {t('allowAiClose')}
                </label>
                <p className="text-xs" style={{ color: '#848E9C' }}>
                  {t('allowAiCloseDesc')}
                </p>
              </div>
              <input
                type="checkbox"
                checked={!!config.allow_ai_close}
                onChange={(e) => updateField('allow_ai_close', e.target.checked)}
                disabled={disabled}
                className="w-5 h-5 rounded accent-yellow-500"
              />
            </div>
            {config.allow_ai_close && (
              <div className="mt-3 pt-3 border-t border-[#2B3139] space-y-2">
                <div>
                  <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>
                    {t('minConfidenceForAiClose')}
                  </label>
                  <p className="text-xs mb-1" style={{ color: '#5E6673' }}>
                    {t('minConfidenceForAiCloseDesc')}
                  </p>
                  <input
                    type="number"
                    min={0}
                    max={100}
                    value={config.min_confidence_for_ai_close ?? 70}
                    onChange={(e) =>
                      updateField('min_confidence_for_ai_close', parseInt(e.target.value, 10) || 0)
                    }
                    disabled={disabled}
                    className="w-20 px-2 py-1 rounded bg-[#1E2329] border border-[#2B3139] text-sm"
                    style={{ color: '#EAECEF' }}
                  />
                </div>
                <div className="flex items-center justify-between gap-2">
                  <div>
                    <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>
                      {t('requireExitReasonForAiClose')}
                    </label>
                    <p className="text-xs" style={{ color: '#5E6673' }}>
                      {t('requireExitReasonForAiCloseDesc')}
                    </p>
                  </div>
                  <input
                    type="checkbox"
                    checked={config.require_exit_reason_for_ai_close !== false}
                    onChange={(e) =>
                      updateField('require_exit_reason_for_ai_close', e.target.checked)
                    }
                    disabled={disabled}
                    className="w-5 h-5 rounded accent-yellow-500"
                  />
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Trading Leverage (Exchange) */}
        <div className="mb-2">
          <p className="text-xs font-medium mb-2" style={{ color: '#F0B90B' }}>
            {t('tradingLeverage')}
          </p>
        </div>
        <div className="grid grid-cols-2 gap-4 mb-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('btcEthLeverage')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('btcEthLeverageDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={config.btc_eth_max_leverage ?? 5}
                onChange={(e) =>
                  updateField('btc_eth_max_leverage', parseInt(e.target.value))
                }
                disabled={disabled}
                min={1}
                max={20}
                className="flex-1 accent-yellow-500"
              />
              <span
                className="w-12 text-center font-mono"
                style={{ color: '#F0B90B' }}
              >
                {config.btc_eth_max_leverage ?? 5}x
              </span>
            </div>
          </div>

          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('altcoinLeverage')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('altcoinLeverageDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={config.altcoin_max_leverage ?? 5}
                onChange={(e) =>
                  updateField('altcoin_max_leverage', parseInt(e.target.value))
                }
                disabled={disabled}
                min={1}
                max={20}
                className="flex-1 accent-yellow-500"
              />
              <span
                className="w-12 text-center font-mono"
                style={{ color: '#F0B90B' }}
              >
                {config.altcoin_max_leverage ?? 5}x
              </span>
            </div>
          </div>
        </div>

        {/* Position Value Ratio (Risk Control - CODE ENFORCED) */}
        <div className="mb-2">
          <p className="text-xs font-medium" style={{ color: '#0ECB81' }}>
            {t('positionValueRatio')}
          </p>
          <p className="text-xs mt-1" style={{ color: '#848E9C' }}>
            {t('positionValueRatioDesc')}
          </p>
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #0ECB81' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('btcEthPositionValueRatio')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('btcEthPositionValueRatioDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={config.btc_eth_max_position_value_ratio ?? 5}
                onChange={(e) =>
                  updateField('btc_eth_max_position_value_ratio', parseFloat(e.target.value))
                }
                disabled={disabled}
                min={0.5}
                max={10}
                step={0.5}
                className="flex-1 accent-green-500"
              />
              <span
                className="w-12 text-center font-mono"
                style={{ color: '#0ECB81' }}
              >
                {config.btc_eth_max_position_value_ratio ?? 5}x
              </span>
            </div>
          </div>

          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #0ECB81' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('altcoinPositionValueRatio')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('altcoinPositionValueRatioDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={config.altcoin_max_position_value_ratio ?? 1}
                onChange={(e) =>
                  updateField('altcoin_max_position_value_ratio', parseFloat(e.target.value))
                }
                disabled={disabled}
                min={0.5}
                max={10}
                step={0.5}
                className="flex-1 accent-green-500"
              />
              <span
                className="w-12 text-center font-mono"
                style={{ color: '#0ECB81' }}
              >
                {config.altcoin_max_position_value_ratio ?? 1}x
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* Risk Parameters */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <AlertTriangle className="w-5 h-5" style={{ color: '#F6465D' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {t('riskParameters')}
          </h3>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('minRiskReward')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('minRiskRewardDesc')}
            </p>
            <div className="flex items-center">
              <input
                type="number"
                value={config.min_risk_reward_ratio ?? 3}
                onChange={(e) =>
                  updateField('min_risk_reward_ratio', parseFloat(e.target.value) || 3)
                }
                disabled={disabled}
                min={1}
                max={10}
                step={0.5}
                className="w-20 px-3 py-2 rounded"
                style={{
                  background: '#1E2329',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              />
              <span style={{ color: '#848E9C' }} className="ml-2">:1</span>
            </div>
          </div>

          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #0ECB81' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('maxMarginUsage')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('maxMarginUsageDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={(config.max_margin_usage ?? 0.9) * 100}
                onChange={(e) =>
                  updateField('max_margin_usage', parseInt(e.target.value) / 100)
                }
                disabled={disabled}
                min={10}
                max={100}
                className="flex-1 accent-green-500"
              />
              <span className="w-12 text-center font-mono" style={{ color: '#0ECB81' }}>
                {Math.round((config.max_margin_usage ?? 0.9) * 100)}%
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* Entry Requirements */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Shield className="w-5 h-5" style={{ color: '#0ECB81' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {t('entryRequirements')}
          </h3>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('minPositionSize')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('minPositionSizeDesc')}
            </p>
            <div className="flex items-center">
              <input
                type="number"
                value={config.min_position_size ?? 12}
                onChange={(e) =>
                  updateField('min_position_size', parseFloat(e.target.value) || 12)
                }
                disabled={disabled}
                min={10}
                max={1000}
                className="w-24 px-3 py-2 rounded"
                style={{
                  background: '#1E2329',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              />
              <span className="ml-2" style={{ color: '#848E9C' }}>
                USDT
              </span>
            </div>
          </div>

          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('minConfidence')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('minConfidenceDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={config.min_confidence ?? 75}
                onChange={(e) =>
                  updateField('min_confidence', parseInt(e.target.value))
                }
                disabled={disabled}
                min={50}
                max={100}
                className="flex-1 accent-green-500"
              />
              <span className="w-12 text-center font-mono" style={{ color: '#0ECB81' }}>
                {config.min_confidence ?? 75}
              </span>
            </div>

            {/* Regime 调节：震荡/高波/反转时提高开仓置信度 */}
            <div className="mt-4 pt-4 border-t border-[#2B3139]">
              <label className="flex items-center gap-2 cursor-pointer text-sm" style={{ color: '#EAECEF' }}>
                <input
                  type="checkbox"
                  checked={config.regime_adjust_enabled ?? false}
                  onChange={(e) => updateField('regime_adjust_enabled', e.target.checked)}
                  disabled={disabled}
                  className="rounded accent-yellow-500"
                />
                {t('regimeAdjust')}
              </label>
              <p className="text-xs mt-1 mb-2" style={{ color: '#848E9C' }}>{t('regimeAdjustDesc')}</p>
              {(config.regime_adjust_enabled ?? false) && (
                <div className="grid grid-cols-3 gap-2 text-xs">
                  {[
                    { key: 'ranging', label: t('regimeRanging') },
                    { key: 'high_volatility', label: t('regimeHighVol') },
                    { key: 'reversal', label: t('regimeReversal') },
                  ].map(({ key, label }) => (
                    <div key={key} className="flex items-center gap-2">
                      <span className="shrink-0" style={{ color: '#848E9C' }}>{label}</span>
                      <input
                        type="number"
                        min={50}
                        max={100}
                        value={config.regime_min_confidence_map?.[key] ?? (key === 'ranging' ? 75 : key === 'high_volatility' ? 78 : 80)}
                        onChange={(e) => {
                          const v = parseInt(e.target.value, 10) || 0
                          const map = { ...(config.regime_min_confidence_map ?? {}), [key]: v }
                          updateField('regime_min_confidence_map', map)
                        }}
                        disabled={disabled}
                        className="w-14 px-2 py-1 rounded"
                        style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                      />
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* AI sizing buckets & SL/TP profiles */}
      <div>
        <div className="flex items-center justify-between gap-2 mb-2">
          <div className="flex items-center gap-2">
            <Shield className="w-5 h-5" style={{ color: '#F0B90B' }} />
            <h3 className="font-medium" style={{ color: '#EAECEF' }}>
              {t('aiSizingAndProfiles')}
            </h3>
          </div>
          <button
            type="button"
            onClick={resetAiProfiles}
            disabled={disabled}
            className="px-3 py-1.5 rounded text-xs"
            style={{
              background: '#1E2329',
              border: '1px solid #2B3139',
              color: disabled ? '#5E6673' : '#EAECEF',
            }}
          >
            {t('resetProfiles')}
          </button>
        </div>
        <p className="text-xs mb-4" style={{ color: '#848E9C' }}>
          {t('aiSizingAndProfilesDesc')}
        </p>

        {/* Position size buckets */}
        <div className="p-4 rounded-lg mb-4" style={{ background: '#0B0E11', border: '1px solid #2B3139' }}>
          <div className="flex items-center justify-between gap-2 mb-3">
            <div>
              <div className="text-sm" style={{ color: '#EAECEF' }}>{t('posBuckets')}</div>
              <div className="text-xs" style={{ color: '#848E9C' }}>{t('bucketRatios')}</div>
            </div>
            <label className="flex items-center gap-2 text-xs" style={{ color: '#EAECEF' }}>
              {t('posBucketsEnabled')}
              <input
                type="checkbox"
                checked={!!getBuckets().enabled}
                onChange={(e) => setBuckets({ ...getBuckets(), enabled: e.target.checked })}
                disabled={disabled}
                className="w-5 h-5 rounded accent-yellow-500"
              />
            </label>
          </div>

          <div className="grid grid-cols-2 gap-4 mb-3">
            <div>
              <div className="text-xs mb-1" style={{ color: '#848E9C' }}>{t('defaultBucket')}</div>
              <select
                value={getBuckets().default_bucket || 'medium'}
                onChange={(e) => setBuckets({ ...getBuckets(), default_bucket: e.target.value })}
                disabled={disabled}
                className="w-full px-3 py-2 rounded"
                style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
              >
                {bucketOptions.map((b) => (
                  <option key={b} value={b}>{b}</option>
                ))}
              </select>
            </div>
            <div>
              <div className="text-xs mb-1" style={{ color: '#848E9C' }}>{t('maxBucket')}</div>
              <select
                value={getBuckets().max_bucket || 'high'}
                onChange={(e) => setBuckets({ ...getBuckets(), max_bucket: e.target.value })}
                disabled={disabled}
                className="w-full px-3 py-2 rounded"
                style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
              >
                {bucketOptions.map((b) => (
                  <option key={b} value={b}>{b}</option>
                ))}
              </select>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-4 mb-3">
            <div>
              <div className="text-xs mb-1" style={{ color: '#848E9C' }}>{t('minBucketConf')}</div>
              <input
                type="number"
                value={getBuckets().min_bucket_confidence ?? 70}
                onChange={(e) => setBuckets({ ...getBuckets(), min_bucket_confidence: parseInt(e.target.value) || 0 })}
                disabled={disabled}
                min={0}
                max={100}
                className="w-full px-3 py-2 rounded"
                style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
              />
            </div>
          </div>

          <div className="grid grid-cols-3 gap-3">
            {bucketOptions.map((b) => (
              <div key={b}>
                <div className="text-xs mb-1" style={{ color: '#848E9C' }}>{b}</div>
                <input
                  type="number"
                  value={(getBuckets().buckets?.[b] ?? defaultBuckets().buckets?.[b] ?? 0) as number}
                  onChange={(e) => {
                    const v = parseFloat(e.target.value) || 0
                    const nextBuckets = { ...(getBuckets().buckets || {}) }
                    nextBuckets[b] = v
                    setBuckets({ ...getBuckets(), buckets: nextBuckets })
                  }}
                  disabled={disabled}
                  step={0.001}
                  min={0}
                  className="w-full px-3 py-2 rounded"
                  style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                />
                <div className="text-[11px] mt-1" style={{ color: '#5E6673' }}>
                  equity × {((getBuckets().buckets?.[b] ?? 0) * 100).toFixed(2)}%
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* TP profiles */}
        <div className="p-4 rounded-lg mb-4" style={{ background: '#0B0E11', border: '1px solid #2B3139' }}>
          <div className="text-sm mb-3" style={{ color: '#EAECEF' }}>{t('tpProfiles')}</div>
          <div className="space-y-4">
            {tpProfileKeys.map((k) => {
              const prof = getTPProfiles()[k] || defaultTPProfiles()[k]
              const levels = prof.scaled_levels || []
              return (
                <div key={k} className="p-3 rounded" style={{ background: '#1E2329', border: '1px solid #2B3139' }}>
                  <div className="flex items-center justify-between mb-2">
                    <div className="text-xs font-mono" style={{ color: '#F0B90B' }}>{k}</div>
                    <button
                      type="button"
                      onClick={() => addTPLevel(k)}
                      disabled={disabled}
                      className="px-2 py-1 rounded text-xs"
                      style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                    >
                      {t('addLevel')}
                    </button>
                  </div>
                  <div className="grid grid-cols-12 gap-2 text-xs mb-1" style={{ color: '#848E9C' }}>
                    <div className="col-span-3">{t('levelProfit')}</div>
                    <div className="col-span-3">{t('levelClose')}</div>
                    <div className="col-span-4">{t('levelMoveBE')}</div>
                    <div className="col-span-2">{t('remove')}</div>
                  </div>
                  {levels.map((lv, idx) => (
                    <div key={idx} className="grid grid-cols-12 gap-2 items-center mb-2">
                      <input
                        type="number"
                        value={lv.profit_percent ?? 0}
                        onChange={(e) => updateTPLevel(k, idx, { profit_percent: parseFloat(e.target.value) || 0 })}
                        disabled={disabled}
                        className="col-span-3 px-2 py-1 rounded text-xs"
                        style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                      />
                      <input
                        type="number"
                        value={lv.close_percent ?? 0}
                        onChange={(e) => updateTPLevel(k, idx, { close_percent: parseFloat(e.target.value) || 0 })}
                        disabled={disabled}
                        className="col-span-3 px-2 py-1 rounded text-xs"
                        style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                      />
                      <label className="col-span-4 flex items-center gap-2 text-xs" style={{ color: '#EAECEF' }}>
                        <input
                          type="checkbox"
                          checked={!!lv.move_stop_to_breakeven}
                          onChange={(e) => updateTPLevel(k, idx, { move_stop_to_breakeven: e.target.checked })}
                          disabled={disabled}
                          className="w-4 h-4 rounded accent-yellow-500"
                        />
                        {t('levelMoveBE')}
                      </label>
                      <button
                        type="button"
                        onClick={() => removeTPLevel(k, idx)}
                        disabled={disabled || levels.length <= 1}
                        className="col-span-2 px-2 py-1 rounded text-xs"
                        style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                      >
                        {t('remove')}
                      </button>
                    </div>
                  ))}
                </div>
              )
            })}
          </div>
        </div>

        {/* SL profiles */}
        <div className="p-4 rounded-lg" style={{ background: '#0B0E11', border: '1px solid #2B3139' }}>
          <div className="text-sm mb-3" style={{ color: '#EAECEF' }}>{t('slProfiles')}</div>
          <div className="space-y-4">
            {slProfileKeys.map((k) => {
              const prof = getSLProfiles()[k] || defaultSLProfiles()[k]
              const trailing = prof.trailing_levels || []
              return (
                <div key={k} className="p-3 rounded" style={{ background: '#1E2329', border: '1px solid #2B3139' }}>
                  <div className="flex items-center justify-between mb-2">
                    <div className="text-xs font-mono" style={{ color: '#F6465D' }}>{k}</div>
                    <button
                      type="button"
                      onClick={() => addTrailingLevel(k)}
                      disabled={disabled}
                      className="px-2 py-1 rounded text-xs"
                      style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                    >
                      {t('addLevel')}
                    </button>
                  </div>

                  <div className="grid grid-cols-3 gap-3 mb-3">
                    <div>
                      <div className="text-xs mb-1" style={{ color: '#848E9C' }}>{t('confirmCycles')}</div>
                      <input
                        type="number"
                        value={prof.confirm_cycles ?? 2}
                        onChange={(e) => {
                          const all = getSLProfiles()
                          setSLProfiles({ ...all, [k]: { ...prof, confirm_cycles: parseInt(e.target.value) || 1 } })
                        }}
                        disabled={disabled}
                        min={1}
                        max={10}
                        className="w-full px-2 py-1 rounded text-xs"
                        style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                      />
                    </div>
                    <div>
                      <div className="text-xs mb-1" style={{ color: '#848E9C' }}>{t('atrMultMin')}</div>
                      <input
                        type="number"
                        value={prof.atr_multiplier_min ?? 1.5}
                        onChange={(e) => {
                          const all = getSLProfiles()
                          setSLProfiles({ ...all, [k]: { ...prof, atr_multiplier_min: parseFloat(e.target.value) || 0 } })
                        }}
                        disabled={disabled}
                        step={0.1}
                        min={0}
                        className="w-full px-2 py-1 rounded text-xs"
                        style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                      />
                    </div>
                    <div>
                      <div className="text-xs mb-1" style={{ color: '#848E9C' }}>{t('atrMultMax')}</div>
                      <input
                        type="number"
                        value={prof.atr_multiplier_max ?? 2.5}
                        onChange={(e) => {
                          const all = getSLProfiles()
                          setSLProfiles({ ...all, [k]: { ...prof, atr_multiplier_max: parseFloat(e.target.value) || 0 } })
                        }}
                        disabled={disabled}
                        step={0.1}
                        min={0}
                        className="w-full px-2 py-1 rounded text-xs"
                        style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                      />
                    </div>
                  </div>

                  <div className="text-xs mb-2" style={{ color: '#848E9C' }}>{t('trailingLevels')}</div>
                  <div className="grid grid-cols-12 gap-2 text-xs mb-1" style={{ color: '#848E9C' }}>
                    <div className="col-span-4">{t('trailingProfit')}</div>
                    <div className="col-span-4">{t('trailingPct')}</div>
                    <div className="col-span-4">{t('remove')}</div>
                  </div>
                  {trailing.map((lv, idx) => (
                    <div key={idx} className="grid grid-cols-12 gap-2 items-center mb-2">
                      <input
                        type="number"
                        value={lv.profit_threshold ?? 0}
                        onChange={(e) => updateTrailingLevel(k, idx, { profit_threshold: parseFloat(e.target.value) || 0 })}
                        disabled={disabled}
                        className="col-span-4 px-2 py-1 rounded text-xs"
                        style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                      />
                      <input
                        type="number"
                        value={lv.trailing_percent ?? 0}
                        onChange={(e) => updateTrailingLevel(k, idx, { trailing_percent: parseFloat(e.target.value) || 0 })}
                        disabled={disabled}
                        className="col-span-4 px-2 py-1 rounded text-xs"
                        style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                      />
                      <button
                        type="button"
                        onClick={() => removeTrailingLevel(k, idx)}
                        disabled={disabled || trailing.length <= 1}
                        className="col-span-4 px-2 py-1 rounded text-xs"
                        style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                      >
                        {t('remove')}
                      </button>
                    </div>
                  ))}
                </div>
              )
            })}
          </div>
        </div>
      </div>

      {/* Dynamic Stop Loss */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <TrendingDown className="w-5 h-5" style={{ color: '#F6465D' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {t('dynamicStopLoss')}
          </h3>
        </div>
        <DynamicStopLossEditor
          config={config.dynamic_stop_loss}
          onChange={(dynamicStopLoss) => updateField('dynamic_stop_loss', dynamicStopLoss)}
          disabled={disabled}
          language={language}
        />
      </div>

      {/* Dynamic Take Profit */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <TrendingUp className="w-5 h-5" style={{ color: '#0ECB81' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {t('dynamicTakeProfit')}
          </h3>
        </div>
        <DynamicTakeProfitEditor
          config={config.dynamic_take_profit}
          onChange={(dynamicTakeProfit) => updateField('dynamic_take_profit', dynamicTakeProfit)}
          disabled={disabled}
          language={language}
        />
      </div>

      {/* Partial close cooldown */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Shield className="w-5 h-5" style={{ color: '#F0B90B' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {t('partialCloseCooldown')}
          </h3>
        </div>
        <div className="rounded-xl p-4" style={{ background: '#0B0E11', border: '1px solid #2B3139' }}>
          <div className="text-xs mb-3" style={{ color: '#848E9C' }}>
            {t('partialCloseCooldownDesc')}
          </div>
          <div className="flex items-center gap-3">
            <input
              type="number"
              min={0}
              max={600}
              step={1}
              value={config.partial_close_cooldown_seconds ?? 0}
              onChange={(e) => updateField('partial_close_cooldown_seconds', Number(e.target.value))}
              disabled={disabled}
              className="w-28 px-3 py-2 rounded-lg text-sm"
              style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
            />
            <span className="text-xs" style={{ color: '#848E9C' }}>sec</span>
          </div>
        </div>
      </div>

      {/* SLTP execution knobs */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Shield className="w-5 h-5" style={{ color: '#F0B90B' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            SL/TP 执行参数
          </h3>
        </div>
        <div className="rounded-xl p-4 space-y-4" style={{ background: '#0B0E11', border: '1px solid #2B3139' }}>
          <div>
            <div className="text-xs mb-2" style={{ color: '#848E9C' }}>{t('partialCloseMinPercentDesc')}</div>
            <div className="flex items-center gap-3">
              <input
                type="number"
                min={0}
                max={50}
                step={0.1}
                value={config.partial_close_min_percent ?? 0}
                onChange={(e) => updateField('partial_close_min_percent', Number(e.target.value))}
                disabled={disabled}
                className="w-28 px-3 py-2 rounded-lg text-sm"
                style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
              />
              <span className="text-xs" style={{ color: '#848E9C' }}>%</span>
              <span className="text-xs" style={{ color: '#EAECEF' }}>{t('partialCloseMinPercent')}</span>
            </div>
          </div>

          <div>
            <div className="text-xs mb-2" style={{ color: '#848E9C' }}>{t('sltpExitStateTtlDesc')}</div>
            <div className="flex items-center gap-3">
              <input
                type="number"
                min={0}
                max={1440}
                step={1}
                value={config.sltp_exit_state_ttl_minutes ?? 0}
                onChange={(e) => updateField('sltp_exit_state_ttl_minutes', Number(e.target.value))}
                disabled={disabled}
                className="w-28 px-3 py-2 rounded-lg text-sm"
                style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
              />
              <span className="text-xs" style={{ color: '#848E9C' }}>min</span>
              <span className="text-xs" style={{ color: '#EAECEF' }}>{t('sltpExitStateTtl')}</span>
            </div>
          </div>

          <div>
            <div className="text-xs mb-2" style={{ color: '#848E9C' }}>{t('signalExitMinHoldDesc')}</div>
            <div className="flex items-center gap-3">
              <input
                type="number"
                min={0}
                max={86400}
                step={1}
                value={config.signal_exit_min_hold_seconds ?? 0}
                onChange={(e) => updateField('signal_exit_min_hold_seconds', Number(e.target.value))}
                disabled={disabled}
                className="w-28 px-3 py-2 rounded-lg text-sm"
                style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
              />
              <span className="text-xs" style={{ color: '#848E9C' }}>sec</span>
              <span className="text-xs" style={{ color: '#EAECEF' }}>{t('signalExitMinHold')}</span>
            </div>
          </div>

          <div>
            <div className="text-xs mb-2" style={{ color: '#848E9C' }}>{t('structExitEscalationDesc')}</div>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <div className="text-xs mb-2" style={{ color: '#848E9C' }}>{t('structExitScaleOutStrength')}</div>
                <input
                  type="number"
                  min={0}
                  max={100}
                  step={1}
                  value={config.struct_exit_scale_out_strength ?? 0}
                  onChange={(e) => updateField('struct_exit_scale_out_strength', Number(e.target.value))}
                  disabled={disabled}
                  className="w-full px-3 py-2 rounded-lg text-sm"
                  style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                />
              </div>
              <div>
                <div className="text-xs mb-2" style={{ color: '#848E9C' }}>{t('structExitExitStrength')}</div>
                <input
                  type="number"
                  min={0}
                  max={100}
                  step={1}
                  value={config.struct_exit_exit_strength ?? 0}
                  onChange={(e) => updateField('struct_exit_exit_strength', Number(e.target.value))}
                  disabled={disabled}
                  className="w-full px-3 py-2 rounded-lg text-sm"
                  style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                />
              </div>
              <div>
                <div className="text-xs mb-2" style={{ color: '#848E9C' }}>{t('structExitScaleOutConfirm')}</div>
                <input
                  type="number"
                  min={0}
                  max={20}
                  step={1}
                  value={config.struct_exit_scale_out_confirm ?? 0}
                  onChange={(e) => updateField('struct_exit_scale_out_confirm', Number(e.target.value))}
                  disabled={disabled}
                  className="w-full px-3 py-2 rounded-lg text-sm"
                  style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                />
              </div>
              <div>
                <div className="text-xs mb-2" style={{ color: '#848E9C' }}>{t('structExitExitConfirm')}</div>
                <input
                  type="number"
                  min={0}
                  max={20}
                  step={1}
                  value={config.struct_exit_exit_confirm ?? 0}
                  onChange={(e) => updateField('struct_exit_exit_confirm', Number(e.target.value))}
                  disabled={disabled}
                  className="w-full px-3 py-2 rounded-lg text-sm"
                  style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                />
              </div>
            </div>
            <div className="text-xs mt-2" style={{ color: '#EAECEF' }}>{t('structExitEscalation')}</div>
          </div>

          <label className="flex items-center gap-3 cursor-pointer">
            <input
              type="checkbox"
              checked={config.sltp_prefer_stop_loss_over_take_profit ?? false}
              onChange={(e) => updateField('sltp_prefer_stop_loss_over_take_profit', e.target.checked)}
              disabled={disabled}
              className="w-5 h-5 accent-red-500 rounded"
            />
            <span className="text-sm" style={{ color: '#EAECEF' }}>{t('sltpPriority')}</span>
            <span className="text-[10px]" style={{ color: '#848E9C' }}>{t('sltpPriorityDesc')}</span>
          </label>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <div className="text-xs mb-2" style={{ color: '#848E9C' }}>{t('scaleoutBlocksScaledtp')}</div>
              <input
                type="number"
                min={-1}
                max={3600}
                step={1}
                value={config.scale_out_blocks_scaled_tp_seconds ?? 0}
                onChange={(e) => updateField('scale_out_blocks_scaled_tp_seconds', Number(e.target.value))}
                disabled={disabled}
                className="w-full px-3 py-2 rounded-lg text-sm"
                style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
              />
            </div>
            <div>
              <div className="text-xs mb-2" style={{ color: '#848E9C' }}>{t('scaledtpBlocksScaleout')}</div>
              <input
                type="number"
                min={-1}
                max={3600}
                step={1}
                value={config.scaled_tp_blocks_scale_out_seconds ?? 0}
                onChange={(e) => updateField('scaled_tp_blocks_scale_out_seconds', Number(e.target.value))}
                disabled={disabled}
                className="w-full px-3 py-2 rounded-lg text-sm"
                style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
              />
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
