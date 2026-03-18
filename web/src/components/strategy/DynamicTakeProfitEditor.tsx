import { TrendingUp, Target, Layers, BarChart3, Plus, Trash2, Lock } from 'lucide-react'
import type { DynamicTakeProfitConfig, ScaledTakeProfitLevel } from '../../types'

interface DynamicTakeProfitEditorProps {
  config: DynamicTakeProfitConfig | undefined
  onChange: (config: DynamicTakeProfitConfig | undefined) => void
  disabled?: boolean
  language: string
}

export function DynamicTakeProfitEditor({
  config,
  onChange,
  disabled,
  language,
}: DynamicTakeProfitEditorProps) {
  const t = (key: string) => {
    const translations: Record<string, Record<string, string>> = {
      enableDynamicTakeProfit: { zh: '启用动态止盈', en: 'Enable Dynamic Take Profit' },
      
      // Fixed Take Profit
      fixedTakeProfit: { zh: '固定止盈', en: 'Fixed Take Profit' },
      enableFixed: { zh: '启用固定止盈', en: 'Enable Fixed' },
      fixedPercent: { zh: '固定止盈百分比', en: 'Fixed Percent' },
      fixedPercentDesc: { zh: '达到此盈利百分比时全部平仓', en: 'Close all at this profit percentage' },
      
      // Scaled Take Profit
      scaledTakeProfit: { zh: '分批止盈', en: 'Scaled Take Profit' },
      enableScaled: { zh: '启用分批止盈', en: 'Enable Scaled' },
      scaledLevels: { zh: '分批止盈层级', en: 'Scaled Levels' },
      addLevel: { zh: '添加层级', en: 'Add Level' },
      profitPercent: { zh: '盈利百分比', en: 'Profit %' },
      closePercent: { zh: '平仓百分比', en: 'Close %' },
      moveToBreakeven: { zh: '移动止损到盈亏平衡', en: 'Move Stop to Breakeven' },
      profitPercentDesc: { zh: '当盈利达到此百分比时触发。可选择按价格变动%或按ROE(保证金收益率)%触发；ROE≈价格%×杠杆', en: 'Trigger when profit reaches this %. You can choose price% or ROE% (margin return); ROE ≈ price% × leverage.' },
      scaledProfitMode: { zh: '分层止盈阈值口径', en: 'Scaled TP profit mode' },
      scaledProfitModeDesc: { zh: 'price=按价格涨跌幅%触发；roe=按ROE%触发（约等于价格%×杠杆）。建议短中线/带杠杆使用 roe。', en: 'price=price change %; roe=ROE% (≈ price%×leverage). For leveraged swing, prefer roe.' },
      closePercentDesc: { zh: '平仓该百分比的持仓', en: 'Close this percentage of position' },
      
      // ATR Take Profit
      atrTakeProfit: { zh: 'ATR 止盈（动态区间）', en: 'ATR Take Profit (Dynamic Range)' },
      enableAtr: { zh: '启用 ATR 止盈', en: 'Enable ATR' },
      atrMultiplierRange: { zh: 'ATR 倍数区间（AI 自适应）', en: 'ATR Multiplier Range (AI Adaptive)' },
      atrMultiplierMin: { zh: '最小倍数', en: 'Min Multiplier' },
      atrMultiplierMax: { zh: '最大倍数', en: 'Max Multiplier' },
      atrMultiplierDesc: { zh: 'AI 将根据市场趋势在此区间内选择合适的倍数', en: 'AI will select appropriate multiplier within this range based on market trend' },
      atrPeriod: { zh: 'ATR 周期（按币种）', en: 'ATR Period (By Coin Type)' },
      atrPeriodBtcEth: { zh: 'BTC/ETH 周期', en: 'BTC/ETH Period' },
      atrPeriodAltcoin: { zh: '山寨币周期', en: 'Altcoin Period' },
      atrPeriodDesc: { zh: '主流币使用较长周期，山寨币使用较短周期', en: 'Mainstream coins use longer periods, altcoins use shorter periods' },
      
      // Resistance Take Profit
      resistanceTakeProfit: { zh: '阻力位止盈', en: 'Resistance Take Profit' },
      enableResistance: { zh: '启用阻力位止盈', en: 'Enable Resistance' },
      resistanceBuffer: { zh: '阻力位缓冲', en: 'Resistance Buffer' },
      resistanceBufferDesc: { zh: '在阻力位基础上的缓冲百分比', en: 'Buffer percentage from resistance level' },
      
      // Common Settings
      lockProfitPercent: { zh: '锁定利润阈值', en: 'Lock Profit Threshold' },
      lockProfitPercentDesc: { zh: '达到此保证金收益率%后，回撤到入场价时由止损侧触发盈亏平衡止损（与止盈档位的价格%不同）', en: 'After this margin% profit, breakeven stop triggers on retrace to entry (differs from TP levels which use price%)' },
      minHoldMinutes: { zh: '最小持仓时间（分钟）', en: 'Min hold (minutes)' },
      minHoldMinutesDesc: { zh: '未满此时间不触发动态止盈，避免开仓即止盈。0=不限制', en: 'Do not trigger before this many minutes; 0=no limit' },
      trailingTP: { zh: '回撤止盈', en: 'Trailing / Pullback TP' },
      trailingTPDesc: { zh: '有盈利后从峰值回撤即止盈，不依赖涨到固定档位；用 ATR 或确认时长防震荡', en: 'Take profit on pullback from peak once in profit; ATR or confirm to resist chop' },
      trailingTPActivate: { zh: '激活阈值（盈利%）', en: 'Activate min profit %' },
      trailingTPRetrace: { zh: '回撤幅度（%）', en: 'Retrace % from peak' },
      trailingTPRetraceATR: { zh: '回撤 ATR 倍数', en: 'Retrace ATR mult' },
      trailingTPConfirmMin: { zh: '确认时长（分钟）', en: 'Confirm (minutes)' },
      trailingTPClosePct: { zh: '触发时平仓比例', en: 'Close % on trigger' },
      trailingTPActivateAltcoin: { zh: '山寨币激活阈值%', en: 'Altcoin activate %' },
      trailingTPRetraceAltcoin: { zh: '山寨币回撤%', en: 'Altcoin retrace %' },
      minProfitToAllowTP: { zh: '最低盈利%才允许止盈', en: 'Min profit % to allow TP' },
      minProfitToAllowTPDesc: { zh: '当前浮盈（价格%）低于此值不触发任何止盈；0=不限制', en: 'No TP if current profit (price %) below this; 0=no limit' },
      atrUseMaxInHighVol: { zh: '高波动时用 Max 倍数', en: 'Use max multiplier in high vol' },
      atrHighVolThreshold: { zh: '高波动阈值 (atr/atrLong≥)', en: 'High vol threshold' },
      pricePercentNote: { zh: '所有止盈档位均为价格相对入场价的变动%，与页面「当前盈亏%」（保证金%）不同；带杠杆时保证金% ≈ 价格% × 杠杆。', en: 'All TP levels use price change % from entry, not margin P/L %; with leverage, margin % ≈ price % × leverage.' },
    }
    return translations[key]?.[language] || key
  }

  // 预设与后端 GetDefaultStrategyConfig 对齐；未列出的可选项（如山寨回撤）不设=使用主参数
  const defaultConfig: DynamicTakeProfitConfig = {
    enabled: true,
    min_hold_minutes: 10,           // 与止损一致，减少开仓即触发
    min_profit_percent_to_allow_tp: 0, // 0=不限制；>0 时仅当浮盈≥此值才允许触发止盈
    fixed_enabled: false,
    fixed_percent: 8,
    scaled_enabled: true,
    scaled_profit_percent_mode: 'roe',
    scaled_levels: [
      { profit_percent: 5, close_percent: 25, move_stop_to_breakeven: false },
      { profit_percent: 8, close_percent: 25, move_stop_to_breakeven: true },
      { profit_percent: 12, close_percent: 100, move_stop_to_breakeven: false },
    ],
    atr_enabled: false,
    atr_multiplier_min: 2.5,
    atr_multiplier_max: 4.0,       // 与后端预设、kernel 默认一致
    atr_use_max_in_high_volatility: true,  // 高波动时用 Max 倍数，与止损宽容一致
    atr_high_volatility_threshold: 1.2,    // atr ≥ atrLong*1.2 视为高波动
    atr_period_btc_eth: 20,
    atr_period_altcoin: 14,
    resistance_enabled: false,
    resistance_buffer: 0.5,
    trailing_tp_enabled: false,
    trailing_tp_activate_profit_pct: 2,
    trailing_tp_retrace_pct: 1.5,
    trailing_tp_retrace_atr_mult: 0.5,
    trailing_tp_confirm_minutes: 0,
    trailing_tp_close_percent: 100,
    lock_profit_percent: 2.5,      // 达此盈利后由止损侧移动至盈亏平衡
  }

  const currentConfig = config || defaultConfig

  const updateField = <K extends keyof DynamicTakeProfitConfig>(
    key: K,
    value: DynamicTakeProfitConfig[K]
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

  const addScaledLevel = () => {
    const levels = currentConfig.scaled_levels || []
    const newLevel: ScaledTakeProfitLevel = {
      profit_percent: levels.length > 0 ? levels[levels.length - 1].profit_percent + 3 : 3,
      close_percent: 25,
      move_stop_to_breakeven: false,
    }
    updateField('scaled_levels', [...levels, newLevel])
  }

  const removeScaledLevel = (index: number) => {
    const levels = currentConfig.scaled_levels || []
    updateField('scaled_levels', levels.filter((_, i) => i !== index))
  }

  const updateScaledLevel = (index: number, field: keyof ScaledTakeProfitLevel, value: any) => {
    const levels = [...(currentConfig.scaled_levels || [])]
    levels[index] = { ...levels[index], [field]: value }
    updateField('scaled_levels', levels)
  }

  return (
    <div className="space-y-5">
      {/* Enable Toggle */}
      <div className="flex items-center justify-between p-4 rounded-xl shadow-lg" style={{ background: 'linear-gradient(135deg, #1a1d24 0%, #0f1115 100%)', border: '1px solid #2B3139' }}>
        <div className="flex items-center gap-3">
          <div className="p-2 rounded-lg" style={{ background: 'rgba(14, 203, 129, 0.1)' }}>
            <TrendingUp className="w-5 h-5" style={{ color: '#0ECB81' }} />
          </div>
          <span className="text-base font-semibold" style={{ color: '#EAECEF' }}>
            {t('enableDynamicTakeProfit')}
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
          <p className="text-xs px-1 py-2 rounded-lg" style={{ color: '#848E9C', background: 'rgba(14, 203, 129, 0.06)', border: '1px solid rgba(14, 203, 129, 0.2)' }}>
            {t('pricePercentNote')}
          </p>
          {/* Min hold + Min profit filter */}
          <div className="p-4 rounded-xl shadow-lg" style={{ background: 'linear-gradient(135deg, #1a1d24 0%, #0f1115 100%)', border: '1px solid #2B3139' }}>
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
            <div className="mt-3">
              <label className="text-xs block mb-1" style={{ color: '#848E9C' }}>{t('minProfitToAllowTP')}</label>
              <input
                type="number"
                min={0}
                max={20}
                step={0.5}
                value={currentConfig.min_profit_percent_to_allow_tp ?? 0}
                onChange={(e) => updateField('min_profit_percent_to_allow_tp', parseFloat(e.target.value) || 0)}
                disabled={disabled}
                className="w-20 rounded px-2 py-1 text-sm"
                style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
              />
              <span className="text-xs ml-1" style={{ color: '#848E9C' }}>%</span>
              <span className="text-[10px] ml-2" style={{ color: '#5E6673' }}>{t('minProfitToAllowTPDesc')}</span>
            </div>
          </div>

          {/* Fixed Take Profit */}
          <div className="p-4 rounded-xl shadow-lg" style={{ background: 'linear-gradient(135deg, #1a1d24 0%, #0f1115 100%)', border: currentConfig.fixed_enabled ? '2px solid #0ECB81' : '1px solid #2B3139' }}>
            <label className="flex items-center gap-3 cursor-pointer mb-4">
              <input
                type="checkbox"
                checked={currentConfig.fixed_enabled || false}
                onChange={(e) => updateField('fixed_enabled', e.target.checked)}
                disabled={disabled}
                className="w-5 h-5 accent-green-500 rounded"
              />
              <div className="p-2 rounded-lg" style={{ background: currentConfig.fixed_enabled ? 'rgba(14, 203, 129, 0.1)' : 'rgba(132, 142, 156, 0.1)' }}>
                <Target className="w-5 h-5" style={{ color: currentConfig.fixed_enabled ? '#0ECB81' : '#848E9C' }} />
              </div>
              <span className="text-base font-semibold" style={{ color: currentConfig.fixed_enabled ? '#EAECEF' : '#848E9C' }}>
                {t('fixedTakeProfit')}
              </span>
            </label>

            {currentConfig.fixed_enabled && (
              <div className="pl-2">
                <label className="block text-sm mb-2 font-medium" style={{ color: '#EAECEF' }}>
                  {t('fixedPercent')}
                </label>
                <p className="text-xs mb-3 leading-relaxed" style={{ color: '#848E9C' }}>
                  {t('fixedPercentDesc')}
                </p>
                <div className="flex items-center gap-3">
                  <input
                    type="range"
                    value={currentConfig.fixed_percent || 8}
                    onChange={(e) => updateField('fixed_percent', parseFloat(e.target.value))}
                    disabled={disabled}
                    min={1}
                    max={50}
                    step={0.5}
                    className="flex-1 h-2 accent-green-500"
                  />
                  <span className="w-20 text-center font-bold text-lg px-3 py-1 rounded-lg" style={{ color: '#0ECB81', background: 'rgba(14, 203, 129, 0.1)' }}>
                    {currentConfig.fixed_percent || 8}%
                  </span>
                </div>
              </div>
            )}
          </div>

          {/* Scaled Take Profit */}
          <div className="p-4 rounded-xl shadow-lg" style={{ background: 'linear-gradient(135deg, #1a1d24 0%, #0f1115 100%)', border: currentConfig.scaled_enabled ? '2px solid #0ECB81' : '1px solid #2B3139' }}>
            <label className="flex items-center gap-3 cursor-pointer mb-4">
              <input
                type="checkbox"
                checked={currentConfig.scaled_enabled || false}
                onChange={(e) => updateField('scaled_enabled', e.target.checked)}
                disabled={disabled}
                className="w-5 h-5 accent-green-500 rounded"
              />
              <div className="p-2 rounded-lg" style={{ background: currentConfig.scaled_enabled ? 'rgba(14, 203, 129, 0.1)' : 'rgba(132, 142, 156, 0.1)' }}>
                <Layers className="w-5 h-5" style={{ color: currentConfig.scaled_enabled ? '#0ECB81' : '#848E9C' }} />
              </div>
              <span className="text-base font-semibold" style={{ color: currentConfig.scaled_enabled ? '#EAECEF' : '#848E9C' }}>
                {t('scaledTakeProfit')}
              </span>
            </label>

            {currentConfig.scaled_enabled && (
              <div className="space-y-3 pl-2">
                <div className="flex items-center justify-between mb-3">
                  <span className="text-xs font-semibold" style={{ color: '#848E9C' }}>
                    {t('scaledLevels')}
                  </span>
                  <button
                    onClick={addScaledLevel}
                    disabled={disabled}
                    className="flex items-center gap-1 px-3 py-1.5 rounded-lg text-xs font-medium transition-all hover:scale-105"
                    style={{ background: 'rgba(14, 203, 129, 0.1)', color: '#0ECB81', border: '1px solid #0ECB81' }}
                  >
                    <Plus className="w-3 h-3" />
                    {t('addLevel')}
                  </button>
                </div>

                {(currentConfig.scaled_levels || []).map((level, index) => (
                  <div key={index} className="p-3 rounded-lg border border-gray-700 bg-black/30 space-y-3">
                    <div className="flex items-center justify-between">
                      <span className="text-xs font-semibold" style={{ color: '#0ECB81' }}>
                        层级 {index + 1}
                      </span>
                      {(currentConfig.scaled_levels?.length || 0) > 1 && (
                        <button
                          onClick={() => removeScaledLevel(index)}
                          disabled={disabled}
                          className="p-1 rounded hover:bg-red-500/20 transition-colors"
                        >
                          <Trash2 className="w-3 h-3" style={{ color: '#F6465D' }} />
                        </button>
                      )}
                    </div>

                    <div>
                      <label className="block text-xs mb-1.5 font-medium" style={{ color: '#EAECEF' }}>
                        {t('profitPercent')}
                      </label>
                      <p className="text-[10px] mb-2 leading-relaxed" style={{ color: '#848E9C' }}>
                        {t('profitPercentDesc')}
                      </p>
                      <div className="flex items-center gap-2">
                        <input
                          type="range"
                          value={level.profit_percent}
                          onChange={(e) => updateScaledLevel(index, 'profit_percent', parseFloat(e.target.value))}
                          disabled={disabled}
                          min={0}
                          max={50}
                          step={0.5}
                          className="flex-1 h-1.5 accent-green-500"
                        />
                        <span className="w-16 text-center font-mono text-xs font-bold px-2 py-1 rounded" style={{ color: '#0ECB81', background: 'rgba(14, 203, 129, 0.1)' }}>
                          {level.profit_percent}%
                        </span>
                      </div>
                    </div>

                    <div>
                      <label className="block text-xs mb-1.5 font-medium" style={{ color: '#EAECEF' }}>
                        {t('closePercent')}
                      </label>
                      <p className="text-[10px] mb-2 leading-relaxed" style={{ color: '#848E9C' }}>
                        {t('closePercentDesc')}
                      </p>
                      <div className="flex items-center gap-2">
                        <input
                          type="range"
                          value={level.close_percent}
                          onChange={(e) => updateScaledLevel(index, 'close_percent', parseFloat(e.target.value))}
                          disabled={disabled}
                          min={10}
                          max={100}
                          step={5}
                          className="flex-1 h-1.5 accent-blue-500"
                        />
                        <span className="w-16 text-center font-mono text-xs font-bold px-2 py-1 rounded" style={{ color: '#3B82F6', background: 'rgba(59, 130, 246, 0.1)' }}>
                          {level.close_percent}%
                        </span>
                      </div>
                    </div>

                    <label className="flex items-center gap-2 cursor-pointer p-2 rounded-lg hover:bg-white/5 transition-colors">
                      <input
                        type="checkbox"
                        checked={level.move_stop_to_breakeven || false}
                        onChange={(e) => updateScaledLevel(index, 'move_stop_to_breakeven', e.target.checked)}
                        disabled={disabled}
                        className="w-4 h-4 accent-yellow-500 rounded"
                      />
                      <Lock className="w-3 h-3" style={{ color: level.move_stop_to_breakeven ? '#F0B90B' : '#848E9C' }} />
                      <span className="text-xs" style={{ color: level.move_stop_to_breakeven ? '#F0B90B' : '#848E9C' }}>
                        {t('moveToBreakeven')}
                      </span>
                    </label>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Trailing / Pullback Take Profit */}
          <div className="p-4 rounded-xl shadow-lg" style={{ background: 'linear-gradient(135deg, #1a1d24 0%, #0f1115 100%)', border: currentConfig.trailing_tp_enabled ? '2px solid #0ECB81' : '1px solid #2B3139' }}>
            <label className="flex items-center gap-3 cursor-pointer mb-2">
              <input
                type="checkbox"
                checked={currentConfig.trailing_tp_enabled || false}
                onChange={(e) => updateField('trailing_tp_enabled', e.target.checked)}
                disabled={disabled}
                className="w-5 h-5 accent-green-500 rounded"
              />
              <span className="text-base font-semibold" style={{ color: currentConfig.trailing_tp_enabled ? '#EAECEF' : '#848E9C' }}>
                {t('trailingTP')}
              </span>
            </label>
            <p className="text-xs mb-4 pl-8 leading-relaxed" style={{ color: '#848E9C' }}>
              {t('trailingTPDesc')}
            </p>
            {currentConfig.trailing_tp_enabled && (
              <div className="space-y-3 pl-2 border-l-2 border-green-500/30">
                <div className="flex items-center gap-4 flex-wrap">
                  <div>
                    <label className="block text-xs mb-0.5" style={{ color: '#848E9C' }}>{t('trailingTPActivate')}</label>
                    <input
                      type="number"
                      min={0.5}
                      max={20}
                      step={0.5}
                      value={currentConfig.trailing_tp_activate_profit_pct ?? 2}
                      onChange={(e) => updateField('trailing_tp_activate_profit_pct', parseFloat(e.target.value) || 2)}
                      disabled={disabled}
                      className="w-20 rounded px-2 py-1 text-sm"
                      style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                    />
                    <span className="text-xs ml-1" style={{ color: '#848E9C' }}>%</span>
                  </div>
                  <div>
                    <label className="block text-xs mb-0.5" style={{ color: '#848E9C' }}>{t('trailingTPRetrace')}</label>
                    <input
                      type="number"
                      min={0.5}
                      max={10}
                      step={0.25}
                      value={currentConfig.trailing_tp_retrace_pct ?? 1.5}
                      onChange={(e) => updateField('trailing_tp_retrace_pct', parseFloat(e.target.value) || 1.5)}
                      disabled={disabled}
                      className="w-20 rounded px-2 py-1 text-sm"
                      style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                    />
                    <span className="text-xs ml-1" style={{ color: '#848E9C' }}>%</span>
                  </div>
                  <div>
                    <label className="block text-xs mb-0.5" style={{ color: '#848E9C' }}>{t('trailingTPRetraceATR')}</label>
                    <input
                      type="number"
                      min={0.2}
                      max={2}
                      step={0.1}
                      value={currentConfig.trailing_tp_retrace_atr_mult ?? 0.5}
                      onChange={(e) => updateField('trailing_tp_retrace_atr_mult', parseFloat(e.target.value) || 0.5)}
                      disabled={disabled}
                      className="w-20 rounded px-2 py-1 text-sm"
                      style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                    />
                  </div>
                  <div>
                    <label className="block text-xs mb-0.5" style={{ color: '#848E9C' }}>{t('trailingTPConfirmMin')}</label>
                    <input
                      type="number"
                      min={0}
                      max={30}
                      step={1}
                      value={currentConfig.trailing_tp_confirm_minutes ?? 0}
                      onChange={(e) => updateField('trailing_tp_confirm_minutes', parseFloat(e.target.value) || 0)}
                      disabled={disabled}
                      className="w-20 rounded px-2 py-1 text-sm"
                      style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                    />
                    <span className="text-xs ml-1" style={{ color: '#848E9C' }}>{language === 'zh' ? '分钟' : 'min'}</span>
                  </div>
                  <div>
                    <label className="block text-xs mb-0.5" style={{ color: '#848E9C' }}>{t('trailingTPClosePct')}</label>
                    <input
                      type="number"
                      min={10}
                      max={100}
                      step={10}
                      value={currentConfig.trailing_tp_close_percent ?? 100}
                      onChange={(e) => updateField('trailing_tp_close_percent', parseFloat(e.target.value) || 100)}
                      disabled={disabled}
                      className="w-20 rounded px-2 py-1 text-sm"
                      style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                    />
                    <span className="text-xs ml-1" style={{ color: '#848E9C' }}>%</span>
                  </div>
                  <div>
                    <label className="block text-xs mb-0.5" style={{ color: '#848E9C' }}>{t('trailingTPActivateAltcoin')}</label>
                    <input
                      type="number"
                      min={0}
                      max={20}
                      step={0.5}
                      placeholder="0=同主"
                      value={currentConfig.trailing_tp_activate_profit_pct_altcoin ?? ''}
                      onChange={(e) => updateField('trailing_tp_activate_profit_pct_altcoin', e.target.value === '' ? undefined : parseFloat(e.target.value) || 0)}
                      disabled={disabled}
                      className="w-20 rounded px-2 py-1 text-sm"
                      style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                    />
                    <span className="text-xs ml-1" style={{ color: '#848E9C' }}>%</span>
                  </div>
                  <div>
                    <label className="block text-xs mb-0.5" style={{ color: '#848E9C' }}>{t('trailingTPRetraceAltcoin')}</label>
                    <input
                      type="number"
                      min={0}
                      max={10}
                      step={0.25}
                      placeholder="0=同主"
                      value={currentConfig.trailing_tp_retrace_pct_altcoin ?? ''}
                      onChange={(e) => updateField('trailing_tp_retrace_pct_altcoin', e.target.value === '' ? undefined : parseFloat(e.target.value) || 0)}
                      disabled={disabled}
                      className="w-20 rounded px-2 py-1 text-sm"
                      style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                    />
                    <span className="text-xs ml-1" style={{ color: '#848E9C' }}>%</span>
                  </div>
                </div>
              </div>
            )}
          </div>

          {/* ATR Take Profit - Dynamic Range */}
          <div className="p-4 rounded-xl shadow-lg" style={{ background: 'linear-gradient(135deg, #1a1d24 0%, #0f1115 100%)', border: currentConfig.atr_enabled ? '2px solid #0ECB81' : '1px solid #2B3139' }}>
            <label className="flex items-center gap-3 cursor-pointer mb-4">
              <input
                type="checkbox"
                checked={currentConfig.atr_enabled || false}
                onChange={(e) => updateField('atr_enabled', e.target.checked)}
                disabled={disabled}
                className="w-5 h-5 accent-green-500 rounded"
              />
              <div className="p-2 rounded-lg" style={{ background: currentConfig.atr_enabled ? 'rgba(14, 203, 129, 0.1)' : 'rgba(132, 142, 156, 0.1)' }}>
                <BarChart3 className="w-5 h-5" style={{ color: currentConfig.atr_enabled ? '#0ECB81' : '#848E9C' }} />
              </div>
              <span className="text-base font-semibold" style={{ color: currentConfig.atr_enabled ? '#EAECEF' : '#848E9C' }}>
                {t('atrTakeProfit')}
              </span>
            </label>

            {currentConfig.atr_enabled && (
              <div className="space-y-4 pl-2">
                <div className="p-3 rounded-lg" style={{ background: 'rgba(14, 203, 129, 0.05)', border: '1px solid rgba(14, 203, 129, 0.2)' }}>
                  <label className="block text-xs mb-2 font-semibold" style={{ color: '#0ECB81' }}>
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
                          value={currentConfig.atr_multiplier_min || 2.5}
                          onChange={(e) => updateField('atr_multiplier_min', parseFloat(e.target.value))}
                          disabled={disabled}
                          min={1}
                          max={10}
                          step={0.1}
                          className="flex-1 h-1.5 accent-green-500"
                        />
                        <span className="w-16 text-center font-mono text-xs font-bold px-2 py-1 rounded" style={{ color: '#0ECB81', background: 'rgba(14, 203, 129, 0.1)' }}>
                          {currentConfig.atr_multiplier_min || 2.5}x
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
                          value={currentConfig.atr_multiplier_max ?? 4.0}
                          onChange={(e) => updateField('atr_multiplier_max', parseFloat(e.target.value))}
                          disabled={disabled}
                          min={1}
                          max={10}
                          step={0.1}
                          className="flex-1 h-1.5 accent-green-500"
                        />
                        <span className="w-16 text-center font-mono text-xs font-bold px-2 py-1 rounded" style={{ color: '#0ECB81', background: 'rgba(14, 203, 129, 0.1)' }}>
                          {currentConfig.atr_multiplier_max ?? 4.0}x
                        </span>
                      </div>
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-3 flex-wrap mt-2">
                  <label className="flex items-center gap-2 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={currentConfig.atr_use_max_in_high_volatility || false}
                      onChange={(e) => updateField('atr_use_max_in_high_volatility', e.target.checked)}
                      disabled={disabled}
                      className="w-4 h-4 accent-green-500 rounded"
                    />
                    <span className="text-xs" style={{ color: '#EAECEF' }}>{t('atrUseMaxInHighVol')}</span>
                  </label>
                  {currentConfig.atr_use_max_in_high_volatility && (
                    <>
                      <span className="text-xs" style={{ color: '#848E9C' }}>{t('atrHighVolThreshold')}</span>
                      <input
                        type="number"
                        min={1}
                        max={2}
                        step={0.1}
                        value={currentConfig.atr_high_volatility_threshold ?? 1.2}
                        onChange={(e) => updateField('atr_high_volatility_threshold', parseFloat(e.target.value) || 1.2)}
                        disabled={disabled}
                        className="w-14 rounded px-2 py-1 text-sm"
                        style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                      />
                    </>
                  )}
                </div>

                <div className="p-3 rounded-lg" style={{ background: 'rgba(240, 185, 11, 0.05)', border: '1px solid rgba(240, 185, 11, 0.2)' }}>
                  <label className="block text-xs mb-2 font-semibold" style={{ color: '#F0B90B' }}>
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
                        style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#F0B90B' }}
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
                        style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#F0B90B' }}
                      />
                    </div>
                  </div>
                </div>
              </div>
            )}
          </div>

          {/* Resistance Take Profit */}
          <div className="p-4 rounded-xl shadow-lg" style={{ background: 'linear-gradient(135deg, #1a1d24 0%, #0f1115 100%)', border: currentConfig.resistance_enabled ? '2px solid #0ECB81' : '1px solid #2B3139' }}>
            <label className="flex items-center gap-3 cursor-pointer mb-4">
              <input
                type="checkbox"
                checked={currentConfig.resistance_enabled || false}
                onChange={(e) => updateField('resistance_enabled', e.target.checked)}
                disabled={disabled}
                className="w-5 h-5 accent-green-500 rounded"
              />
              <div className="p-2 rounded-lg" style={{ background: currentConfig.resistance_enabled ? 'rgba(14, 203, 129, 0.1)' : 'rgba(132, 142, 156, 0.1)' }}>
                <TrendingUp className="w-5 h-5" style={{ color: currentConfig.resistance_enabled ? '#0ECB81' : '#848E9C' }} />
              </div>
              <span className="text-base font-semibold" style={{ color: currentConfig.resistance_enabled ? '#EAECEF' : '#848E9C' }}>
                {t('resistanceTakeProfit')}
              </span>
            </label>

            {currentConfig.resistance_enabled && (
              <div className="pl-2">
                <label className="block text-xs mb-1.5 font-medium" style={{ color: '#EAECEF' }}>
                  {t('resistanceBuffer')}
                </label>
                <p className="text-[10px] mb-3 leading-relaxed" style={{ color: '#848E9C' }}>
                  {t('resistanceBufferDesc')}
                </p>
                <div className="flex items-center gap-3">
                  <input
                    type="range"
                    value={currentConfig.resistance_buffer || 0.5}
                    onChange={(e) => updateField('resistance_buffer', parseFloat(e.target.value))}
                    disabled={disabled}
                    min={0.1}
                    max={2}
                    step={0.1}
                    className="flex-1 h-1.5 accent-green-500"
                  />
                  <span className="w-16 text-center font-mono text-xs font-bold px-2 py-1 rounded" style={{ color: '#0ECB81', background: 'rgba(14, 203, 129, 0.1)' }}>
                    {currentConfig.resistance_buffer || 0.5}%
                  </span>
                </div>
              </div>
            )}
          </div>

          {/* Common Settings - Lock Profit */}
          <div className="p-4 rounded-xl shadow-lg" style={{ background: 'linear-gradient(135deg, #2a2410 0%, #1a1808 100%)', border: '1px solid #F0B90B' }}>
            <div className="flex items-center gap-2 mb-3">
              <Lock className="w-5 h-5" style={{ color: '#F0B90B' }} />
              <label className="text-sm font-semibold" style={{ color: '#EAECEF' }}>
                {t('lockProfitPercent')}
              </label>
            </div>
            <div className="mb-4">
              <label className="text-xs block mb-1" style={{ color: '#848E9C' }}>{t('scaledProfitMode')}</label>
              <select
                value={currentConfig.scaled_profit_percent_mode || 'price'}
                onChange={(e) => updateField('scaled_profit_percent_mode', e.target.value)}
                disabled={disabled}
                className="rounded px-2 py-1 text-sm"
                style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
              >
                <option value="price">price</option>
                <option value="roe">roe</option>
              </select>
              <span className="text-[10px] ml-2" style={{ color: '#5E6673' }}>{t('scaledProfitModeDesc')}</span>
            </div>
            <p className="text-xs mb-3 leading-relaxed" style={{ color: '#848E9C' }}>
              {t('lockProfitPercentDesc')}
            </p>
            <div className="flex items-center gap-3">
              <input
                type="range"
                value={currentConfig.lock_profit_percent || 5}
                onChange={(e) => updateField('lock_profit_percent', parseFloat(e.target.value))}
                disabled={disabled}
                min={1}
                max={20}
                step={0.5}
                className="flex-1 h-2 accent-yellow-500"
              />
              <span className="w-20 text-center font-bold text-lg px-3 py-1 rounded-lg" style={{ color: '#F0B90B', background: 'rgba(240, 185, 11, 0.1)' }}>
                {currentConfig.lock_profit_percent || 5}%
              </span>
            </div>
          </div>
        </>
      )}
    </div>
  )
}
