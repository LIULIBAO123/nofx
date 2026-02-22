# NOFX Meridian Design System

基于 Meridian Financial Analytics Dashboard 的现代化设计系统，采用 Cyan/Teal 主题色。

## 🎨 设计理念

- **色彩空间**: 使用 OKLCH 色彩空间，确保视觉感知的一致性
- **主题色**: Cyan/Teal (#00D9D9) 作为主色调，保留 Gold (#F0B90B) 作为备选
- **玻璃态**: Glassmorphism 效果，带有模糊和半透明
- **微交互**: 流畅的动画和过渡效果
- **排版**: DM Sans (正文) + Outfit (标题) + JetBrains Mono (数字)

## 📦 核心组件

### KpiCard - KPI 指标卡片

```tsx
import { KpiCard } from '@/components/ui';
import { TrendingUp } from 'lucide-react';

<KpiCard
  label="总资产"
  value="$2,847,392"
  change={12.5}
  changeLabel="vs 昨日"
  icon={TrendingUp}
  trend="up"
/>
```

**Props:**
- `label`: 标签文本
- `value`: 显示值
- `change?`: 变化百分比
- `changeLabel?`: 变化说明
- `icon?`: Lucide 图标组件
- `trend?`: 'up' | 'down' | 'neutral'
- `loading?`: 加载状态

### SectionPanel - 区域面板

```tsx
import { SectionPanel, SectionHeader } from '@/components/ui';

<SectionPanel>
  <SectionHeader
    title="组合表现"
    subtitle="过去 30 天的收益趋势"
    action={<button>查看详情</button>}
  />
  {/* 内容 */}
</SectionPanel>
```

### MiniSparkline - 迷你折线图

```tsx
import { MiniSparkline } from '@/components/ui';

<MiniSparkline
  data={[10, 20, 15, 30, 25]}
  width={80}
  height={24}
  color="oklch(0.55 0.18 180)"
/>
```

### ChartTooltipContent - 图表提示框

```tsx
import { ChartTooltipContent } from '@/components/ui';

<Tooltip content={<ChartTooltipContent />} />
```

### GrainOverlay - 纹理叠加层

```tsx
import { GrainOverlay } from '@/components/ui';

<div className="relative">
  <GrainOverlay />
  {/* 你的内容 */}
</div>
```

## 🎨 CSS 工具类

### 卡片样式

```tsx
// 现代卡片 - 带玻璃态效果
<div className="modern-card p-6">
  内容
</div>

// 统计卡片 - 带悬停动画
<div className="stat-card-modern p-6">
  内容
</div>

// 传统卡片
<div className="binance-card p-6">
  内容
</div>
```

### 按钮样式

```tsx
// Teal 渐变按钮（主题色）
<button className="px-6 py-3 rounded-lg btn-teal-gradient">
  主要操作
</button>

// Gold 渐变按钮（备选）
<button className="px-6 py-3 rounded-lg btn-gold-gradient">
  次要操作
</button>

// 成功按钮
<button className="btn-success">成功</button>

// 危险按钮
<button className="btn-danger">危险</button>

// 轮廓按钮
<button className="btn-outline">轮廓</button>
```

### 徽章样式

```tsx
// Teal 徽章
<span className="badge-modern badge-teal">
  Teal Badge
</span>

// Gold 徽章
<span className="badge-modern badge-gold">
  Gold Badge
</span>

// 传统徽章
<span className="badge-green">成功</span>
<span className="badge-red">失败</span>
<span className="badge-yellow">警告</span>
```

### 输入框样式

```tsx
// 现代输入框
<input className="modern-input" />

// 传统输入框
<input className="premium-input" />
```

### 文字渐变

```tsx
// Teal 渐变文字
<h1 className="text-gradient-teal">
  标题文字
</h1>

// Gold 渐变文字
<h1 className="text-gradient-gold">
  标题文字
</h1>
```

### 标签页

```tsx
<div className="flex gap-2">
  <button className="modern-tab active">标签 1</button>
  <button className="modern-tab">标签 2</button>
  <button className="modern-tab">标签 3</button>
</div>
```

### 滚动条

```tsx
<div className="modern-scrollbar overflow-auto">
  内容
</div>
```

## 🎯 设计令牌

### 颜色

```typescript
import { COLORS, SEMANTIC } from '@/lib/designTokens';

// 主题色
COLORS.teal[500]      // oklch(0.55 0.18 180)
COLORS.azure[500]     // oklch(0.60 0.18 240)
COLORS.amber[500]     // oklch(0.70 0.18 75)
COLORS.emerald[500]   // oklch(0.65 0.18 145)
COLORS.rose[500]      // oklch(0.60 0.22 12)

// 语义色
SEMANTIC.financial.gain    // 盈利色
SEMANTIC.financial.loss    // 亏损色
SEMANTIC.text.primary      // 主文本
SEMANTIC.text.secondary    // 次要文本
```

### 阴影

```typescript
import { SHADOWS } from '@/lib/designTokens';

SHADOWS.card          // 卡片阴影
SHADOWS.cardHover     // 卡片悬停阴影
SHADOWS.glowTeal      // Teal 发光效果
SHADOWS.glowAmber     // Amber 发光效果
```

### 动画

```typescript
import { ANIMATIONS } from '@/lib/designTokens';

ANIMATIONS.spring     // 弹簧动画
ANIMATIONS.easeOut    // 缓出动画
ANIMATIONS.duration.fast    // 150ms
ANIMATIONS.duration.normal  // 250ms
```

## 🛠️ 工具函数

```typescript
import {
  formatCompactNumber,
  formatPercentage,
  getTrend,
  getFinancialColor,
  withAlpha,
  coloredShadow,
} from '@/lib/designUtils';

// 格式化数字
formatCompactNumber(1234567)  // "1.2M"

// 格式化百分比
formatPercentage(12.5)        // "+12.50%"

// 获取趋势
getTrend(5)                   // "up"

// 获取金融颜色
getFinancialColor(100)        // emerald-500

// 添加透明度
withAlpha('oklch(0.55 0.18 180)', 0.5)

// 彩色阴影
coloredShadow('oklch(0.55 0.18 180)', 'md')
```

## 🎬 动画类

```tsx
// 淡入
<div className="animate-fade-in">内容</div>

// 滑入
<div className="animate-slide-in">内容</div>

// 缩放进入
<div className="animate-scale-in">内容</div>

// 发光脉冲
<div className="animate-glow-pulse">内容</div>

// 骨架屏
<div className="skeleton-modern h-8 w-32" />
```

## 📱 响应式

设计系统内置响应式支持：

```tsx
<div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
  {/* 移动端 1 列，平板 2 列，桌面 4 列 */}
</div>
```

## 🌈 主题切换

系统支持两套主题：

1. **Meridian Cyan** (默认) - 现代 Cyan/Teal 主题
2. **NOFX Gold** (备选) - 传统金色主题

切换方式：使用对应的 CSS 类名（`btn-teal-gradient` vs `btn-gold-gradient`）

## 📖 示例页面

查看 `DesignSystemDemo.tsx` 获取完整的使用示例。

## 🚀 快速开始

1. 导入组件：
```tsx
import { KpiCard, SectionPanel } from '@/components/ui';
```

2. 使用设计令牌：
```tsx
import { COLORS } from '@/lib/designTokens';
```

3. 应用样式类：
```tsx
<div className="modern-card p-6">
  <h2 className="text-gradient-teal">标题</h2>
</div>
```

## 🎨 设计原则

1. **一致性**: 使用统一的设计令牌和组件
2. **可访问性**: 确保足够的对比度和可读性
3. **性能**: 使用 `will-change` 优化动画性能
4. **响应式**: 移动优先的设计方法
5. **渐进增强**: 支持 `prefers-reduced-motion`

## 📝 注意事项

- 所有颜色使用 OKLCH 色彩空间，确保视觉一致性
- 动画使用 `cubic-bezier` 缓动函数，提供流畅体验
- 玻璃态效果需要 `backdrop-filter` 支持
- 建议在现代浏览器中使用以获得最佳效果







