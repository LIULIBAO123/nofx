import type { DirectionPoolConfig, MultilayerFilterConfig } from '../../types'

interface DirectionPoolEditorProps {
  config: MultilayerFilterConfig | null | undefined
  onChange: (multilayerFilter: MultilayerFilterConfig) => void
  disabled?: boolean
  language: string
}

const defaultDirectionPool = (): DirectionPoolConfig => ({
  min_strength_pct: 50,
  min_strength_pct_to_open: 55,
  single_side_only: true,
  sort_by_strength: true,
  filter_pool_by_layer2: true,
  enable_strength_smoothing: true,
  strength_smoothing_weight: 0.7,
  strength_bonus_cap: 10,
  reli_bonus_cap: 8,
})

export function DirectionPoolEditor({
  config,
  onChange,
  disabled,
  language,
}: DirectionPoolEditorProps) {
  const dp = config?.direction_pool ?? defaultDirectionPool()
  const t = (key: string) => {
    const m: Record<string, Record<string, string>> = {
      title: { zh: '方向池', en: 'Direction Pool' },
      titleDesc: { zh: '多层过滤下的多/空池划分与开仓优先级', en: 'Long/Short pool and open priority under multilayer filter' },
      minStrength: { zh: '最低强度 (%)', en: 'Min strength (%)' },
      minStrengthDesc: { zh: '仅 StrengthPct ≥ 此值的标的进池；0 = 不过滤', en: 'Only symbols with StrengthPct ≥ this enter pool; 0 = no filter' },
      minStrengthToOpen: { zh: '开仓最低强度 (%)', en: 'Min strength to open (%)' },
      minStrengthToOpenDesc: { zh: '三条件共振后仅当方向池中该标的 StrengthPct ≥ 此值才允许开仓；0 = 不额外过滤（减少弱信号开仓、方向选错）', en: 'Only open when pool StrengthPct ≥ this after 3-way resonance; 0 = no extra filter (reduces weak-signal entries)' },
      singleSideOnly: { zh: '单侧归属', en: 'Single side only' },
      singleSideOnlyDesc: { zh: '每标的只归入多空池中强度更高的一侧', en: 'Each symbol in the side with higher strength only' },
      sortByStrength: { zh: '按强度排序', en: 'Sort by strength' },
      sortByStrengthDesc: { zh: '池内与待提交列表按 StrengthPct 降序，优先开高强度', en: 'Pool and to-submit list by StrengthPct desc' },
      filterByLayer2: { zh: '仅 Layer2 达标进池', en: 'Filter pool by Layer2' },
      filterByLayer2Desc: { zh: '仅因子数/可靠度/入场信心达标的标的进入方向池', en: 'Only symbols passing Layer2 enter direction pool' },
      strengthSmoothing: { zh: '强度平滑', en: 'Strength smoothing' },
      strengthSmoothingDesc: { zh: '对 StrengthPct 做短时平滑，减少周期间抖动', en: 'Smooth StrengthPct across cycles to reduce jitter' },
      smoothingWeight: { zh: '平滑权重', en: 'Smoothing weight' },
      smoothingWeightDesc: { zh: 'smoothed = weight×上次 + (1-weight)×本次，建议 0.6~0.8', en: 'smoothed = weight×last + (1-weight)×current, 0.6~0.8' },
      strengthBonusCap: { zh: '强度加成上限', en: 'Strength bonus cap' },
      strengthBonusCapDesc: { zh: '补强数据对 StrengthPct 的加分上限；0 = 不设限', en: 'Max bonus to StrengthPct from data; 0 = no cap' },
      reliBonusCap: { zh: '可靠度加成上限', en: 'Reli bonus cap' },
      reliBonusCapDesc: { zh: '补强数据对 ReliabilityPct 的加分上限；0 = 不设限', en: 'Max bonus to ReliabilityPct; 0 = no cap' },
    }
    return m[key]?.[language] ?? key
  }

  const update = (patch: Partial<DirectionPoolConfig>) => {
    const next = { ...config, direction_pool: { ...dp, ...patch } } as MultilayerFilterConfig
    if (!next.enabled) next.enabled = true
    onChange(next)
  }

  return (
    <div className="space-y-4">
      <p className="text-xs text-[#848E9C]">{t('titleDesc')}</p>
      <div className="space-y-2">
        <label className="flex items-center gap-2 text-sm text-[#EAECEF]">
          <input
            type="number"
            min={0}
            max={100}
            step={1}
            value={dp.min_strength_pct ?? 0}
            onChange={(e) => update({ min_strength_pct: Number(e.target.value) })}
            disabled={disabled}
            className="w-20 rounded border border-[#2B3139] bg-[#1E2329] px-2 py-1 text-sm"
          />
          {t('minStrength')}
        </label>
        <p className="text-xs text-[#5E6673]">{t('minStrengthDesc')}</p>
      </div>
      <div className="space-y-2">
        <label className="flex items-center gap-2 text-sm text-[#EAECEF]">
          <input
            type="number"
            min={0}
            max={100}
            step={1}
            value={dp.min_strength_pct_to_open ?? 0}
            onChange={(e) => update({ min_strength_pct_to_open: Number(e.target.value) })}
            disabled={disabled}
            className="w-20 rounded border border-[#2B3139] bg-[#1E2329] px-2 py-1 text-sm"
          />
          {t('minStrengthToOpen')}
        </label>
        <p className="text-xs text-[#5E6673]">{t('minStrengthToOpenDesc')}</p>
      </div>
      <label className="flex items-center gap-2 cursor-pointer text-sm text-[#EAECEF]">
        <input
          type="checkbox"
          checked={dp.single_side_only ?? false}
          onChange={(e) => update({ single_side_only: e.target.checked })}
          disabled={disabled}
          className="rounded border-[#2B3139] bg-[#1E2329] text-[#F0B90B]"
        />
        {t('singleSideOnly')}
      </label>
      <p className="text-xs text-[#5E6673]">{t('singleSideOnlyDesc')}</p>
      <label className="flex items-center gap-2 cursor-pointer text-sm text-[#EAECEF]">
        <input
          type="checkbox"
          checked={dp.sort_by_strength ?? false}
          onChange={(e) => update({ sort_by_strength: e.target.checked })}
          disabled={disabled}
          className="rounded border-[#2B3139] bg-[#1E2329] text-[#F0B90B]"
        />
        {t('sortByStrength')}
      </label>
      <p className="text-xs text-[#5E6673]">{t('sortByStrengthDesc')}</p>
      <label className="flex items-center gap-2 cursor-pointer text-sm text-[#EAECEF]">
        <input
          type="checkbox"
          checked={dp.filter_pool_by_layer2 ?? false}
          onChange={(e) => update({ filter_pool_by_layer2: e.target.checked })}
          disabled={disabled}
          className="rounded border-[#2B3139] bg-[#1E2329] text-[#F0B90B]"
        />
        {t('filterByLayer2')}
      </label>
      <p className="text-xs text-[#5E6673]">{t('filterByLayer2Desc')}</p>
      <label className="flex items-center gap-2 cursor-pointer text-sm text-[#EAECEF]">
        <input
          type="checkbox"
          checked={dp.enable_strength_smoothing ?? false}
          onChange={(e) => update({ enable_strength_smoothing: e.target.checked })}
          disabled={disabled}
          className="rounded border-[#2B3139] bg-[#1E2329] text-[#F0B90B]"
        />
        {t('strengthSmoothing')}
      </label>
      <p className="text-xs text-[#5E6673]">{t('strengthSmoothingDesc')}</p>
      {(dp.enable_strength_smoothing ?? false) && (
        <div className="pl-4 space-y-1">
          <label className="flex items-center gap-2 text-sm text-[#EAECEF]">
            <input
              type="number"
              min={0}
              max={1}
              step={0.1}
              value={dp.strength_smoothing_weight ?? 0.7}
              onChange={(e) => update({ strength_smoothing_weight: Number(e.target.value) })}
              disabled={disabled}
              className="w-20 rounded border border-[#2B3139] bg-[#1E2329] px-2 py-1 text-sm"
            />
            {t('smoothingWeight')}
          </label>
          <p className="text-xs text-[#5E6673]">{t('smoothingWeightDesc')}</p>
        </div>
      )}
      <div className="space-y-2">
        <label className="flex items-center gap-2 text-sm text-[#EAECEF]">
          <input
            type="number"
            min={0}
            step={1}
            value={dp.strength_bonus_cap ?? 0}
            onChange={(e) => update({ strength_bonus_cap: Number(e.target.value) })}
            disabled={disabled}
            className="w-20 rounded border border-[#2B3139] bg-[#1E2329] px-2 py-1 text-sm"
          />
          {t('strengthBonusCap')}
        </label>
        <p className="text-xs text-[#5E6673]">{t('strengthBonusCapDesc')}</p>
      </div>
      <div className="space-y-2">
        <label className="flex items-center gap-2 text-sm text-[#EAECEF]">
          <input
            type="number"
            min={0}
            step={1}
            value={dp.reli_bonus_cap ?? 0}
            onChange={(e) => update({ reli_bonus_cap: Number(e.target.value) })}
            disabled={disabled}
            className="w-20 rounded border border-[#2B3139] bg-[#1E2329] px-2 py-1 text-sm"
          />
          {t('reliBonusCap')}
        </label>
        <p className="text-xs text-[#5E6673]">{t('reliBonusCapDesc')}</p>
      </div>
    </div>
  )
}
