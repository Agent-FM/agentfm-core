/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        bg:     { 0: '#07090d', 1: '#0d1117', 2: '#131922' },
        border: { 0: '#1a2030', 1: '#283047' },
        text:   { 0: '#f3f6fa', 1: '#c4cdd9', 2: '#7a8595', 3: '#4a5566' },
        accent: {
          DEFAULT: '#22d3ee',
          dim:     '#06b6d4',
          light:   '#67e8f9',
          high:    '#a5f3fc',
          fg:      '#07090d',
          bg:      '#062a36',
        },
        accent2: {
          DEFAULT: '#a855f7',
          dim:     '#7e22ce',
          light:   '#d8b4fe',
          bg:      '#1f0a36',
        },
        ok:   '#84cc16',
        warn: '#f59e0b',
        bad:  '#f43f5e',
      },
      fontFamily: {
        sans: ['system-ui', '-apple-system', 'Inter', 'sans-serif'],
        mono: ['JetBrains Mono', 'SF Mono', 'Menlo', 'monospace'],
      },
      // Bumped one notch from Tailwind defaults for better readability on
      // dense status / radar screens. Keeps the visual hierarchy intact.
      fontSize: {
        '2xs': ['11px', { lineHeight: '15px' }],
        xs: ['13px', { lineHeight: '18px' }],
        sm: ['14.5px', { lineHeight: '21px' }],
        base: ['16px', { lineHeight: '24px' }],
        lg: ['18px', { lineHeight: '27px' }],
        xl: ['21px', { lineHeight: '30px' }],
        '2xl': ['25px', { lineHeight: '32px' }],
        '3xl': ['30px', { lineHeight: '36px' }],
      },
      animation: {
        pulse: 'pulse 2s ease-in-out infinite',
        blink: 'blink 1s steps(2) infinite',
        'pulse-cyan':   'pulseCyan 2.2s ease-in-out infinite',
        'pulse-violet': 'pulseViolet 2.2s ease-in-out infinite',
        'pulse-rose':   'pulseRose 2.2s ease-in-out infinite',
        shimmer:        'shimmer 1.8s cubic-bezier(.4,0,.6,1) infinite',
        drift:          'drift 60s ease-in-out infinite',
        float:          'float 4s ease-in-out infinite',
      },
      keyframes: {
        pulse: { '0%, 100%': { opacity: '1' }, '50%': { opacity: '0.35' } },
        blink: { '50%': { opacity: '0' } },
        pulseCyan:   { '0%,100%': { opacity: '1', transform: 'scale(1)' }, '50%': { opacity: '.55', transform: 'scale(.85)' } },
        pulseViolet: { '0%,100%': { opacity: '1', transform: 'scale(1)' }, '50%': { opacity: '.55', transform: 'scale(.85)' } },
        pulseRose:   { '0%,100%': { opacity: '1', transform: 'scale(1)' }, '50%': { opacity: '.55', transform: 'scale(.85)' } },
        shimmer: { '0%': { transform: 'translateX(-100%)' }, '100%': { transform: 'translateX(100%)' } },
        drift: {
          '0%, 100%': { transform: 'translate(0, 0)' },
          '50%': { transform: 'translate(2%, -1%)' },
        },
        float: {
          '0%, 100%': { transform: 'translateY(0)' },
          '50%': { transform: 'translateY(-6px)' },
        },
      },
    },
  },
}
