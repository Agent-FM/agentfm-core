/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        bg:     { 0: '#0a0b0e', 1: '#101217', 2: '#16191f' },
        border: { 0: 'rgba(255,255,255,0.06)', 1: 'rgba(255,255,255,0.10)' },
        text:   { 0: '#ededf0', 1: '#a0a6b0', 2: '#6b7280', 3: '#4b5360' },
        accent: {
          DEFAULT: '#22d3ee',
          dim:     '#0ea5b7',
          light:   '#67e8f9',
          high:    '#a5f3fc',
          fg:      '#06121a',
          bg:      'rgba(34,211,238,0.12)',
          soft:    'rgba(34,211,238,0.12)',
        },
        ok:   '#34d399',
        warn: '#f59e0b',
        bad:  '#f87171',
      },
      fontFamily: {
        sans: ['system-ui', '-apple-system', 'Inter', 'sans-serif'],
        mono: ['JetBrains Mono', 'SF Mono', 'Menlo', 'monospace'],
      },
      // Bumped one notch from Tailwind defaults for better readability on
      // dense status / radar screens. Keeps the visual hierarchy intact.
      fontSize: {
        '2xs': ['12px', { lineHeight: '16px' }],
        xs: ['13.5px', { lineHeight: '19px' }],
        sm: ['15px', { lineHeight: '22px' }],
        base: ['16.5px', { lineHeight: '26px' }],
        lg: ['19px', { lineHeight: '28px' }],
        xl: ['22px', { lineHeight: '32px' }],
        '2xl': ['27px', { lineHeight: '34px' }],
        '3xl': ['33px', { lineHeight: '40px' }],
      },
      boxShadow: {
        card:        '0 1px 2px rgba(0,0,0,.4)',
        'card-hover':'0 4px 16px -4px rgba(0,0,0,.5)',
      },
      animation: {
        pulse: 'pulse 2s ease-in-out infinite',
        blink: 'blink 1s steps(2) infinite',
        'pulse-cyan':   'pulseCyan 2.2s ease-in-out infinite',
        shimmer:        'shimmer 1.8s cubic-bezier(.4,0,.6,1) infinite',
        drift:          'drift 60s ease-in-out infinite',
        float:          'float 4s ease-in-out infinite',
        'radar-sweep':   'radar-sweep 4s linear infinite',
        'sheen':         'sheen 6s ease-in-out infinite',
        'aurora':        'aurora 18s ease-in-out infinite',
      },
      keyframes: {
        pulse: { '0%, 100%': { opacity: '1' }, '50%': { opacity: '0.35' } },
        blink: { '50%': { opacity: '0' } },
        pulseCyan:   { '0%,100%': { opacity: '1', transform: 'scale(1)' }, '50%': { opacity: '.55', transform: 'scale(.85)' } },
        shimmer: { '0%': { transform: 'translateX(-100%)' }, '100%': { transform: 'translateX(100%)' } },
        drift: {
          '0%, 100%': { transform: 'translate(0, 0)' },
          '50%': { transform: 'translate(2%, -1%)' },
        },
        float: {
          '0%, 100%': { transform: 'translateY(0)' },
          '50%': { transform: 'translateY(-6px)' },
        },
        'radar-sweep':   { '0%': { transform: 'rotate(0deg)' }, '100%': { transform: 'rotate(360deg)' } },
        'sheen':         { '0%': { transform: 'translateX(-150%) skewX(-12deg)' }, '60%,100%': { transform: 'translateX(250%) skewX(-12deg)' } },
        'aurora':        { '0%,100%': { transform: 'translate(0,0) scale(1)', opacity: '.9' }, '50%': { transform: 'translate(24px,-20px) scale(1.08)', opacity: '1' } },
      },
    },
  },
}
