#!/usr/bin/env python3
"""
批量迁移 NOFX 样式到 Meridian 设计系统
"""
import os
import re
from pathlib import Path

# 样式替换映射
REPLACEMENTS = [
    # 颜色类名
    ('nofx-gold', 'teal-400'),
    ('text-nofx-gold', 'text-teal-400'),
    ('bg-nofx-gold', 'bg-teal-500'),
    ('border-nofx-gold', 'border-teal-500'),
    ('nofx-green', 'teal-500'),
    ('nofx-red', 'red-500'),
    ('nofx-text-main', 'white'),
    ('nofx-text-muted', 'zinc-400'),
    ('nofx-text', 'white'),
    ('nofx-bg-lighter', 'white/10'),
    ('nofx-bg', 'zinc-950'),
    ('nofx-glass', 'modern-card'),
    ('binance-card', 'modern-card'),
    ('nofx-accent', 'teal-500'),
    ('nofx-danger', 'red-500'),
    
    # 十六进制颜色
    ('#F0B90B', '#14b8a6'),
    ('#0ECB81', '#14b8a6'),
    ('#F6465D', '#ef4444'),
    ('#0B0E11', 'rgba(255, 255, 255, 0.02)'),
    ('#2B3139', 'rgba(255, 255, 255, 0.05)'),
    ('#EAECEF', '#ffffff'),
    ('#848E9C', '#a1a1aa'),
    
    # RGB 颜色
    ('rgba(240, 185, 11,', 'rgba(20, 184, 166,'),
    ('rgba(14, 203, 129,', 'rgba(20, 184, 166,'),
    ('rgba(246, 70, 93,', 'rgba(239, 68, 68,'),
    ('rgba(11, 14, 17,', 'rgba(255, 255, 255, 0.02'),
    ('rgba(43, 49, 57,', 'rgba(255, 255, 255, 0.05'),
    
    # 阴影效果
    ('shadow-\\[0_0_15px_rgba\\(240,185,11', 'shadow-glow-teal'),
    ('shadow-\\[0_0_20px_rgba\\(240,185,11', 'shadow-glow-teal-lg'),
]

def migrate_file(file_path):
    """迁移单个文件"""
    try:
        with open(file_path, 'r', encoding='utf-8') as f:
            content = f.read()
        
        original_content = content
        
        # 应用所有替换
        for old, new in REPLACEMENTS:
            content = content.replace(old, new)
        
        # 如果内容有变化，写回文件
        if content != original_content:
            with open(file_path, 'w', encoding='utf-8') as f:
                f.write(content)
            print(f"✅ 已更新: {file_path}")
            return True
        else:
            print(f"⏭️  跳过: {file_path} (无需更改)")
            return False
            
    except Exception as e:
        print(f"❌ 错误: {file_path} - {e}")
        return False

def main():
    """主函数"""
    # 要处理的目录
    directories = [
        'src/pages',
        'src/components',
    ]
    
    # 要处理的文件扩展名
    extensions = ['.tsx', '.ts', '.jsx', '.js']
    
    updated_count = 0
    total_count = 0
    
    print("🚀 开始迁移样式到 Meridian 设计系统...\n")
    
    for directory in directories:
        dir_path = Path(directory)
        if not dir_path.exists():
            print(f"⚠️  目录不存在: {directory}")
            continue
            
        # 递归查找所有文件
        for ext in extensions:
            for file_path in dir_path.rglob(f'*{ext}'):
                total_count += 1
                if migrate_file(file_path):
                    updated_count += 1
    
    print(f"\n✨ 完成! 已更新 {updated_count}/{total_count} 个文件")

if __name__ == '__main__':
    main()



