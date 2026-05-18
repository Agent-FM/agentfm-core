/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        bg: { 0: '#0b0d10', 1: '#11151b', 2: '#14181f' },
        border: { 0: '#1c2028', 1: '#2a3038' },
        text: { 0: '#f1f3f7', 1: '#b9c3d1', 2: '#6c7886', 3: '#4b5664' },
        accent: {
          DEFAULT: 'var(--accent)',
          fg: 'var(--accent-fg)',
          bg: 'var(--accent-bg)',
        },
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
      },
      keyframes: {
        pulse: { '0%, 100%': { opacity: '1' }, '50%': { opacity: '0.35' } },
        blink: { '50%': { opacity: '0' } },
      },
    },
  },
}
