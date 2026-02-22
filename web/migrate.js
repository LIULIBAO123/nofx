const fs = require('fs');
const path = require('path');

// 样式替换映射
const replacements = [
  // 类名替换
  ['nofx-gold', 'teal-400'],
  ['text-nofx-gold', 'text-teal-400'],
  ['bg-nofx-gold', 'bg-teal-500'],
  ['border-nofx-gold', 'border-teal-500'],
  ['hover:text-nofx-gold', 'hover:text-teal-400'],
  ['hover:bg-nofx-gold', 'hover:bg-teal-500'],
  ['focus:border-nofx-gold', 'focus:border-teal-500'],
  ['ring-nofx-gold', 'ring-teal-500'],
  
  // 十六进制颜色
  ['#F0B90B', '#14b8a6'],
  ['#0ECB81', '#14b8a6'],
  ['#F6465D', '#ef4444'],
  
  // 卡片样式
  ['nofx-glass', 'modern-card'],
  ['binance-card', 'modern-card'],
  
  // 阴影效果
  ['shadow-\\[0_0_15px_rgba\\(240,185,11,0\\.1\\)\\]', 'shadow-glow-teal'],
  ['shadow-\\[0_0_15px_rgba\\(240,185,11,0\\.2\\)\\]', 'shadow-glow-teal'],
  ['shadow-\\[0_0_20px_rgba\\(255,215,0,0\\.1\\)\\]', 'shadow-glow-teal'],
  ['shadow-\\[0_0_25px_rgba\\(255,215,0,0\\.25\\)\\]', 'shadow-glow-teal-lg'],
  ['shadow-\\[0_0_30px_rgba\\(255,215,0,0\\.3\\)\\]', 'shadow-glow-teal-lg'],
];

// 要处理的文件列表
const files = [
  'src/components/RegisterPage.tsx',
  'src/components/ResetPasswordPage.tsx',
  'src/components/WhitelistFullPage.tsx',
  'src/components/BacktestPage.tsx',
  'src/pages/StrategyStudioPage.tsx',
  'src/pages/StrategyMarketPage.tsx',
  'src/pages/LandingPage.tsx',
  'src/pages/FAQPage.tsx',
  'src/pages/DebateArenaPage.tsx',
];

let updatedCount = 0;
let totalCount = 0;

console.log('🚀 开始批量迁移样式到 Meridian 设计系统...\n');

files.forEach(file => {
  const filePath = path.join(__dirname, file);
  
  if (!fs.existsSync(filePath)) {
    console.log(`⚠️  文件不存在: ${file}`);
    return;
  }
  
  totalCount++;
  
  try {
    let content = fs.readFileSync(filePath, 'utf8');
    const originalContent = content;
    
    // 应用所有替换
    replacements.forEach(([oldStr, newStr]) => {
      const regex = new RegExp(oldStr, 'g');
      content = content.replace(regex, newStr);
    });
    
    // 如果内容有变化，写回文件
    if (content !== originalContent) {
      fs.writeFileSync(filePath, content, 'utf8');
      console.log(`✅ 已更新: ${file}`);
      updatedCount++;
    } else {
      console.log(`⏭️  跳过: ${file} (无需更改)`);
    }
  } catch (error) {
    console.log(`❌ 错误: ${file} - ${error.message}`);
  }
});

console.log(`\n✨ 完成! 已更新 ${updatedCount}/${totalCount} 个文件`);



