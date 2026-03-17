import { TrendingDown, Activity, BarChart3, AlertCircle, Plus, Trash2, CornerDownRight } from 'lucide-react'
import type { DynamicStopLossConfig, TrailingStopLevel } from '../../types'

interface DynamicStopLossEditorProps {
  config: DynamicStopLossConfig | undefined
  onChange: (config: DynamicStopLossConfig | undefined) => void
  disabled?: boolean
  language: string
}

export function DynamicStopLossEditor({
  config,
  onChange,
  disabled,
  language,
}: DynamicStopLossEditorProps) {
  const t = (key: string) => {
    const translations: Record<string, Record<string, string>> = {
      enableDynamicStopLoss: { zh: '启用动态止损', en: 'Enable Dynamic Stop Loss' },
      triggerLogic: { zh: '触发逻辑', en: 'Trigger Logic' },
      triggerLogicAny: { zh: '任一条件触发', en: 'Any Condition' },
      triggerLogicAll: { zh: '所有条件触发', en: 'All Conditions' },
      triggerLogicAnyDesc: { zh: '任一启用的止损条件触发即平仓', en: 'Close position when any enabled condition triggers' },
      triggerLogicAllDesc: { zh: '所有启用的止损条件都触发才平仓', en: 'Close position only when all enabled conditions trigger' },
      
      // Min hold (initial fixed stop removed; only dynamic/ATR/trailing)
      minHoldMinutes: { zh: '最小持仓时间（分钟）', en: 'Min hold (minutes)' },
      minHoldMinutesDesc: { zh: '未满此时间不触发动态止损，避免开仓即止损。0=不限制', en: 'Do not trigger before this many minutes; 0=no limit' },
      
      // Trailing Stop - Tiered
      trailingStop: { zh: '追踪止损（分层模式）', en: 'Trailing Stop (Tiered)' },
      enableTrailing: { zh: '启用追踪止损', en: 'Enable Trailing Stop' },
      trailingLevels: { zh: '追踪止损层级', en: 'Trailing Stop Levels' },
      addLevel: { zh: '添加层级', en: 'Add Level' },
      profitThreshold: { zh: '盈利阈值', en: 'Profit Threshold' },
      trailingPercent: { zh: '允许回撤', en: 'Allowed Drawdown' },
      profitThresholdDesc: { zh: '当盈利达到此百分比时激活该层级', en: 'Activate this level when profit reaches this percentage' },
      trailingPercentDesc: { zh: '该层级允许的最大回撤百分比', en: 'Maximum drawdown allowed at this level' },
      trailingOnlyAfterFirstScaledTP: { zh: '仅在第一档分层止盈后启用追踪', en: 'Trailing only after first scaled TP' },
      trailingOnlyAfterFirstScaledTPDesc: { zh: '未触发过任何分层止盈时不启用追踪止损，先让分层止盈有机会触发，避免尚未止盈就被追踪平仓', en: 'Do not use trailing stop until at least one scaled TP level has triggered' },
      
      // ATR Stop - Dynamic Range
      atrStop: { zh: 'ATR 止损（动态区间）', en: 'ATR Stop (Dynamic Range)' },
      enableAtr: { zh: '启用 ATR 止损', en: 'Enable ATR Stop' },
      atrMultiplierRange: { zh: 'ATR 倍数区间（AI 自适应）', en: 'ATR Multiplier Range (AI Adaptive)' },
      atrMultiplierMin: { zh: '最小倍数', en: 'Min Multiplier' },
      atrMultiplierMax: { zh: '最大倍数', en: 'Max Multiplier' },
      atrMultiplierDesc: { zh: 'AI 将根据市场波动在此区间内选择合适的倍数', en: 'AI will select appropriate multiplier within this range based on market volatility' },
      atrPeriod: { zh: 'ATR 周期（按币种）', en: 'ATR Period (By Coin Type)' },
      atrPeriodBtcEth: { zh: 'BTC/ETH 周期', en: 'BTC/ETH Period' },
      atrPeriodAltcoin: { zh: '山寨币周期', en: 'Altcoin Period' },
      atrPeriodDesc: { zh: '主流币使用较长周期，山寨币使用较短周期以适应高波动', en: 'Mainstream coins use longer periods, altcoins use shorter periods for high volatility' },
      
      // Support/Resistance
      supportResistanceStop: { zh: '支撑阻力止损', en: 'Support/Resistance Stop' },
      enableSupportResistance: { zh: '启用支撑阻力止损', en: 'Enable S/R Stop' },
      srBuffer: { zh: '支撑/阻力缓冲', en: 'S/R Buffer' },
      srBufferDesc: { zh: '在支撑/阻力位基础上的缓冲百分比', en: 'Buffer percentage from support/resistance' },

      confirmCycles: { zh: '连续确认周期数', en: 'Confirm cycles' },
      confirmCyclesDesc: { zh: '止损条件连续满足 N 个周期后才执行，减少单 K 线假跌破。1=立即执行', en: 'Execute stop only after condition holds N consecutive cycles; 1=immediate' },
      confirmMinutes: { zh: '确认时长（分钟）', en: 'Confirm duration (min)' },
      confirmMinutesDesc: { zh: '>0 时按真实时间：条件需持续满足此分钟数才执行；0=使用上方周期数', en: 'When >0: require condition to hold this many minutes; 0=use cycles above' },
      klinesTimeframe: { zh: '止损K线周期', en: 'SL klines timeframe' },
      klinesTimeframeDesc: { zh: 'ATR、支撑阻力、逆势早退等使用的K线周期；1h 可减少 15m 毛刺', en: 'Timeframe for ATR, S/R, adverse exit; 1h reduces 15m noise' },
      srUseEma20: { zh: '支撑/阻力用 EMA20', en: 'S/R use EMA20' },
      srUseEma20Desc: { zh: '用同周期 EMA20 作为多单支撑/空单阻力结构位', en: 'Use EMA20 as support (long) / resistance (short)' },
      adverseAltcoin: { zh: '山寨币反向 ATR 倍数', en: 'Altcoin reverse ATR multiple' },
      adverseAltcoinDesc: { zh: '非 BTC/ETH 使用此倍数（留空则与主倍数一致）', en: 'For non-BTC/ETH (empty = same as main)' },
      adverseRequireSpike: { zh: '逆势早退需 ATR 骤升', en: 'Adverse exit requires ATR spike' },
      adverseRequireSpikeDesc: { zh: '仅当当前 ATR ≥ 长期ATR×阈值时触发，避免温和震荡早退', en: 'Only trigger when current ATR ≥ long ATR×threshold' },
      adverseSpikeThreshold: { zh: 'ATR 骤升阈值', en: 'ATR spike threshold' },
      atrTolerance: { zh: '高波动宽容', en: 'ATR volatility tolerance' },
      enableAtrTolerance: { zh: '启用高波动宽容', en: 'Enable high-vol tolerance' },
      atrToleranceDesc: { zh: '高波动时多要求 1 个确认周期且 ATR 止损放宽', en: 'In high vol: +1 confirm cycle and wider ATR stop' },
      atrHighMult: { zh: '高波动阈值倍数', en: 'High vol threshold' },
      atrHighMultDesc: { zh: '当前 ATR > 长期 ATR × 此值视为高波动', en: 'Current ATR > long-term ATR × this = high volatility' },

      adverseExit: { zh: '从未浮盈+反向过大早退', en: 'Adverse exit (never profit + reverse)' },
      enableAdverseExit: { zh: '启用逆势早退', en: 'Enable adverse early exit' },
      adverseExitDesc: { zh: '当持仓从未浮盈且价格反向移动≥设定倍数×ATR 时提前止损，减小逆势单亏损；不收紧正常 ATR 止损', en: 'When never in profit and price moves against by ≥ this × ATR, exit early to reduce loss; does not tighten normal ATR stop' },
      adverseExitAtrMult: { zh: '反向 ATR 倍数', en: 'Reverse ATR multiple' },
      adverseExitAtrMultDesc: { zh: '反向距离 ≥ 此倍数×ATR 时触发（建议 1.0–1.2，过小易被震荡洗出）', en: 'Trigger when reverse move ≥ this × ATR (recommend 1.0–1.2; too small may get stopped by chop)' },
    }
    return translations[key]?.[language] || key
  }

  // 默认值：与后端逻辑一致，兼顾「震荡市拿住仓」与「逆势可控」
  const defaultConfig: DynamicStopLossConfig = {
    enabled: true,
    trigger_logic: 'any',
    min_hold_minutes: 10,          // 与后端预设、止盈一致，主周期 15m 下更稳
    initial_stop_percent: 0,       // 0=仅用动态/ATR/追踪，不设固定百分比
    trailing_enabled: false,
    trailing_levels: [
      { profit_threshold: 2, trailing_percent: 1.5 },
      { profit_threshold: 5, trailing_percent: 2.5 },
      { profit_threshold: 10, trailing_percent: 4 },
    ],
    atr_enabled: false,
    atr_multiplier_min: 1.5,       // 与 kernel getFloat64Value 默认一致
    atr_multiplier_max: 3.5,
    atr_period_btc_eth: 20,
    atr_period_altcoin: 14,
    support_resistance_enabled: false,
    support_resistance_buffer: 0.5,
    confirm_cycles: 2,            // 连续 2 周期确认，减少单 K 线假跌破
    confirm_minutes: 0,           // 0=按周期数确认；>0 则按真实分钟数
    atr_tolerance_enabled: true,
    atr_high_multiplier: 1.2,     // 当前 ATR > 长期×1.2 视为高波动
    klines_timeframe: '15m',      // 15m 响应快；1h 可减毛刺
    support_resistance_use_ema20: false, // 默认用局部极值，可选 EMA20
    adverse_exit_when_never_profit_atr: 0,  // 0=关闭；建议 1.0–1.2 时手动开启
    adverse_exit_require_atr_spike: false,  // 默认不要求 ATR 骤升，避免漏触发
    adverse_exit_atr_spike_threshold: 1.2,  // 开启骤升过滤时默认 1.2
  }

  const currentConfig = config || defaultConfig

  const updateField = <K extends keyof DynamicStopLossConfig>(
    key: K,
    value: DynamicStopLossConfig[K]
  ) => {
    if (!disabled) {
      onChange({ ...currentConfig, [key]: value })
    }
  }

  const toggleEnabled = () => {
    if (disabled) return
    if (config?.enabled) {
      onChange(undefined)
    } else {
      onChange(defaultConfig)
    }
  }

  const addTrailingLevel = () => {
    const levels = currentConfig.trailing_levels || []
    const newLevel: TrailingStopLevel = {
      profit_threshold: levels.length > 0 ? levels[levels.length - 1].profit_threshold + 5 : 2,
      trailing_percent: levels.length > 0 ? levels[levels.length - 1].trailing_percent + 1 : 1.5,
    }
    updateField('trailing_levels', [...levels, newLevel])
  }

  const removeTrailingLevel = (index: number) => {
    const levels = currentConfig.trailing_levels || []
    updateField('trailing_levels', levels.filter((_, i) => i !== index))
  }

  const updateTrailingLevel = (index: number, field: keyof TrailingStopLevel, value: number) => {
    const levels = [...(currentConfig.trailing_levels || [])]
    levels[index] = { ...levels[index], [field]: value }
    updateField('trailing_levels', levels)
  }

  return (
    <div className="space-y-5">
      {/* Enable Toggle */}
      <div className="flex items-center justify-between p-4 rounded-xl shadow-lg" style={{ background: 'linear-gradient(135deg, #1a1d24 0%, #0f1115 100%)', border: '1px solid #2B3139' }}>
        <div className="flex items-center gap-3">
          <div className="p-2 rounded-lg" style={{ background: 'rgba(246, 70, 93, 0.1)' }}>
            <TrendingDown className="w-5 h-5" style={{ color: '#F6465D' }} />
          </div>
          <span className="text-base font-semibold" style={{ color: '#EAECEF' }}>
            {t('enableDynamicStopLoss')}
          </span>
        </div>
        <button
          onClick={toggleEnabled}
          disabled={disabled}
          className={`relative w-14 h-7 rounded-full transition-all duration-300 shadow-inner ${
            currentConfig.enabled ? 'bg-gradient-to-r from-green-500 to-green-600' : 'bg-gray-700'
          }`}
        >
          <div
            className={`absolute top-0.5 left-0.5 w-6 h-6 bg-white rounded-full transition-transform duration-300 shadow-md ${
              currentConfig.enabled ? 'translate-x-7' : ''
            }`}
          />
        </button>
      </div>

      {currentConfig.enabled && (
        <>
          {/* Trigger Logic */}
          <div className="p-4 rounded-xl shadow-lg" style={{ background: 'linear-gradient(135deg, #2a2410 0%, #1a1808 100%)', border: '1px solid #F0B90B' }}>
            <div className="flex items-center gap-2 mb-3">
              <AlertCircle className="w-5 h-5" style={{ color: '#F0B90B' }} />
              <label className="text-sm font-semibold" style={{ color: '#EAECEF' }}>
                {t('triggerLogic')}
              </label>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <button
                onClick={() => updateField('trigger_logic', 'any')}
                disabled={disabled}
                className={`p-4 rounded-xl border-2 transition-all duration-200 ${
                  currentConfig.trigger_logic === 'any'
                    ? 'border-yellow-500 bg-yellow-500/15 shadow-lg'
                    : 'border-gray-700 hover:border-yellow-500/50 hover:bg-yellow-500/5'
                }`}
              >
                <div className="text-sm font-semibold mb-1" style={{ color: currentConfig.trigger_logic === 'any' ? '#F0B90B' : '#848E9C' }}>
                  {t('triggerLogicAny')}
                </div>
                <div className="text-xs leading-relaxed" style={{ color: '#848E9C' }}>
                  {t('triggerLogicAnyDesc')}
                </div>
              </button>
              <button
                onClick={() => updateField('trigger_logic', 'all')}
                disabled={disabled}
                className={`p-4 rounded-xl border-2 transition-all duration-200 ${
                  currentConfig.trigger_logic === 'all'
                    ? 'border-yellow-500 bg-yellow-500/15 shadow-lg'
                    : 'border-gray-700 hover:border-yellow-500/50 hover:bg-yellow-500/5'
                }`}
              >
                <div className="text-sm font-semibold mb-1" style={{ color: currentConfig.trigger_logic === 'all' ? '#F0B90B' : '#848E9C' }}>
                  {t('triggerLogicAll')}
                </div>
                <div className="text-xs leading-relaxed" style={{ color: '#848E9C' }}>
                  {t('triggerLogicAllDesc')}
                </div>
              </button>
            </div>
          </div>

          {/* Min hold (fixed initial stop removed; only dynamic/ATR/trailing) */}
          <div className="p-4 rounded-xl shadow-lg" style={{ background: 'linear-gradient(135deg, #2a1418 0%, #1a0c0f 100%)', border: '1px solid #F6465D' }}>
            <p className="text-xs mb-3 leading-relaxed" style={{ color: '#848E9C' }}>
              {language === 'zh' ? '已移除固定初始止损，仅使用动态/ATR/追踪止损。请设置最小持仓时间避免开仓即止损。' : 'Fixed initial stop removed; only dynamic/ATR/trailing stop. Set min hold to avoid stop right after open.'}
            </p>
            <div>
              <label className="text-xs block mb-1" style={{ color: '#848E9C' }}>{t('minHoldMinutes')}</label>
              <div className="flex items-center gap-2 flex-wrap">
                <input
                  type="number"
                  min={0}
                  max={120}
                  step={1}
                  value={currentConfig.min_hold_minutes ?? 10}
                  onChange={(e) => updateField('min_hold_minutes', parseFloat(e.target.value) || 0)}
                  disabled={disabled}
                  className="w-16 rounded px-2 py-1 text-sm"
                  style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                />
                <span className="text-xs" style={{ color: '#848E9C' }}>{language === 'zh' ? '分钟' : 'min'}</span>
                <span className="text-[10px]" style={{ color: '#5E6673' }}>{t('minHoldMinutesDesc')}</span>
              </div>
            </div>
          </div>

          {/* Confirm cycles + ATR tolerance */}
          <div className="p-4 rounded-xl shadow-lg" style={{ background: 'linear-gradient(135deg, #1a1d24 0%, #0f1115 100%)', border: '1px solid #2B3139' }}>
            <p className="text-xs mb-3 leading-relaxed" style={{ color: '#848E9C' }}>
              {language === 'zh' ? '连续确认再止损：条件连续满足 N 周期后才执行，减少单 K 线假跌破。高波动时更宽容（多 1 个确认周期 + ATR 止损放宽）。' : 'Confirm before execute: require N consecutive cycles; high vol = +1 confirm and wider ATR stop.'}
            </p>
            <div className="space-y-3">
              <div>
                <label className="text-xs block mb-1" style={{ color: '#848E9C' }}>{t('confirmCycles')}</label>
                <input
                  type="number"
                  min={1}
                  max={5}
                  value={currentConfig.confirm_cycles ?? 2}
                  onChange={(e) => updateField('confirm_cycles', Math.max(1, parseInt(e.target.value, 10) || 1))}
                  disabled={disabled}
                  className="w-16 rounded px-2 py-1 text-sm"
                  style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                />
                <span className="text-[10px] ml-2" style={{ color: '#5E6673' }}>{t('confirmCyclesDesc')}</span>
              </div>
              <div>
                <label className="text-xs block mb-1" style={{ color: '#848E9C' }}>{t('confirmMinutes')}</label>
                <input
                  type="number"
                  min={0}
                  max={60}
                  step={1}
                  value={currentConfig.confirm_minutes ?? 0}
                  onChange={(e) => updateField('confirm_minutes', Math.max(0, parseFloat(e.target.value) || 0))}
                  disabled={disabled}
                  className="w-16 rounded px-2 py-1 text-sm"
                  style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                />
                <span className="text-[10px] ml-2" style={{ color: '#5E6673' }}>{t('confirmMinutesDesc')}</span>
              </div>
              <div>
                <label className="text-xs block mb-1" style={{ color: '#848E9C' }}>{t('klinesTimeframe')}</label>
                <select
                  value={currentConfig.klines_timeframe || '15m'}
                  onChange={(e) => updateField('klines_timeframe', e.target.value)}
                  disabled={disabled}
                  className="rounded px-2 py-1 text-sm"
                  style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                >
                  <option value="15m">15m</option>
                  <option value="1h">1h</option>
                </select>
                <span className="text-[10px] ml-2" style={{ color: '#5E6673' }}>{t('klinesTimeframeDesc')}</span>
              </div>
              <label className="flex items-center gap-3 cursor-pointer">
                <input
                  type="checkbox"
                  checked={currentConfig.atr_tolerance_enabled ?? true}
                  onChange={(e) => updateField('atr_tolerance_enabled', e.target.checked)}
                  disabled={disabled}
                  className="w-5 h-5 accent-red-500 rounded"
                />
                <span className="text-sm" style={{ color: '#EAECEF' }}>{t('enableAtrTolerance')}</span>
              </label>
              <p className="text-[10px]" style={{ color: '#848E9C' }}>{t('atrToleranceDesc')}</p>
              {currentConfig.atr_tolerance_enabled !== false && (
                <div>
                  <label className="text-xs block mb-1" style={{ color: '#848E9C' }}>{t('atrHighMult')}</label>
                  <input
                    type="number"
                    min={1}
                    max={2}
                    step={0.1}
                    value={currentConfig.atr_high_multiplier ?? 1.2}
                    onChange={(e) => updateField('atr_high_multiplier', parseFloat(e.target.value) || 1.2)}
                    disabled={disabled}
                    className="w-20 rounded px-2 py-1 text-sm"
                    style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                  />
                  <span className="text-[10px] ml-2" style={{ color: '#5E6673' }}>{t('atrHighMultDesc')}</span>
                </div>
              )}
            </div>
          </div>

          {/* Trailing Stop - Tiered */}
          <div className="p-4 rounded-xl shadow-lg" style={{ background: 'linear-gradient(135deg, #1a1d24 0%, #0f1115 100%)', border: currentConfig.trailing_enabled ? '2px solid #F6465D' : '1px solid #2B3139' }}>
            <label className="flex items-center gap-3 cursor-pointer mb-4">
              <input
                type="checkbox"
                checked={currentConfig.trailing_enabled || false}
                onChange={(e) => updateField('trailing_enabled', e.target.checked)}
                disabled={disabled}
                className="w-5 h-5 accent-red-500 rounded"
              />
              <div className="p-2 rounded-lg" style={{ background: currentConfig.trailing_enabled ? 'rgba(246, 70, 93, 0.1)' : 'rgba(132, 142, 156, 0.1)' }}>
                <Activity className="w-5 h-5" style={{ color: currentConfig.trailing_enabled ? '#F6465D' : '#848E9C' }} />
              </div>
              <span className="text-base font-semibold" style={{ color: currentConfig.trailing_enabled ? '#EAECEF' : '#848E9C' }}>
                {t('trailingStop')}
              </span>
            </label>

            {currentConfig.trailing_enabled && (
              <div className="space-y-3 pl-2">
                <label className="flex items-center gap-2 cursor-pointer text-sm" style={{ color: '#EAECEF' }}>
                  <input
                    type="checkbox"
                    checked={currentConfig.trailing_stop_only_after_first_scaled_tp ?? false}
                    onChange={(e) => updateField('trailing_stop_only_after_first_scaled_tp', e.target.checked)}
                    disabled={disabled}
                    className="rounded border-[#2B3139] bg-[#1E2329] text-[#F0B90B]"
                  />
                  <span>{t('trailingOnlyAfterFirstScaledTP')}</span>
                </label>
                <p className="text-[10px] leading-relaxed" style={{ color: '#848E9C' }}>{t('trailingOnlyAfterFirstScaledTPDesc')}</p>
                <div className="flex items-center justify-between mb-3">
                  <span className="text-xs font-semibold" style={{ color: '#848E9C' }}>
                    {t('trailingLevels')}
                  </span>
                  <button
                    onClick={addTrailingLevel}
                    disabled={disabled}
                    className="flex items-center gap-1 px-3 py-1.5 rounded-lg text-xs font-medium transition-all hover:scale-105"
                    style={{ background: 'rgba(14, 203, 129, 0.1)', color: '#0ECB81', border: '1px solid #0ECB81' }}
                  >
                    <Plus className="w-3 h-3" />
                    {t('addLevel')}
                  </button>
                </div>

                {(currentConfig.trailing_levels || []).map((level, index) => (
                  <div key={index} className="p-3 rounded-lg border border-gray-700 bg-black/30 space-y-3">
                    <div className="flex items-center justify-between">
                      <span className="text-xs font-semibold" style={{ color: '#F0B90B' }}>
                        层级 {index + 1}
                      </span>
                      {(currentConfig.trailing_levels?.length || 0) > 1 && (
                        <button
                          onClick={() => removeTrailingLevel(index)}
                          disabled={disabled}
                          className="p-1 rounded hover:bg-red-500/20 transition-colors"
                        >
                          <Trash2 className="w-3 h-3" style={{ color: '#F6465D' }} />
                        </button>
                      )}
                    </div>

                    <div>
                      <label className="block text-xs mb-1.5 font-medium" style={{ color: '#EAECEF' }}>
                        {t('profitThreshold')}
                      </label>
                      <p className="text-[10px] mb-2 leading-relaxed" style={{ color: '#848E9C' }}>
                        {t('profitThresholdDesc')}
                      </p>
                      <div className="flex items-center gap-2">
                        <input
                          type="range"
                          value={level.profit_threshold}
                          onChange={(e) => updateTrailingLevel(index, 'profit_threshold', parseFloat(e.target.value))}
                          disabled={disabled}
                          min={0}
                          max={30}
                          step={0.5}
                          className="flex-1 h-1.5 accent-green-500"
                        />
                        <span className="w-16 text-center font-mono text-xs font-bold px-2 py-1 rounded" style={{ color: '#0ECB81', background: 'rgba(14, 203, 129, 0.1)' }}>
                          {level.profit_threshold}%
                        </span>
                      </div>
                    </div>

                    <div>
                      <label className="block text-xs mb-1.5 font-medium" style={{ color: '#EAECEF' }}>
                        {t('trailingPercent')}
                      </label>
                      <p className="text-[10px] mb-2 leading-relaxed" style={{ color: '#848E9C' }}>
                        {t('trailingPercentDesc')}
                      </p>
                      <div className="flex items-center gap-2">
                        <input
                          type="range"
                          value={level.trailing_percent}
                          onChange={(e) => updateTrailingLevel(index, 'trailing_percent', parseFloat(e.target.value))}
                          disabled={disabled}
                          min={0.5}
                          max={10}
                          step={0.5}
                          className="flex-1 h-1.5 accent-red-500"
                        />
                        <span className="w-16 text-center font-mono text-xs font-bold px-2 py-1 rounded" style={{ color: '#F6465D', background: 'rgba(246, 70, 93, 0.1)' }}>
                          {level.trailing_percent}%
                        </span>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* ATR Stop - Dynamic Range */}
          <div className="p-4 rounded-xl shadow-lg" style={{ background: 'linear-gradient(135deg, #1a1d24 0%, #0f1115 100%)', border: currentConfig.atr_enabled ? '2px solid #F6465D' : '1px solid #2B3139' }}>
            <label className="flex items-center gap-3 cursor-pointer mb-4">
              <input
                type="checkbox"
                checked={currentConfig.atr_enabled || false}
                onChange={(e) => updateField('atr_enabled', e.target.checked)}
                disabled={disabled}
                className="w-5 h-5 accent-red-500 rounded"
              />
              <div className="p-2 rounded-lg" style={{ background: currentConfig.atr_enabled ? 'rgba(246, 70, 93, 0.1)' : 'rgba(132, 142, 156, 0.1)' }}>
                <BarChart3 className="w-5 h-5" style={{ color: currentConfig.atr_enabled ? '#F6465D' : '#848E9C' }} />
              </div>
              <span className="text-base font-semibold" style={{ color: currentConfig.atr_enabled ? '#EAECEF' : '#848E9C' }}>
                {t('atrStop')}
              </span>
            </label>

            {currentConfig.atr_enabled && (
              <div className="space-y-4 pl-2">
                <div className="p-3 rounded-lg" style={{ background: 'rgba(240, 185, 11, 0.05)', border: '1px solid rgba(240, 185, 11, 0.2)' }}>
                  <label className="block text-xs mb-2 font-semibold" style={{ color: '#F0B90B' }}>
                    {t('atrMultiplierRange')}
                  </label>
                  <p className="text-[10px] mb-3 leading-relaxed" style={{ color: '#848E9C' }}>
                    {t('atrMultiplierDesc')}
                  </p>
                  
                  <div className="space-y-3">
                    <div>
                      <label className="block text-xs mb-1.5" style={{ color: '#EAECEF' }}>
                        {t('atrMultiplierMin')}
                      </label>
                      <div className="flex items-center gap-2">
                        <input
                          type="range"
                          value={currentConfig.atr_multiplier_min || 1.5}
                          onChange={(e) => updateField('atr_multiplier_min', parseFloat(e.target.value))}
                          disabled={disabled}
                          min={0.5}
                          max={5}
                          step={0.1}
                          className="flex-1 h-1.5 accent-orange-500"
                        />
                        <span className="w-16 text-center font-mono text-xs font-bold px-2 py-1 rounded" style={{ color: '#F0B90B', background: 'rgba(240, 185, 11, 0.1)' }}>
                          {currentConfig.atr_multiplier_min || 1.5}x
                        </span>
                      </div>
                    </div>

                    <div>
                      <label className="block text-xs mb-1.5" style={{ color: '#EAECEF' }}>
                        {t('atrMultiplierMax')}
                      </label>
                      <div className="flex items-center gap-2">
                        <input
                          type="range"
                          value={currentConfig.atr_multiplier_max || 3.5}
                          onChange={(e) => updateField('atr_multiplier_max', parseFloat(e.target.value))}
                          disabled={disabled}
                          min={0.5}
                          max={5}
                          step={0.1}
                          className="flex-1 h-1.5 accent-orange-500"
                        />
                        <span className="w-16 text-center font-mono text-xs font-bold px-2 py-1 rounded" style={{ color: '#F0B90B', background: 'rgba(240, 185, 11, 0.1)' }}>
                          {currentConfig.atr_multiplier_max || 3.5}x
                        </span>
                      </div>
                    </div>
                  </div>
                </div>

                <div className="p-3 rounded-lg" style={{ background: 'rgba(14, 203, 129, 0.05)', border: '1px solid rgba(14, 203, 129, 0.2)' }}>
                  <label className="block text-xs mb-2 font-semibold" style={{ color: '#0ECB81' }}>
                    {t('atrPeriod')}
                  </label>
                  <p className="text-[10px] mb-3 leading-relaxed" style={{ color: '#848E9C' }}>
                    {t('atrPeriodDesc')}
                  </p>
                  
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label className="block text-xs mb-1.5" style={{ color: '#EAECEF' }}>
                        {t('atrPeriodBtcEth')}
                      </label>
                      <input
                        type="number"
                        value={currentConfig.atr_period_btc_eth || 20}
                        onChange={(e) => updateField('atr_period_btc_eth', parseInt(e.target.value) || 20)}
                        disabled={disabled}
                        min={5}
                        max={50}
                        className="w-full px-3 py-2 rounded-lg text-sm font-mono font-bold text-center"
                        style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#0ECB81' }}
                      />
                    </div>

                    <div>
                      <label className="block text-xs mb-1.5" style={{ color: '#EAECEF' }}>
                        {t('atrPeriodAltcoin')}
                      </label>
                      <input
                        type="number"
                        value={currentConfig.atr_period_altcoin || 14}
                        onChange={(e) => updateField('atr_period_altcoin', parseInt(e.target.value) || 14)}
                        disabled={disabled}
                        min={5}
                        max={50}
                        className="w-full px-3 py-2 rounded-lg text-sm font-mono font-bold text-center"
                        style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#0ECB81' }}
                      />
                    </div>
                  </div>
                </div>
              </div>
            )}
          </div>

          {/* Support/Resistance Stop */}
          <div className="p-4 rounded-xl shadow-lg" style={{ background: 'linear-gradient(135deg, #1a1d24 0%, #0f1115 100%)', border: currentConfig.support_resistance_enabled ? '2px solid #F6465D' : '1px solid #2B3139' }}>
            <label className="flex items-center gap-3 cursor-pointer mb-4">
              <input
                type="checkbox"
                checked={currentConfig.support_resistance_enabled || false}
                onChange={(e) => updateField('support_resistance_enabled', e.target.checked)}
                disabled={disabled}
                className="w-5 h-5 accent-red-500 rounded"
              />
              <div className="p-2 rounded-lg" style={{ background: currentConfig.support_resistance_enabled ? 'rgba(246, 70, 93, 0.1)' : 'rgba(132, 142, 156, 0.1)' }}>
                <TrendingDown className="w-5 h-5" style={{ color: currentConfig.support_resistance_enabled ? '#F6465D' : '#848E9C' }} />
              </div>
              <span className="text-base font-semibold" style={{ color: currentConfig.support_resistance_enabled ? '#EAECEF' : '#848E9C' }}>
                {t('supportResistanceStop')}
              </span>
            </label>

            {currentConfig.support_resistance_enabled && (
              <div className="pl-2 space-y-3">
                <label className="flex items-center gap-3 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={currentConfig.support_resistance_use_ema20 ?? false}
                    onChange={(e) => updateField('support_resistance_use_ema20', e.target.checked)}
                    disabled={disabled}
                    className="w-5 h-5 accent-red-500 rounded"
                  />
                  <span className="text-sm" style={{ color: '#EAECEF' }}>{t('srUseEma20')}</span>
                </label>
                <p className="text-[10px]" style={{ color: '#848E9C' }}>{t('srUseEma20Desc')}</p>
                <label className="block text-xs mb-1.5 font-medium" style={{ color: '#EAECEF' }}>
                  {t('srBuffer')}
                </label>
                <p className="text-[10px] mb-3 leading-relaxed" style={{ color: '#848E9C' }}>
                  {t('srBufferDesc')}
                </p>
                <div className="flex items-center gap-3">
                  <input
                    type="range"
                    value={currentConfig.support_resistance_buffer || 0.5}
                    onChange={(e) => updateField('support_resistance_buffer', parseFloat(e.target.value))}
                    disabled={disabled}
                    min={0.1}
                    max={2}
                    step={0.1}
                    className="flex-1 h-1.5 accent-red-500"
                  />
                  <span className="w-16 text-center font-mono text-xs font-bold px-2 py-1 rounded" style={{ color: '#F6465D', background: 'rgba(246, 70, 93, 0.1)' }}>
                    {currentConfig.support_resistance_buffer || 0.5}%
                  </span>
                </div>
              </div>
            )}
          </div>

          {/* 从未浮盈+反向过大早退 */}
          <div className="p-4 rounded-xl shadow-lg" style={{ background: 'linear-gradient(135deg, #1a1d24 0%, #0f1115 100%)', border: (currentConfig.adverse_exit_when_never_profit_atr ?? 0) > 0 ? '2px solid #F6465D' : '1px solid #2B3139' }}>
            <label className="flex items-center gap-3 cursor-pointer mb-4">
              <input
                type="checkbox"
                checked={(currentConfig.adverse_exit_when_never_profit_atr ?? 0) > 0}
                onChange={(e) => updateField('adverse_exit_when_never_profit_atr', e.target.checked ? (currentConfig.adverse_exit_when_never_profit_atr && currentConfig.adverse_exit_when_never_profit_atr > 0 ? currentConfig.adverse_exit_when_never_profit_atr : 1.2) : 0)}
                disabled={disabled}
                className="w-5 h-5 accent-red-500 rounded"
              />
              <div className="p-2 rounded-lg" style={{ background: (currentConfig.adverse_exit_when_never_profit_atr ?? 0) > 0 ? 'rgba(246, 70, 93, 0.1)' : 'rgba(132, 142, 156, 0.1)' }}>
                <CornerDownRight className="w-5 h-5" style={{ color: (currentConfig.adverse_exit_when_never_profit_atr ?? 0) > 0 ? '#F6465D' : '#848E9C' }} />
              </div>
              <span className="text-base font-semibold" style={{ color: (currentConfig.adverse_exit_when_never_profit_atr ?? 0) > 0 ? '#EAECEF' : '#848E9C' }}>
                {t('adverseExit')}
              </span>
            </label>
            <p className="text-[10px] mb-3 leading-relaxed" style={{ color: '#848E9C' }}>
              {t('adverseExitDesc')}
            </p>
            {(currentConfig.adverse_exit_when_never_profit_atr ?? 0) > 0 && (
              <div className="pl-2 space-y-3">
                <div>
                  <label className="block text-xs mb-1.5 font-medium" style={{ color: '#EAECEF' }}>
                    {t('adverseExitAtrMult')}
                  </label>
                  <p className="text-[10px] mb-2 leading-relaxed" style={{ color: '#848E9C' }}>
                    {t('adverseExitAtrMultDesc')}
                  </p>
                  <div className="flex items-center gap-3">
                    <input
                      type="range"
                      min={0.5}
                      max={2.5}
                      step={0.1}
                      value={currentConfig.adverse_exit_when_never_profit_atr || 1.2}
                      onChange={(e) => updateField('adverse_exit_when_never_profit_atr', parseFloat(e.target.value) || 1.2)}
                      disabled={disabled}
                      className="flex-1 h-1.5 accent-red-500"
                    />
                    <span className="w-14 text-center font-mono text-xs font-bold px-2 py-1 rounded" style={{ color: '#F6465D', background: 'rgba(246, 70, 93, 0.1)' }}>
                      {(currentConfig.adverse_exit_when_never_profit_atr || 1.2).toFixed(1)}×
                    </span>
                  </div>
                </div>
                <div>
                  <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>{t('adverseAltcoin')}</label>
                  <input
                    type="number"
                    min={0}
                    max={2.5}
                    step={0.1}
                    placeholder={language === 'zh' ? '同主倍数' : 'Same as main'}
                    value={currentConfig.adverse_exit_when_never_profit_atr_altcoin != null && currentConfig.adverse_exit_when_never_profit_atr_altcoin > 0 ? currentConfig.adverse_exit_when_never_profit_atr_altcoin : ''}
                    onChange={(e) => updateField('adverse_exit_when_never_profit_atr_altcoin', e.target.value === '' ? undefined : parseFloat(e.target.value) || 0)}
                    disabled={disabled}
                    className="w-20 rounded px-2 py-1 text-sm"
                    style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                  />
                  <span className="text-[10px] ml-2" style={{ color: '#5E6673' }}>{t('adverseAltcoinDesc')}</span>
                </div>
                <label className="flex items-center gap-3 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={currentConfig.adverse_exit_require_atr_spike ?? false}
                    onChange={(e) => updateField('adverse_exit_require_atr_spike', e.target.checked)}
                    disabled={disabled}
                    className="w-5 h-5 accent-red-500 rounded"
                  />
                  <span className="text-sm" style={{ color: '#EAECEF' }}>{t('adverseRequireSpike')}</span>
                </label>
                <p className="text-[10px]" style={{ color: '#848E9C' }}>{t('adverseRequireSpikeDesc')}</p>
                {currentConfig.adverse_exit_require_atr_spike && (
                  <div>
                    <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>{t('adverseSpikeThreshold')}</label>
                    <input
                      type="number"
                      min={1}
                      max={2}
                      step={0.1}
                      value={currentConfig.adverse_exit_atr_spike_threshold ?? 1.2}
                      onChange={(e) => updateField('adverse_exit_atr_spike_threshold', parseFloat(e.target.value) || 1.2)}
                      disabled={disabled}
                      className="w-16 rounded px-2 py-1 text-sm"
                      style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                    />
                  </div>
                )}
              </div>
            )}
          </div>
        </>
      )}
    </div>
  )
}
