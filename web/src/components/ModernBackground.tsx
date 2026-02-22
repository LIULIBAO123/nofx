import React from 'react'

interface ModernBackgroundProps extends React.HTMLAttributes<HTMLDivElement> {
  children?: React.ReactNode
  className?: string
  variant?: 'default' | 'trading' | 'minimal'
}

export function ModernBackground({ 
  children, 
  className = '', 
  variant = 'default',
  ...props 
}: ModernBackgroundProps) {
  return (
    <div 
      className={`relative w-full min-h-screen overflow-hidden flex flex-col ${className}`} 
      style={{
        background: variant === 'minimal' 
          ? 'linear-gradient(135deg, #1a1d23 0%, #0f1419 50%, #1a2028 100%)'
          : 'linear-gradient(135deg, #1a1d23 0%, #0f1419 30%, #1a2028 60%, #0d1117 100%)',
      }}
      {...props}
    >
      {/* 1. 深色渐变基础层 - 参考网站风格 */}
      <div 
        className="absolute inset-0 pointer-events-none"
        style={{
          background: 'radial-gradient(ellipse 80% 50% at 50% -20%, rgba(240, 185, 11, 0.08) 0%, transparent 60%)',
        }}
      />

      {/* 2. 微妙的网格纹理 */}
      <div 
        className="absolute inset-0 pointer-events-none opacity-[0.02]"
        style={{
          backgroundImage: `
            linear-gradient(rgba(255, 255, 255, 0.05) 1px, transparent 1px),
            linear-gradient(90deg, rgba(255, 255, 255, 0.05) 1px, transparent 1px)
          `,
          backgroundSize: '50px 50px',
        }}
      />

      {/* 3. 动态光晕效果 */}
      <div className="absolute inset-0 overflow-hidden pointer-events-none">
        {/* 金色光晕 - 左上 */}
        <div 
          className="absolute -top-[20%] -left-[10%] w-[600px] h-[600px] rounded-full blur-[120px] opacity-20 animate-pulse-slow"
          style={{
            background: 'radial-gradient(circle, rgba(240, 185, 11, 0.4) 0%, transparent 70%)',
            animationDuration: '8s',
          }}
        />
        
        {/* 青色光晕 - 右下 */}
        <div 
          className="absolute -bottom-[20%] -right-[10%] w-[600px] h-[600px] rounded-full blur-[120px] opacity-15 animate-pulse-slow"
          style={{
            background: 'radial-gradient(circle, rgba(0, 240, 255, 0.3) 0%, transparent 70%)',
            animationDuration: '10s',
            animationDelay: '2s',
          }}
        />

        {/* 中央微光 */}
        <div 
          className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[800px] h-[400px] rounded-full blur-[100px] opacity-10"
          style={{
            background: 'radial-gradient(ellipse, rgba(240, 185, 11, 0.3) 0%, transparent 60%)',
          }}
        />
      </div>

      {/* 4. 细腻的噪点纹理 */}
      <div 
        className="absolute inset-0 pointer-events-none opacity-[0.015] mix-blend-overlay"
        style={{
          backgroundImage: `url("data:image/svg+xml,%3Csvg viewBox='0 0 400 400' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='noiseFilter'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.9' numOctaves='4' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23noiseFilter)'/%3E%3C/svg%3E")`,
        }}
      />

      {/* 5. 顶部渐变遮罩 - 增强深度 */}
      <div 
        className="absolute inset-x-0 top-0 h-[300px] pointer-events-none"
        style={{
          background: 'linear-gradient(180deg, rgba(10, 13, 18, 0.8) 0%, transparent 100%)',
        }}
      />

      {/* 6. 底部渐变遮罩 */}
      <div 
        className="absolute inset-x-0 bottom-0 h-[200px] pointer-events-none"
        style={{
          background: 'linear-gradient(0deg, rgba(10, 13, 18, 0.6) 0%, transparent 100%)',
        }}
      />

      {/* 内容层 */}
      <div className="relative z-10 flex-1 flex flex-col h-full w-full">
        {children}
      </div>
    </div>
  )
}







