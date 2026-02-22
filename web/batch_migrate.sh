#!/bin/bash
# 批量替换所有页面中的 Binance 风格为 Meridian 风格

# 要处理的文件列表
files=(
  "src/components/RegisterPage.tsx"
  "src/components/ResetPasswordPage.tsx"
  "src/components/WhitelistFullPage.tsx"
  "src/components/BacktestPage.tsx"
  "src/pages/StrategyStudioPage.tsx"
  "src/pages/StrategyMarketPage.tsx"
  "src/pages/LandingPage.tsx"
  "src/pages/FAQPage.tsx"
  "src/pages/DebateArenaPage.tsx"
)

# 颜色替换映射
declare -A replacements=(
  ["nofx-gold"]="teal-400"
  ["text-nofx-gold"]="text-teal-400"
  ["bg-nofx-gold"]="bg-teal-500"
  ["border-nofx-gold"]="border-teal-500"
  ["hover:text-nofx-gold"]="hover:text-teal-400"
  ["hover:bg-nofx-gold"]="hover:bg-teal-500"
  ["focus:border-nofx-gold"]="focus:border-teal-500"
  ["ring-nofx-gold"]="ring-teal-500"
  ["#F0B90B"]="#14b8a6"
  ["#0ECB81"]="#14b8a6"
  ["#F6465D"]="#ef4444"
  ["nofx-glass"]="modern-card"
  ["binance-card"]="modern-card"
  ["shadow-\[0_0_15px_rgba(240,185,11"]="shadow-glow-teal"
  ["shadow-\[0_0_20px_rgba(255,215,0"]="shadow-glow-teal"
  ["shadow-\[0_0_25px_rgba(255,215,0"]="shadow-glow-teal-lg"
)

echo "🚀 开始批量迁移样式..."

for file in "${files[@]}"; do
  if [ -f "$file" ]; then
    echo "📝 处理: $file"
    
    # 应用所有替换
    for old in "${!replacements[@]}"; do
      new="${replacements[$old]}"
      sed -i "s/$old/$new/g" "$file"
    done
    
    echo "✅ 完成: $file"
  else
    echo "⚠️  文件不存在: $file"
  fi
done

echo ""
echo "✨ 批量迁移完成！"
echo ""
echo "📋 已处理的文件："
for file in "${files[@]}"; do
  echo "  - $file"
done



