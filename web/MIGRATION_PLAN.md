# 🎨 Meridian 设计系统迁移计划

## 📋 页面清单（15个）

### 核心功能页面（优先级：高）
1. ✅ **AITradersPage.tsx** - AI 交易员页面
2. ✅ **BacktestPage.tsx** - 回测页面
3. ✅ **TraderDashboardPage.tsx** - 交易员仪表盘
4. ✅ **DataPage.tsx** - 数据页面

### 市场和策略页面（优先级：中）
5. ⏳ **StrategyMarketPage.tsx** - 策略市场
6. ⏳ **StrategyStudioPage.tsx** - 策略工作室
7. ⏳ **CompetitionPage.tsx** - 竞赛页面
8. ⏳ **DebateArenaPage.tsx** - 辩论竞技场

### 用户相关页面（优先级：中）
9. ⏳ **LoginPage.tsx** - 登录页面
10. ⏳ **RegisterPage.tsx** - 注册页面
11. ⏳ **ResetPasswordPage.tsx** - 重置密码页面

### 其他页面（优先级：低）
12. ⏳ **LandingPage.tsx** - 落地页
13. ⏳ **FAQPage.tsx** - FAQ 页面
14. ⏳ **PageNotFound.tsx** - 404 页面
15. ⏳ **WhitelistFullPage.tsx** - 白名单页面

## 🎯 迁移策略

### 第一阶段：核心功能页面（4个）
重点更新数据密集型页面，应用 KpiCard 和现代卡片样式

### 第二阶段：市场和策略页面（4个）
更新交互密集型页面，应用按钮和表单样式

### 第三阶段：用户相关页面（3个）
更新认证相关页面，保持简洁专业

### 第四阶段：其他页面（4个）
更新辅助页面，统一整体风格

## 📝 迁移检查清单

每个页面需要完成：
- [ ] 添加 GrainOverlay 纹理
- [ ] 更新背景为 bg-slate-950
- [ ] 替换卡片样式为 modern-card
- [ ] 更新按钮为 btn-teal-gradient
- [ ] 更新标题为 text-gradient-teal
- [ ] 应用 KpiCard 组件（如适用）
- [ ] 应用 SectionPanel 组件（如适用）
- [ ] 更新输入框为 modern-input
- [ ] 更新徽章为 badge-modern
- [ ] 测试响应式布局

## 🚀 开始迁移

当前状态：准备开始第一阶段







