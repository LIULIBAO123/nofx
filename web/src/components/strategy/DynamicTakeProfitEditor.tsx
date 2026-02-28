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
      profitPercentDesc: { zh: '当盈利达到此百分比时触发（按价格相对入场价的变动，非保证金收益率；带杠杆时页面「当前盈亏」会高于价格变动）', en: 'Trigger when profit reaches this % (price move from entry, not margin return; with leverage, dashboard P/L % is higher than price %).' },
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
      lockProfitPercentDesc: { zh: '达到此盈利后移动止损到盈亏平衡点', en: 'Move stop to breakeven after this profit' },
      minHoldMinutes: { zh: '最小持仓时间（分钟）', en: 'Min hold (minutes)' },
      minHoldMinutesDesc: { zh: '未满此时间不触发动态止盈，避免开仓即止盈。0=不限制', en: 'Do not trigger before this many minutes; 0=no limit' },
    }
    return translations[key]?.[language] || key
  }

  const defaultConfig: DynamicTakeProfitConfig = {
    enabled: true,
    min_hold_minutes: 5,
    fixed_enabled: false,
    fixed_percent: 8,
    scaled_enabled: true,
    scaled_levels: [
      { profit_percent: 3, close_percent: 25, move_stop_to_breakeven: false },
      { profit_percent: 6, close_percent: 25, move_stop_to_breakeven: true },
      { profit_percent: 10, close_percent: 50, move_stop_to_breakeven: false },
    ],
    atr_enabled: false,
    atr_multiplier_min: 2.5,
    atr_multiplier_max: 6.0,
    atr_period_btc_eth: 20,
    atr_period_altcoin: 14,
    resistance_enabled: false,
    resistance_buffer: 0.5,
    lock_profit_percent: 5,
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
          {/* Min hold time */}
          <div className="p-4 rounded-xl shadow-lg" style={{ background: 'linear-gradient(135deg, #1a1d24 0%, #0f1115 100%)', border: '1px solid #2B3139' }}>
            <label className="text-xs block mb-1" style={{ color: '#848E9C' }}>{t('minHoldMinutes')}</label>
            <div className="flex items-center gap-2 flex-wrap">
              <input
                type="number"
                min={0}
                max={120}
                step={1}
                value={currentConfig.min_hold_minutes ?? 5}
                onChange={(e) => updateField('min_hold_minutes', parseFloat(e.target.value) || 0)}
                disabled={disabled}
                className="w-16 rounded px-2 py-1 text-sm"
                style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
              />
              <span className="text-xs" style={{ color: '#848E9C' }}>{language === 'zh' ? '分钟' : 'min'}</span>
              <span className="text-[10px]" style={{ color: '#5E6673' }}>{t('minHoldMinutesDesc')}</span>
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
                          value={currentConfig.atr_multiplier_max || 5.0}
                          onChange={(e) => updateField('atr_multiplier_max', parseFloat(e.target.value))}
                          disabled={disabled}
                          min={1}
                          max={10}
                          step={0.1}
                          className="flex-1 h-1.5 accent-green-500"
                        />
                        <span className="w-16 text-center font-mono text-xs font-bold px-2 py-1 rounded" style={{ color: '#0ECB81', background: 'rgba(14, 203, 129, 0.1)' }}>
                          {currentConfig.atr_multiplier_max || 5.0}x
                        </span>
                      </div>
                    </div>
                  </div>
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
