/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        // Meridian Cyan/Teal Theme (OKLCH)
        teal: {
          50: 'oklch(0.95 0.02 180)',
          100: 'oklch(0.90 0.04 180)',
          200: 'oklch(0.80 0.08 180)',
          300: 'oklch(0.70 0.12 180)',
          400: 'oklch(0.60 0.16 180)',
          500: 'oklch(0.55 0.18 180)',
          600: 'oklch(0.50 0.16 180)',
          700: 'oklch(0.40 0.14 180)',
          800: 'oklch(0.30 0.10 180)',
          900: 'oklch(0.20 0.06 180)',
        },
        azure: {
          400: 'oklch(0.65 0.15 240)',
          500: 'oklch(0.60 0.18 240)',
          600: 'oklch(0.55 0.16 240)',
        },
        amber: {
          400: 'oklch(0.75 0.15 80)',
          500: 'oklch(0.70 0.18 75)',
          600: 'oklch(0.65 0.16 70)',
        },
        rose: {
          400: 'oklch(0.65 0.20 15)',
          500: 'oklch(0.60 0.22 12)',
          600: 'oklch(0.55 0.20 10)',
        },
        emerald: {
          400: 'oklch(0.70 0.16 150)',
          500: 'oklch(0.65 0.18 145)',
          600: 'oklch(0.60 0.16 140)',
        },
        slate: {
          50: 'oklch(0.98 0.002 260)',
          100: 'oklch(0.95 0.004 260)',
          200: 'oklch(0.88 0.006 260)',
          300: 'oklch(0.78 0.008 260)',
          400: 'oklch(0.62 0.010 260)',
          500: 'oklch(0.50 0.012 260)',
          600: 'oklch(0.42 0.012 260)',
          700: 'oklch(0.35 0.010 260)',
          800: 'oklch(0.27 0.008 260)',
          900: 'oklch(0.18 0.006 260)',
          950: 'oklch(0.11 0.008 260)',
        },
        // Legacy support
        'nofx-gold': {
          DEFAULT: 'oklch(0.70 0.18 75)',
          dim: 'oklch(0.70 0.18 75 / 0.1)',
          glow: 'oklch(0.70 0.18 75 / 0.3)',
          highlight: 'oklch(0.75 0.15 80)',
          dark: 'oklch(0.65 0.16 70)',
        },
        'nofx-accent': 'oklch(0.55 0.18 180)',
        'nofx-success': 'oklch(0.65 0.18 145)',
        'nofx-danger': 'oklch(0.60 0.22 12)',
      },
      fontFamily: {
        sans: ['DM Sans', 'Inter', 'ui-sans-serif', 'system-ui'],
        display: ['Outfit', 'Space Grotesk', 'Inter', 'sans-serif'],
        mono: ['JetBrains Mono', 'Menlo', 'Monaco', 'Courier New', 'monospace'],
      },
      backgroundImage: {
        'gradient-radial': 'radial-gradient(circle at center, var(--tw-gradient-stops))',
        'gradient-meridian': 'radial-gradient(circle at 20% 10%, oklch(0.15 0.05 180 / 0.15) 0%, transparent 50%), radial-gradient(circle at 80% 80%, oklch(0.15 0.05 240 / 0.1) 0%, transparent 50%)',
        'grid-pattern': "linear-gradient(to right, oklch(0.98 0.002 260 / 0.05) 1px, transparent 1px), linear-gradient(to bottom, oklch(0.98 0.002 260 / 0.05) 1px, transparent 1px)",
      },
      animation: {
        'pulse-slow': 'pulse 4s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        'float': 'float 6s ease-in-out infinite',
        'shimmer': 'shimmer 2s linear infinite',
        'fade-in': 'fadeIn 0.3s ease-out',
        'slide-in': 'slideIn 0.4s ease-out',
        'scale-in': 'scaleIn 0.3s ease-out',
        'glow-pulse': 'glowPulse 2s ease-in-out infinite',
      },
      keyframes: {
        float: {
          '0%, 100%': { transform: 'translateY(0)' },
          '50%': { transform: 'translateY(-10px)' },
        },
        shimmer: {
          '0%': { backgroundPosition: '-200% 0' },
          '100%': { backgroundPosition: '200% 0' },
        },
        fadeIn: {
          '0%': { opacity: '0', transform: 'translateY(10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
        slideIn: {
          '0%': { opacity: '0', transform: 'translateX(-20px)' },
          '100%': { opacity: '1', transform: 'translateX(0)' },
        },
        scaleIn: {
          '0%': { opacity: '0', transform: 'scale(0.95)' },
          '100%': { opacity: '1', transform: 'scale(1)' },
        },
        glowPulse: {
          '0%, 100%': { opacity: '1', boxShadow: '0 0 8px currentColor' },
          '50%': { opacity: '0.7', boxShadow: '0 0 16px currentColor' },
        },
      },
      boxShadow: {
        'card': '0 2px 8px rgba(0, 0, 0, 0.12), 0 1px 2px rgba(0, 0, 0, 0.08)',
        'card-hover': '0 8px 24px rgba(0, 0, 0, 0.16), 0 2px 8px rgba(0, 0, 0, 0.12)',
        'button': '0 2px 4px rgba(0, 0, 0, 0.1)',
        'glow-teal': '0 0 20px oklch(0.55 0.18 180 / 0.4), 0 0 40px oklch(0.55 0.18 180 / 0.2)',
        'glow-teal-sm': '0 0 10px oklch(0.55 0.18 180 / 0.3)',
        'glow-amber': '0 0 20px oklch(0.70 0.18 75 / 0.4), 0 0 40px oklch(0.70 0.18 75 / 0.2)',
      },
      backdropBlur: {
        xs: '2px',
      },
    },
  },
  plugins: [],
}
