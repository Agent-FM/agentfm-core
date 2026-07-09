/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        // Xcode Default Dark pane surfaces (CSS vars in globals.css).
        bg: {
          0: 'rgb(var(--editor-bg) / <alpha-value>)',
          1: 'rgb(var(--window-chrome) / <alpha-value>)',
          2: 'rgb(var(--raised) / <alpha-value>)',
          well: 'rgb(var(--field-bg) / <alpha-value>)',
        },
        chrome: 'rgb(var(--window-chrome) / <alpha-value>)',
        navigator: 'rgb(var(--navigator-bg) / <alpha-value>)',
        editor: 'rgb(var(--editor-bg) / <alpha-value>)',
        inspector: 'rgb(var(--inspector-bg) / <alpha-value>)',
        raised: 'rgb(var(--raised) / <alpha-value>)',
        control: {
          DEFAULT: 'rgb(var(--control) / <alpha-value>)',
          hover: 'rgb(var(--control-hover) / <alpha-value>)',
          active: 'rgb(var(--control-active) / <alpha-value>)',
          selected: 'rgb(var(--control-selected) / <alpha-value>)',
        },
        border: {
          0: 'rgb(var(--hairline) / 0.085)',
          1: 'rgb(var(--hairline) / 0.14)',
        },
        text: {
          0: 'rgb(var(--text-primary) / <alpha-value>)',
          1: 'rgb(var(--text-secondary) / <alpha-value>)',
          2: 'rgb(var(--text-muted) / <alpha-value>)',
          3: 'rgb(var(--text-disabled) / <alpha-value>)',
        },
        accent: {
          DEFAULT: 'rgb(var(--accent) / <alpha-value>)',
          light: 'rgb(var(--accent-hover) / <alpha-value>)',
          dim: 'rgb(var(--accent-pressed) / <alpha-value>)',
          high: 'rgb(var(--accent-hover) / <alpha-value>)',
          fg: 'rgb(var(--on-accent) / <alpha-value>)',
          bg: 'rgb(var(--accent) / 0.28)',
          soft: 'rgb(var(--accent) / 0.28)',
        },
        ok: 'rgb(var(--ok) / <alpha-value>)',
        warn: 'rgb(var(--warn) / <alpha-value>)',
        bad: 'rgb(var(--bad) / <alpha-value>)',
        run: 'rgb(var(--run-green) / <alpha-value>)',
      },
      fontFamily: {
        sans: ['-apple-system', 'BlinkMacSystemFont', 'SF Pro Text', 'Segoe UI Variable', 'system-ui', 'sans-serif'],
        display: ['-apple-system', 'BlinkMacSystemFont', 'SF Pro Text', 'Segoe UI Variable', 'system-ui', 'sans-serif'],
        mono: ['ui-monospace', 'SF Mono', 'Menlo', 'JetBrains Mono', 'monospace'],
      },
      // Xcode density: 11 captions/jump bar, 13 default UI, 15 semibold titles.
      fontSize: {
        '2xs': ['12px', { lineHeight: '16px' }],
        xs: ['13px', { lineHeight: '17px' }],
        sm: ['14px', { lineHeight: '19px' }],
        base: ['14px', { lineHeight: '20px' }],
        lg: ['16px', { lineHeight: '21px' }],
        xl: ['17px', { lineHeight: '22px' }],
        '2xl': ['19px', { lineHeight: '24px' }],
        '3xl': ['24px', { lineHeight: '28px' }],
      },
      boxShadow: {
        card: 'none',
        float: '0 8px 24px rgba(0,0,0,0.5)',
      },
      // 5px controls/fields, 6px pills/popovers, 0 on panes/tabs. ≤8px always.
      borderRadius: {
        ctl: '5px',
        card: '6px',
        sheet: '8px',
      },
      animation: {
        pulse: 'pulse 1.2s ease-in-out infinite',
        blink: 'blink 1s steps(2) infinite',
        'pulse-cyan': 'pulseCyan 1.2s ease-in-out infinite',
        shimmer: 'shimmer 1.8s cubic-bezier(.4,0,.6,1) infinite',
        'radar-sweep': 'radar-sweep 4s linear infinite',
        'progress-slide': 'progressSlide 1.2s ease-in-out infinite',
      },
      keyframes: {
        pulse: { '0%, 100%': { opacity: '1' }, '50%': { opacity: '0.35' } },
        blink: { '50%': { opacity: '0' } },
        pulseCyan: { '0%,100%': { opacity: '1' }, '50%': { opacity: '.45' } },
        shimmer: { '0%': { transform: 'translateX(-100%)' }, '100%': { transform: 'translateX(100%)' } },
        'radar-sweep': { '0%': { transform: 'rotate(0deg)' }, '100%': { transform: 'rotate(360deg)' } },
        progressSlide: { '0%': { transform: 'translateX(-100%)' }, '100%': { transform: 'translateX(350%)' } },
      },
    },
  },
}
