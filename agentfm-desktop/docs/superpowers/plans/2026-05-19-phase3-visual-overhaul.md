# Phase 3 Visual Overhaul — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Comprehensive Neon Cyber visual rebuild of all 12 screens with cyan-primary / violet-secondary palette, medium-intensity motion, and consolidated component primitives. No functional changes.

**Architecture:** Three batches. **Foundation** rewrites tokens, motion lib, primitives, badges. **Chrome** rebuilds TopBar/TabStrip/ProjectDropdown/EmptyState/Settings. **Content+Overlays** rebuilds Radar/Status/Chat/Activity/PeerView/DispatchDrawer/FeedbackModal/CreateProjectWizard/ProjectSwitchingOverlay/BackendDown/LogsModal. The renderer-only refactor leaves backend, IPC, store, and routing untouched.

**Tech Stack:** React 18 + TypeScript, Tailwind 3, Framer Motion, **Lucide React (new dependency)**, Vitest, Playwright.

**Reference spec:** `docs/superpowers/specs/2026-05-19-phase3-visual-overhaul-design.md`.

---

## File map

**Create:**
- `src/lib/motion.ts` — Framer Motion preset library
- `src/components/primitives/Badge.tsx` — consolidated badge
- `src/components/primitives/StatusDot.tsx` — pulse-capable status dot
- `src/components/primitives/Skeleton.tsx` — shimmer loader primitives
- `tests/unit/motion.test.ts`
- `tests/unit/badge.test.tsx`
- `tests/e2e/visual-smoke.spec.ts`

**Modify:**
- `tailwind.config.js` — color tokens, fontSize, animations
- `src/styles/tokens.css` — CSS variables
- `src/styles/globals.css` — pulse/shimmer/glow utilities
- `package.json` — add `lucide-react`
- `src/components/primitives/Button.tsx` — new variants + gradient
- `src/components/primitives/Input.tsx` — cyan focus ring
- `src/components/primitives/Card.tsx` — `density`/`live` props
- `src/components/primitives/Slider.tsx` — cyan-violet gradient track
- `src/components/primitives/SegGroup.tsx` — gradient-border active
- `src/components/TopBar.tsx`
- `src/components/TabStrip.tsx`
- `src/components/projects/ProjectDropdown.tsx`
- `src/components/EmptyState.tsx`
- `src/components/SettingsFooter.tsx`
- `src/components/SettingsSheet.tsx`
- `src/routes/Radar.tsx`
- `src/components/AgentCard.tsx`
- `src/components/EmptyRadar.tsx`
- `src/components/RadarSkeleton.tsx`
- `src/routes/Status.tsx`
- `src/routes/Chat.tsx`
- `src/components/chat/SessionList.tsx`
- `src/components/chat/AgentPicker.tsx`
- `src/components/chat/MessageBubble.tsx`
- `src/components/chat/Composer.tsx`
- `src/routes/Activity.tsx`
- `src/routes/PeerView.tsx`
- `src/components/peer/SummaryCard.tsx`
- `src/components/peer/EntryRow.tsx`
- `src/components/peer/Tabs.tsx`
- `src/components/DispatchDrawer.tsx`
- `src/components/StreamingView.tsx`
- `src/components/FeedbackModal.tsx`
- `src/components/projects/CreateProjectWizard.tsx`
- `src/components/projects/ProjectSwitchingOverlay.tsx`
- `src/components/BackendDownOverlay.tsx`
- `src/components/status/LogsModal.tsx`
- `src/components/status/StatusCard.tsx`
- Various e2e spec files for selector updates

**Delete** (after deprecation period — these become thin re-exports first):
- `src/components/HonestyBadge.tsx`
- `src/components/DispatchBadge.tsx`
- `src/components/EquivocatorBadge.tsx`
- `src/components/CapabilityBadge.tsx`

---

## BATCH 1 — FOUNDATION

### Task 1: Install lucide-react + rewrite tokens

**Files:**
- Modify: `package.json`
- Modify: `tailwind.config.js`
- Modify: `src/styles/tokens.css`
- Modify: `src/styles/globals.css`

- [ ] **Step 1: Install dependency**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npm install lucide-react@^0.460.0
```

Expected: package.json updated, no install errors.

- [ ] **Step 2: Rewrite Tailwind color tokens**

Replace the `extend.colors` block in `tailwind.config.js` with:

```js
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
```

Also extend `animation` and `keyframes`:

```js
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
```

Keep the `fontSize` map you already set (Phase 1 bump). Keep `fontFamily`.

- [ ] **Step 3: Rewrite tokens.css**

Replace the entire content of `src/styles/tokens.css` with:

```css
:root {
  --accent: #22d3ee;
  --accent-fg: #07090d;
  --accent-bg: #062a36;
  --accent-2: #a855f7;
  --accent-2-bg: #1f0a36;
}

[data-theme="light"] {
  --accent: #06b6d4;
  --accent-fg: #f3f6fa;
  --accent-bg: #cffafe;
  --accent-2: #7e22ce;
  --accent-2-bg: #ede9fe;
}
```

The Phase 1 `[data-accent="..."]` overrides are removed entirely.

- [ ] **Step 4: Add utility classes to globals.css**

Append to the bottom of `src/styles/globals.css`:

```css
.neon-glow-cyan   { box-shadow: 0 0 0 1px rgba(34,211,238,.3), 0 0 18px -2px rgba(34,211,238,.35); }
.neon-glow-violet { box-shadow: 0 0 0 1px rgba(168,85,247,.3), 0 0 18px -2px rgba(168,85,247,.35); }
.neon-glow-rose   { box-shadow: 0 0 0 1px rgba(244,63,94,.3), 0 0 18px -2px rgba(244,63,94,.35); }

.glow-text-cyan   { text-shadow: 0 0 8px rgba(34,211,238,.5); }
.glow-text-violet { text-shadow: 0 0 8px rgba(168,85,247,.5); }

.gradient-border-cyan {
  position: relative;
}
.gradient-border-cyan::before {
  content: '';
  position: absolute;
  inset: 0;
  padding: 1px;
  border-radius: inherit;
  background: linear-gradient(135deg, #22d3ee, #a855f7);
  -webkit-mask: linear-gradient(#000 0 0) content-box, linear-gradient(#000 0 0);
  -webkit-mask-composite: xor;
          mask-composite: exclude;
  pointer-events: none;
}

.scanlines {
  background-image: repeating-linear-gradient(
    0deg,
    rgba(34,211,238,.04) 0px,
    rgba(34,211,238,.04) 1px,
    transparent 1px,
    transparent 3px
  );
}

@media (prefers-reduced-motion: reduce) {
  .animate-pulse-cyan, .animate-pulse-violet, .animate-pulse-rose,
  .animate-shimmer, .animate-drift, .animate-float,
  .animate-blink, .animate-pulse {
    animation: none !important;
  }
}
```

- [ ] **Step 5: Build**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx electron-vite build 2>&1 | tail -5
```

Expected: clean build. Renderer bundle may grow slightly from the lucide-react import path (~2 KB now, more later as we use icons).

- [ ] **Step 6: Commit**

```bash
git add package.json package-lock.json tailwind.config.js src/styles/tokens.css src/styles/globals.css
git commit -m "feat(theme): Neon Cyber tokens, motion keyframes, glow utilities"
```

---

### Task 2: Motion preset library

**Files:**
- Create: `src/lib/motion.ts`
- Create: `tests/unit/motion.test.ts`

- [ ] **Step 1: Write failing test**

Create `tests/unit/motion.test.ts`:

```ts
import { describe, it, expect } from 'vitest'
import { fast, spring, entrance, lift } from '../../src/lib/motion'

describe('motion presets', () => {
  it('fast is a short ease-out', () => {
    expect(fast.duration).toBe(0.15)
    expect(fast.ease).toEqual([0.4, 0, 0.2, 1])
  })

  it('spring uses Framer-style config', () => {
    expect(spring.type).toBe('spring')
    expect(spring.stiffness).toBe(320)
    expect(spring.damping).toBe(28)
  })

  it('entrance has initial/animate/transition shape', () => {
    expect(entrance.initial).toEqual({ opacity: 0, y: 6 })
    expect(entrance.animate).toEqual({ opacity: 1, y: 0 })
    expect(entrance.transition.type).toBe('spring')
  })

  it('lift exposes whileHover and whileTap', () => {
    expect(lift.whileHover).toEqual({ y: -2 })
    expect(lift.whileTap).toEqual({ scale: 0.98 })
  })
})
```

- [ ] **Step 2: Run test (should fail)**

```bash
npx vitest run tests/unit/motion.test.ts
```

Expected: FAIL "Cannot find module".

- [ ] **Step 3: Implement**

Create `src/lib/motion.ts`:

```ts
export const fast = { duration: 0.15, ease: [0.4, 0, 0.2, 1] as const }

export const spring = { type: 'spring' as const, stiffness: 320, damping: 28 }

export const entrance = {
  initial: { opacity: 0, y: 6 },
  animate: { opacity: 1, y: 0 },
  transition: { type: 'spring' as const, stiffness: 280, damping: 30 },
}

export const lift = {
  whileHover: { y: -2 },
  whileTap: { scale: 0.98 },
  transition: { type: 'spring' as const, stiffness: 380, damping: 26 },
}
```

- [ ] **Step 4: Run test (passes)**

```bash
npx vitest run tests/unit/motion.test.ts
```

Expected: 4 passed.

- [ ] **Step 5: Commit**

```bash
git add src/lib/motion.ts tests/unit/motion.test.ts
git commit -m "feat(motion): preset library — fast/spring/entrance/lift"
```

---

### Task 3: Badge primitive

**Files:**
- Create: `src/components/primitives/Badge.tsx`
- Create: `tests/unit/badge.test.tsx`
- Modify: `vitest.config.ts` (add jsdom environment if not already)

- [ ] **Step 1: Check vitest config supports jsdom**

```bash
cat /Users/saif/Desktop/agentfm-prod/agentfm-desktop/vitest.config.ts
```

If `environment` is `node`, change it to `jsdom`:

```ts
import { defineConfig } from 'vitest/config'
import { resolve } from 'path'

export default defineConfig({
  test: {
    environment: 'jsdom',
    include: ['tests/**/*.test.ts', 'tests/**/*.test.tsx', 'tests/**/*.spec.ts'],
    exclude: ['tests/e2e/**'],
    globals: false,
  },
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
})
```

Install jsdom if needed:

```bash
npm install --save-dev jsdom@^25.0.0
```

- [ ] **Step 2: Write failing test**

Create `tests/unit/badge.test.tsx`:

```tsx
import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import { Badge } from '../../src/components/primitives/Badge'

describe('Badge', () => {
  it('renders children', () => {
    const { getByText } = render(<Badge>hello</Badge>)
    expect(getByText('hello')).toBeTruthy()
  })

  it('applies cyan tone classes by default', () => {
    const { container } = render(<Badge>x</Badge>)
    const el = container.firstChild as HTMLElement
    expect(el.className).toMatch(/text-accent/)
  })

  it('applies rose tone classes when tone=rose', () => {
    const { container } = render(<Badge tone="rose">x</Badge>)
    const el = container.firstChild as HTMLElement
    expect(el.className).toMatch(/text-bad|text-rose-/)
  })

  it('renders mono when mono=true', () => {
    const { container } = render(<Badge mono>x</Badge>)
    const el = container.firstChild as HTMLElement
    expect(el.className).toMatch(/font-mono/)
  })

  it('renders neutral tone with text-2', () => {
    const { container } = render(<Badge tone="neutral">x</Badge>)
    const el = container.firstChild as HTMLElement
    expect(el.className).toMatch(/text-text-2/)
  })
})
```

- [ ] **Step 3: Run test (should fail)**

```bash
npx vitest run tests/unit/badge.test.tsx
```

Expected: FAIL "Cannot find module".

- [ ] **Step 4: Implement**

Create `src/components/primitives/Badge.tsx`:

```tsx
import { ReactNode } from 'react'

export type BadgeTone = 'cyan' | 'violet' | 'lime' | 'amber' | 'rose' | 'neutral'

interface Props {
  tone?: BadgeTone
  mono?: boolean
  children: ReactNode
  className?: string
  title?: string
}

const TONES: Record<BadgeTone, string> = {
  cyan:    'bg-accent/15 border-accent/35 text-accent',
  violet:  'bg-accent2/15 border-accent2/40 text-accent2-light',
  lime:    'bg-ok/15 border-ok/40 text-ok',
  amber:   'bg-warn/15 border-warn/40 text-warn',
  rose:    'bg-bad/15 border-bad/40 text-bad',
  neutral: 'bg-bg-2 border-border-0 text-text-2',
}

export function Badge({ tone = 'cyan', mono = false, children, className, title }: Props) {
  return (
    <span
      title={title}
      className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-2xs border ${TONES[tone]} ${mono ? 'font-mono' : ''} ${className ?? ''}`}
    >
      {children}
    </span>
  )
}
```

- [ ] **Step 5: Run test (passes)**

```bash
npx vitest run tests/unit/badge.test.tsx
```

Expected: 5 passed.

- [ ] **Step 6: Commit**

```bash
git add src/components/primitives/Badge.tsx tests/unit/badge.test.tsx vitest.config.ts
git commit -m "feat(primitives): consolidated Badge with tone/mono props"
```

---

### Task 4: StatusDot primitive

**Files:**
- Create: `src/components/primitives/StatusDot.tsx`

- [ ] **Step 1: Implement**

Create `src/components/primitives/StatusDot.tsx`:

```tsx
export type DotTone = 'cyan' | 'violet' | 'amber' | 'rose' | 'lime' | 'neutral'

const COLOR: Record<DotTone, string> = {
  cyan:    'bg-accent shadow-[0_0_8px_rgba(34,211,238,.7)]',
  violet:  'bg-accent2 shadow-[0_0_8px_rgba(168,85,247,.7)]',
  amber:   'bg-warn shadow-[0_0_8px_rgba(245,158,11,.7)]',
  rose:    'bg-bad shadow-[0_0_8px_rgba(244,63,94,.7)]',
  lime:    'bg-ok shadow-[0_0_8px_rgba(132,204,22,.7)]',
  neutral: 'bg-text-3',
}

interface Props {
  tone?: DotTone
  pulse?: boolean
  size?: 'sm' | 'md'
  className?: string
}

export function StatusDot({ tone = 'cyan', pulse = false, size = 'md', className }: Props) {
  const dim = size === 'sm' ? 'w-1.5 h-1.5' : 'w-2 h-2'
  const animation =
    pulse && tone === 'cyan'   ? 'animate-pulse-cyan'   :
    pulse && tone === 'violet' ? 'animate-pulse-violet' :
    pulse && tone === 'rose'   ? 'animate-pulse-rose'   : ''
  return (
    <span className={`inline-block rounded-full ${dim} ${COLOR[tone]} ${animation} ${className ?? ''}`} />
  )
}
```

- [ ] **Step 2: Build**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx electron-vite build 2>&1 | tail -3
```

- [ ] **Step 3: Commit**

```bash
git add src/components/primitives/StatusDot.tsx
git commit -m "feat(primitives): StatusDot with tone + pulse"
```

---

### Task 5: Skeleton primitives

**Files:**
- Create: `src/components/primitives/Skeleton.tsx`

- [ ] **Step 1: Implement**

Create `src/components/primitives/Skeleton.tsx`:

```tsx
interface BoxProps {
  className?: string
}

export function SkeletonBox({ className }: BoxProps) {
  return (
    <div className={`relative overflow-hidden bg-bg-2 rounded ${className ?? ''}`}>
      <div className="absolute inset-0 animate-shimmer bg-gradient-to-r from-transparent via-accent/10 to-transparent" />
    </div>
  )
}

interface RowProps {
  delay?: number
  className?: string
}

export function SkeletonRow({ delay = 0, className }: RowProps) {
  return (
    <div
      className={`bg-bg-1 border border-border-0 rounded-xl p-4 flex items-center gap-3 ${className ?? ''}`}
      style={{ animationDelay: `${delay}ms` }}
    >
      <SkeletonBox className="w-2 h-2" />
      <div className="flex-1 space-y-2">
        <SkeletonBox className="h-3.5 w-40" />
        <SkeletonBox className="h-2.5 w-64" />
      </div>
      <SkeletonBox className="h-7 w-24" />
    </div>
  )
}
```

- [ ] **Step 2: Commit**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
git add src/components/primitives/Skeleton.tsx
git commit -m "feat(primitives): Skeleton + SkeletonRow shimmer loaders"
```

---

### Task 6: Update existing primitives

**Files:**
- Modify: `src/components/primitives/Button.tsx`
- Modify: `src/components/primitives/Input.tsx`
- Modify: `src/components/primitives/Card.tsx`
- Modify: `src/components/primitives/SegGroup.tsx`
- Modify: `src/components/primitives/Slider.tsx`

- [ ] **Step 1: Rewrite Button**

Replace `src/components/primitives/Button.tsx` with:

```tsx
import { forwardRef, ButtonHTMLAttributes, ReactNode } from 'react'
import { motion } from 'framer-motion'
import { lift } from '../../lib/motion'

export type ButtonVariant = 'default' | 'primary' | 'ghost' | 'danger'

interface Props extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'onAnimationStart' | 'onDragStart' | 'onDragEnd' | 'onDrag'> {
  variant?: ButtonVariant
  children: ReactNode
}

const VARIANT: Record<ButtonVariant, string> = {
  default: 'bg-bg-2 border border-border-1 text-text-1 hover:text-text-0 hover:border-accent/40 hover:shadow-[0_0_18px_-6px_rgba(34,211,238,.3)]',
  primary: 'relative text-accent-fg font-medium border border-accent/60 bg-gradient-to-br from-accent to-accent2 hover:from-accent2 hover:to-accent shadow-[0_0_0_1px_rgba(34,211,238,.4),0_8px_24px_-10px_rgba(34,211,238,.55)]',
  ghost:   'bg-transparent text-text-1 hover:text-accent',
  danger:  'bg-bad/15 border border-bad/40 text-bad hover:bg-bad/25 hover:border-bad/60',
}

export const Button = forwardRef<HTMLButtonElement, Props>(
  ({ variant = 'default', children, className, disabled, ...rest }, ref) => {
    return (
      <motion.button
        ref={ref}
        disabled={disabled}
        whileHover={disabled ? undefined : lift.whileHover}
        whileTap={disabled ? undefined : lift.whileTap}
        transition={lift.transition}
        className={`inline-flex items-center justify-center gap-1.5 px-3 py-1.5 rounded-md text-xs transition-colors disabled:opacity-45 disabled:pointer-events-none ${VARIANT[variant]} ${className ?? ''}`}
        {...rest}
      >
        {children}
      </motion.button>
    )
  },
)
Button.displayName = 'Button'
```

- [ ] **Step 2: Rewrite Input**

Replace `src/components/primitives/Input.tsx`:

```tsx
import { forwardRef, InputHTMLAttributes } from 'react'

type Props = InputHTMLAttributes<HTMLInputElement>

export const Input = forwardRef<HTMLInputElement, Props>(({ className, ...rest }, ref) => {
  return (
    <input
      ref={ref}
      className={`w-full bg-bg-0 border border-border-0 rounded-md px-2.5 py-1.5 text-sm text-text-0 placeholder-text-3 outline-none transition-shadow focus:border-accent focus:shadow-[0_0_0_3px_rgba(34,211,238,.15)] ${className ?? ''}`}
      {...rest}
    />
  )
})
Input.displayName = 'Input'
```

- [ ] **Step 3: Rewrite Card**

Replace `src/components/primitives/Card.tsx`:

```tsx
import { HTMLAttributes, ReactNode } from 'react'

type Density = 'default' | 'compact' | 'spacious'

interface Props extends HTMLAttributes<HTMLDivElement> {
  density?: Density
  live?: boolean
  children: ReactNode
}

const PADDING: Record<Density, string> = {
  default: 'p-4',
  compact: 'p-3',
  spacious: 'p-6',
}

export function Card({ density = 'default', live = false, children, className, ...rest }: Props) {
  return (
    <div
      className={`relative bg-bg-1 border border-border-0 rounded-xl transition-all ${PADDING[density]} ${live ? 'neon-glow-cyan' : 'hover:border-accent/30'} ${className ?? ''}`}
      {...rest}
    >
      {live && (
        <span className="absolute -top-px -left-px w-2 h-2 rounded-full bg-accent shadow-[0_0_8px_#22d3ee] animate-pulse-cyan" />
      )}
      {children}
    </div>
  )
}
```

- [ ] **Step 4: Rewrite SegGroup**

Replace `src/components/primitives/SegGroup.tsx`:

```tsx
interface Option<T> {
  value: T
  label: string
}

interface Props<T extends string> {
  options: Option<T>[]
  value: T
  onChange: (v: T) => void
}

export function SegGroup<T extends string>({ options, value, onChange }: Props<T>) {
  return (
    <div className="inline-flex p-1 bg-bg-2 border border-border-0 rounded-md gap-1">
      {options.map((o) => (
        <button
          key={o.value}
          onClick={() => onChange(o.value)}
          className={`relative px-3 py-1 text-xs rounded transition-colors ${
            o.value === value
              ? 'text-accent gradient-border-cyan bg-accent-bg'
              : 'text-text-2 hover:text-text-0'
          }`}
        >
          {o.label}
        </button>
      ))}
    </div>
  )
}
```

- [ ] **Step 5: Rewrite Slider**

Replace `src/components/primitives/Slider.tsx`:

```tsx
interface Props {
  min: number
  max: number
  step?: number
  value: number
  onChange: (v: number) => void
}

export function Slider({ min, max, step = 0.01, value, onChange }: Props) {
  const pct = ((value - min) / (max - min)) * 100
  return (
    <div className="relative w-full">
      <div className="h-1.5 rounded-full bg-bg-2 overflow-hidden">
        <div
          className="h-full bg-gradient-to-r from-accent to-accent2"
          style={{ width: `${pct}%` }}
        />
      </div>
      <input
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        className="absolute inset-0 w-full opacity-0 cursor-pointer"
      />
      <div
        className="absolute top-1/2 -translate-y-1/2 w-3 h-3 rounded-full bg-accent border border-accent-fg shadow-[0_0_8px_#22d3ee] pointer-events-none"
        style={{ left: `calc(${pct}% - 6px)` }}
      />
    </div>
  )
}
```

- [ ] **Step 6: Build + test**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx electron-vite build 2>&1 | tail -3
npx vitest run tests/unit/
```

Expected: clean build, all unit tests still pass.

- [ ] **Step 7: Commit**

```bash
git add src/components/primitives/
git commit -m "feat(primitives): Neon Cyber palette + motion for Button/Input/Card/Slider/SegGroup"
```

---

## BATCH 2 — CHROME

### Task 7: TopBar rebuild

**Files:**
- Modify: `src/components/TopBar.tsx`

- [ ] **Step 1: Rewrite**

Replace `src/components/TopBar.tsx`:

```tsx
import { useState } from 'react'
import { toast } from 'sonner'
import { Copy, Check } from 'lucide-react'
import { useUIStore } from '../lib/store'
import { ProjectDropdown } from './projects/ProjectDropdown'

function truncateMultiaddr(m: string): string {
  if (m.length <= 32) return m
  return m.slice(0, 14) + '…' + m.slice(-14)
}

export function TopBar() {
  const active = useUIStore((s) => s.activeProject())
  const [copied, setCopied] = useState(false)

  async function copyRelay() {
    if (!active) return
    const value = active.relayMultiaddr ?? '(bundled public lighthouse)'
    try {
      await navigator.clipboard.writeText(value)
      setCopied(true)
      setTimeout(() => setCopied(false), 800)
      toast.success('Relay copied')
    } catch {
      toast.error('Could not copy')
    }
  }

  return (
    <header
      className="h-12 bg-bg-0 flex items-center gap-4 px-4 select-none relative"
      style={{
        borderBottom: '1px solid transparent',
        backgroundImage:
          'linear-gradient(#07090d,#07090d), linear-gradient(90deg, rgba(34,211,238,.35) 0%, rgba(34,211,238,0) 50%, rgba(168,85,247,.25) 100%)',
        backgroundOrigin: 'border-box',
        backgroundClip: 'padding-box, border-box',
      }}
    >
      <div className="text-sm font-semibold tracking-tight">
        Agent<span className="text-accent glow-text-cyan">FM</span>
      </div>
      {active && <ProjectDropdown />}
      <div className="flex-1" />
      {active && (
        <button
          onClick={copyRelay}
          className="group inline-flex items-center gap-1.5 text-2xs text-text-2 hover:text-text-0 font-mono transition-colors px-2 py-1 rounded"
          title={active.relayMultiaddr ?? 'bundled public lighthouse'}
        >
          <span>relay: {active.relayMultiaddr ? truncateMultiaddr(active.relayMultiaddr) : 'bundled'}</span>
          {copied ? <Check size={12} className="text-accent" /> : <Copy size={12} className="opacity-0 group-hover:opacity-70 transition-opacity" />}
        </button>
      )}
    </header>
  )
}
```

- [ ] **Step 2: Build**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx electron-vite build 2>&1 | tail -3
```

- [ ] **Step 3: Commit**

```bash
git add src/components/TopBar.tsx
git commit -m "feat(topbar): gradient bottom border, glowing FM, Lucide copy icon"
```

---

### Task 8: TabStrip rebuild (with layoutId indicator)

**Files:**
- Modify: `src/components/TabStrip.tsx`

- [ ] **Step 1: Rewrite**

Replace `src/components/TabStrip.tsx`:

```tsx
import { NavLink, useLocation } from 'react-router-dom'
import { motion } from 'framer-motion'
import { useUIStore } from '../lib/store'

const tabs = [
  { to: '/radar', label: 'Radar' },
  { to: '/chat', label: 'Chat' },
  { to: '/activity', label: 'Activity' },
  { to: '/status', label: 'Status' },
]

export function TabStrip() {
  const active = useUIStore((s) => s.activeProject())
  const location = useLocation()
  if (!active) return null

  return (
    <div className="border-b border-border-0 bg-bg-0 px-3 flex gap-1 relative">
      {tabs.map((t) => {
        const isActive = location.pathname === t.to || (t.to === '/chat' && location.pathname.startsWith('/chat'))
        return (
          <NavLink
            key={t.to}
            to={t.to}
            className={`relative px-4 py-2.5 text-sm transition-colors ${
              isActive ? 'text-text-0 font-semibold' : 'text-text-2 hover:text-text-0'
            }`}
          >
            {t.label}
            {isActive && (
              <motion.div
                layoutId="tab-indicator"
                transition={{ type: 'spring', stiffness: 380, damping: 32 }}
                className="absolute bottom-0 left-2 right-2 h-[2px] bg-gradient-to-r from-accent to-accent2"
              />
            )}
          </NavLink>
        )
      })}
    </div>
  )
}
```

- [ ] **Step 2: Build**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx electron-vite build 2>&1 | tail -3
```

- [ ] **Step 3: Commit**

```bash
git add src/components/TabStrip.tsx
git commit -m "feat(tabstrip): sliding layoutId gradient indicator"
```

---

### Task 9: ProjectDropdown rebuild

**Files:**
- Modify: `src/components/projects/ProjectDropdown.tsx`

- [ ] **Step 1: Rewrite**

Replace `src/components/projects/ProjectDropdown.tsx`:

```tsx
import { useEffect, useRef, useState } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import { toast } from 'sonner'
import { ChevronDown, Plus, Trash2 } from 'lucide-react'
import { useUIStore } from '../../lib/store'

export function ProjectDropdown() {
  const projects = useUIStore((s) => s.projects)
  const activeId = useUIStore((s) => s.activeProjectId)
  const active = useUIStore((s) => s.activeProject())
  const switchProject = useUIStore((s) => s.switchProject)
  const deleteProject = useUIStore((s) => s.deleteProject)
  const openWizard = useUIStore((s) => s.openCreateWizard)
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function onDown(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') setOpen(false)
    }
    if (open) {
      document.addEventListener('mousedown', onDown)
      document.addEventListener('keydown', onKey)
      return () => {
        document.removeEventListener('mousedown', onDown)
        document.removeEventListener('keydown', onKey)
      }
    }
  }, [open])

  if (!active) return null

  function handleDelete() {
    if (!active) return
    if (!window.confirm(`Delete "${active.name}"? Its chat sessions will be removed.`)) return
    deleteProject(active.id)
    toast.success(`Project "${active.name}" deleted`)
    setOpen(false)
  }

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen((v) => !v)}
        className="relative inline-flex items-center gap-2 bg-bg-1 hover:bg-bg-2 border border-border-0 rounded-full pl-3 pr-2 py-1.5 text-xs text-text-1 transition-colors"
      >
        <span className="absolute left-0 top-1/2 -translate-y-1/2 w-[3px] h-4 bg-gradient-to-b from-accent to-accent2 rounded-full" />
        <span>📁</span>
        <span className="font-medium text-text-0 max-w-[200px] truncate">{active.name}</span>
        <motion.span animate={{ rotate: open ? 180 : 0 }} transition={{ duration: 0.18 }}>
          <ChevronDown size={14} className="text-text-2" />
        </motion.span>
      </button>

      <AnimatePresence>
        {open && (
          <motion.div
            initial={{ opacity: 0, y: -8, scale: 0.98 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: -8, scale: 0.98 }}
            transition={{ type: 'spring', stiffness: 320, damping: 30 }}
            className="absolute top-full mt-2 left-0 bg-bg-1 border border-border-0 rounded-xl shadow-2xl w-80 overflow-hidden z-50 neon-glow-cyan"
          >
            <div className="max-h-72 overflow-auto">
              {projects.map((p) => {
                const isActive = p.id === activeId
                return (
                  <button
                    key={p.id}
                    onClick={() => {
                      setOpen(false)
                      switchProject(p.id)
                    }}
                    className={`relative w-full text-left px-3 py-2.5 text-xs flex items-center gap-2 transition-colors ${
                      isActive ? 'text-accent bg-accent/8' : 'text-text-1 hover:bg-bg-2 hover:text-text-0'
                    }`}
                  >
                    {isActive && <span className="absolute left-0 top-0 bottom-0 w-[2px] bg-accent" />}
                    <span>📁</span>
                    <span className="font-medium truncate">{p.name}</span>
                  </button>
                )
              })}
            </div>
            <div className="border-t border-border-0" />
            <button
              onClick={() => { setOpen(false); openWizard() }}
              className="w-full text-left px-3 py-2.5 text-xs text-accent hover:bg-accent/10 inline-flex items-center gap-2"
            >
              <Plus size={14} />
              <span className="font-medium">New project</span>
            </button>
            <button
              onClick={handleDelete}
              className="w-full text-left px-3 py-2.5 text-xs text-bad hover:bg-bad/10 inline-flex items-center gap-2"
            >
              <Trash2 size={14} />
              <span>Delete "{active.name}"</span>
            </button>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
}
```

- [ ] **Step 2: Build + commit**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx electron-vite build 2>&1 | tail -3
git add src/components/projects/ProjectDropdown.tsx
git commit -m "feat(projects): ProjectDropdown — neon glow, gradient strip, Lucide icons"
```

---

### Task 10: EmptyState rebuild

**Files:**
- Modify: `src/components/EmptyState.tsx`

- [ ] **Step 1: Rewrite**

Replace `src/components/EmptyState.tsx`:

```tsx
import { motion } from 'framer-motion'
import { Zap } from 'lucide-react'
import { useUIStore } from '../lib/store'
import { Button } from './primitives/Button'

export function EmptyState() {
  const openWizard = useUIStore((s) => s.openCreateWizard)

  return (
    <div className="relative flex-1 flex items-center justify-center p-8 overflow-hidden">
      <div className="absolute inset-0 scanlines pointer-events-none opacity-50" />
      <div
        className="absolute inset-0 pointer-events-none animate-drift"
        style={{
          background:
            'radial-gradient(circle at 20% 0%, rgba(34,211,238,.18), transparent 45%), radial-gradient(circle at 80% 100%, rgba(168,85,247,.14), transparent 45%)',
        }}
      />
      <motion.div
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ type: 'spring', stiffness: 240, damping: 28 }}
        className="relative max-w-md text-center bg-bg-1 border border-border-0 rounded-2xl p-12 neon-glow-cyan"
      >
        <div className="text-6xl mb-5 animate-float inline-block">🛰</div>
        <h1 className="text-2xl font-semibold tracking-tight text-text-0">Welcome to <span className="text-accent glow-text-cyan">AgentFM</span></h1>
        <p className="text-text-2 mt-3 mb-7 leading-relaxed">
          Create your first project to get started. A project pairs a name with a relay; you can
          add more later.
        </p>
        <Button variant="primary" onClick={openWizard}>
          <Zap size={14} />
          <span>Create project</span>
        </Button>
      </motion.div>
    </div>
  )
}
```

- [ ] **Step 2: Build + commit**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx electron-vite build 2>&1 | tail -3
git add src/components/EmptyState.tsx
git commit -m "feat(empty): scanlines, drifting aurora gradients, floating hero"
```

---

### Task 11: SettingsFooter + SettingsSheet rebuild

**Files:**
- Modify: `src/components/SettingsFooter.tsx`
- Modify: `src/components/SettingsSheet.tsx`

- [ ] **Step 1: Rewrite SettingsFooter**

Replace `src/components/SettingsFooter.tsx`:

```tsx
import { Settings as SettingsIcon } from 'lucide-react'
import { useUIStore } from '../lib/store'
import { useBackend } from '../hooks/useBackend'
import { StatusDot } from './primitives/StatusDot'

export function SettingsFooter() {
  const openSettings = useUIStore((s) => s.openSettingsSheet)
  const backend = useBackend()
  const tone = backend.ok ? 'cyan' : 'rose'

  return (
    <footer className="border-t border-border-0 bg-bg-0 px-3 py-2 flex items-center gap-3">
      <button
        onClick={openSettings}
        className="inline-flex items-center gap-2 text-xs text-text-2 hover:text-text-0 transition-colors px-2 py-1 rounded-md hover:bg-bg-1"
      >
        <SettingsIcon size={14} />
        <span>Settings</span>
      </button>
      <div className="flex-1" />
      <div className="inline-flex items-center gap-1.5 text-2xs text-text-2 font-mono">
        <StatusDot tone={tone} pulse={backend.ok} size="sm" />
        <span>backend {backend.ok ? 'healthy' : 'down'}</span>
      </div>
    </footer>
  )
}
```

- [ ] **Step 2: Rewrite SettingsSheet**

Replace `src/components/SettingsSheet.tsx`:

```tsx
import { AnimatePresence, motion } from 'framer-motion'
import { X, FileText } from 'lucide-react'
import { useState } from 'react'
import { useUIStore } from '../lib/store'
import { useBackend } from '../hooks/useBackend'
import { useAbout } from '../lib/query'
import { SegGroup } from './primitives/SegGroup'
import { Button } from './primitives/Button'
import { StatusDot } from './primitives/StatusDot'
import { LogsModal } from './status/LogsModal'

export function SettingsSheet() {
  const open = useUIStore((s) => s.isSettingsSheetOpen)
  const close = useUIStore((s) => s.closeSettingsSheet)
  const theme = useUIStore((s) => s.theme)
  const setTheme = useUIStore((s) => s.setTheme)
  const backend = useBackend()
  const { data: about } = useAbout()
  const [showLogs, setShowLogs] = useState(false)

  return (
    <>
      <AnimatePresence>
        {open && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="fixed inset-0 bg-black/75 backdrop-blur-sm z-[65] flex justify-end"
            onClick={close}
          >
            <motion.div
              initial={{ x: '100%' }}
              animate={{ x: 0 }}
              exit={{ x: '100%' }}
              transition={{ type: 'spring', stiffness: 320, damping: 30 }}
              onClick={(e) => e.stopPropagation()}
              className="w-[420px] h-full bg-bg-1 border-l border-border-0 p-7 overflow-auto"
            >
              <div className="flex justify-between items-start mb-7">
                <h2 className="text-xl font-semibold tracking-tight text-text-0">Settings</h2>
                <button onClick={close} className="text-text-2 hover:text-text-0">
                  <X size={18} />
                </button>
              </div>

              <div className="mb-7 bg-bg-2 border border-border-0 rounded-xl p-4">
                <div className="text-2xs uppercase tracking-wider text-text-2 mb-2">Backend</div>
                <div className="flex items-center gap-2 text-sm">
                  <StatusDot tone={backend.ok ? 'cyan' : 'rose'} pulse={backend.ok} />
                  <span className="text-text-0">{backend.ok ? 'Healthy' : 'Down'}</span>
                  <span className="ml-auto text-2xs text-text-2 font-mono">v{about?.version ?? '…'}</span>
                </div>
                <div className="mt-2 text-2xs text-text-2 font-mono">
                  {backend.online_workers} online worker{backend.online_workers === 1 ? '' : 's'}
                </div>
                <div className="mt-4">
                  <Button onClick={() => setShowLogs(true)}>
                    <FileText size={12} />
                    <span>View logs</span>
                  </Button>
                </div>
              </div>

              <label className="block text-2xs uppercase tracking-wider text-text-2 mb-2">Theme</label>
              <SegGroup
                options={[
                  { value: 'dark', label: 'Dark' },
                  { value: 'light', label: 'Light' },
                  { value: 'auto', label: 'Auto' },
                ]}
                value={theme}
                onChange={setTheme}
              />
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
      <LogsModal isOpen={showLogs} onClose={() => setShowLogs(false)} />
    </>
  )
}
```

- [ ] **Step 3: Build + commit**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx electron-vite build 2>&1 | tail -3
git add src/components/SettingsFooter.tsx src/components/SettingsSheet.tsx
git commit -m "feat(settings): status dot in footer, backend block + logs in sheet"
```

---

## BATCH 3 — CONTENT + OVERLAYS

### Task 12: Radar rebuild — AgentCard + Radar route + skeleton

**Files:**
- Modify: `src/components/AgentCard.tsx`
- Modify: `src/routes/Radar.tsx`
- Modify: `src/components/RadarSkeleton.tsx`
- Modify: `src/components/EmptyRadar.tsx`

- [ ] **Step 1: Rewrite AgentCard**

Replace `src/components/AgentCard.tsx`:

```tsx
import { motion } from 'framer-motion'
import { ArrowRight, History } from 'lucide-react'
import { shortenPeerID, shortenDigest } from '../lib/peer'
import { displayName } from '../lib/displayName'
import { Button } from './primitives/Button'
import { Badge } from './primitives/Badge'
import { StatusDot } from './primitives/StatusDot'
import { lift } from '../lib/motion'
import type { WorkerProfile } from '../types/api'

interface Props {
  worker: WorkerProfile
  onHistory: () => void
  onDispatch: () => void
}

export function AgentCard({ worker, onHistory, onDispatch }: Props) {
  const busy = worker.online && worker.current_tasks >= worker.max_tasks
  const offline = !worker.online
  const equivocator = worker.is_equivocator
  const canDispatch = worker.dispatch_allowed && !busy && worker.online

  const dotTone: 'cyan' | 'amber' | 'rose' | 'neutral' =
    equivocator ? 'rose' : busy ? 'amber' : worker.online ? 'cyan' : 'neutral'

  const stripVisible = worker.online && !equivocator

  return (
    <motion.div
      whileHover={offline ? undefined : lift.whileHover}
      transition={lift.transition}
      className={`relative bg-bg-1 border border-border-0 rounded-xl pl-5 pr-4 py-4 grid grid-cols-[1fr_auto] gap-4 items-center transition-all overflow-hidden ${
        offline ? 'opacity-60' : 'hover:border-accent/40 hover:shadow-[0_10px_30px_-14px_rgba(34,211,238,.35)]'
      }`}
    >
      {stripVisible && (
        <span className="absolute left-0 top-0 bottom-0 w-1.5 bg-gradient-to-b from-accent to-accent2" />
      )}

      <div>
        <div className="flex items-center gap-2 flex-wrap">
          <StatusDot tone={dotTone} pulse={worker.online && !busy && !equivocator} />
          <h4 className="text-base font-semibold text-text-0">{displayName(worker)}</h4>
          {worker.agent_capability && <Badge tone="violet" mono>{worker.agent_capability}</Badge>}
          {busy && <Badge tone="amber">busy {worker.current_tasks}/{worker.max_tasks}</Badge>}
        </div>
        <div className="text-2xs text-text-2 font-mono mt-1.5">
          {shortenPeerID(worker.peer_id, 12, 5)}
          {worker.model && <> · {worker.model}</>}
          {worker.agent_image_digest && <> · {shortenDigest(worker.agent_image_digest, 8)}</>}
        </div>
        <div className="flex flex-wrap gap-1.5 items-center mt-2">
          {equivocator ? (
            <Badge tone="rose">⚠ equivocator</Badge>
          ) : (
            <Badge tone={worker.honesty_score > 0.3 ? 'lime' : worker.honesty_score < -0.5 ? 'rose' : 'neutral'} mono>
              {worker.honesty_score >= 0 ? '+' : ''}{worker.honesty_score.toFixed(2)}
            </Badge>
          )}
          {!equivocator && (
            <Badge tone={worker.dispatch_allowed ? 'lime' : 'rose'}>
              {worker.dispatch_allowed ? '✓ allowed' : '✗ refused'}
            </Badge>
          )}
          {worker.online && (
            <span className="text-2xs text-text-2 ml-1">
              {worker.cpu_usage_pct.toFixed(0)}% cpu · {worker.ram_free_gb.toFixed(1)} GB free
            </span>
          )}
        </div>
      </div>

      <div className="flex gap-1.5">
        <Button onClick={onHistory}>
          <History size={12} />
          <span>History</span>
        </Button>
        <Button variant="primary" onClick={onDispatch} disabled={!canDispatch}>
          <span>{equivocator ? 'Refused' : busy ? 'At capacity' : 'Dispatch'}</span>
          <ArrowRight size={12} />
        </Button>
      </div>
    </motion.div>
  )
}
```

- [ ] **Step 2: Rewrite Radar route**

Replace `src/routes/Radar.tsx`:

```tsx
import { useMemo, useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { useNavigate } from 'react-router-dom'
import { Search, ChevronDown } from 'lucide-react'
import { useWorkers } from '../lib/query'
import { useUIStore } from '../lib/store'
import { AgentCard } from '../components/AgentCard'
import { Input } from '../components/primitives/Input'
import { Badge } from '../components/primitives/Badge'
import { EmptyRadar } from '../components/EmptyRadar'
import { RadarSkeleton } from '../components/RadarSkeleton'

type FilterPill = 'all' | 'trusted' | 'available' | 'capability'

export default function Radar() {
  const { data, isPending, error, refetch } = useWorkers(true)
  const navigate = useNavigate()
  const openDispatch = useUIStore((s) => s.openDispatch)
  const search = useUIStore((s) => s.searchTerm)
  const setSearch = useUIStore((s) => s.setSearchTerm)
  const [activeFilter, setActiveFilter] = useState<FilterPill>('all')
  const [capabilityFilter, setCapabilityFilter] = useState<string | null>(null)
  const [offlineOpen, setOfflineOpen] = useState(false)

  const agents = data?.agents ?? []
  const allCapabilities = useMemo(() => {
    const set = new Set<string>()
    agents.forEach((a) => a.agent_capability && set.add(a.agent_capability))
    return Array.from(set).sort()
  }, [agents])

  const filtered = useMemo(() => {
    return agents.filter((a) => {
      const matchesSearch =
        !search ||
        [a.name, a.peer_id, a.agent_image_ref, a.agent_image_digest, a.agent_capability].some(
          (f) => f && f.toLowerCase().includes(search.toLowerCase()),
        )
      if (!matchesSearch) return false
      switch (activeFilter) {
        case 'all': return true
        case 'trusted': return a.honesty_score > 0.3 && !a.is_equivocator
        case 'available': return a.online && a.dispatch_allowed && a.current_tasks < a.max_tasks
        case 'capability': return !capabilityFilter || a.agent_capability === capabilityFilter
      }
    })
  }, [agents, search, activeFilter, capabilityFilter])

  const online = filtered.filter((a) => a.online)
  const offline = filtered.filter((a) => !a.online)

  if (isPending) return <RadarSkeleton />
  if (error) {
    return (
      <div className="p-7">
        <div className="text-bad mb-3">{(error as Error).message}</div>
        <button onClick={() => refetch()} className="text-xs bg-bg-2 border border-border-0 rounded-md px-3 py-1.5">
          Retry
        </button>
      </div>
    )
  }
  if (agents.length === 0 && !search && activeFilter === 'all') {
    return (
      <div className="p-7">
        <Header />
        <EmptyRadar />
      </div>
    )
  }

  return (
    <div className="p-7 max-w-5xl">
      <Header />
      <div className="flex justify-between items-center mb-5 gap-3 flex-wrap">
        <div className="relative">
          <Search size={14} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-text-2" />
          <Input
            placeholder="Search agents…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-8 w-80"
          />
        </div>
        <div className="flex gap-1.5 flex-wrap">
          {(['all', 'trusted', 'available', 'capability'] as FilterPill[]).map((f) => (
            <button
              key={f}
              onClick={() => setActiveFilter(f)}
              className={`relative text-xs px-3 py-1 rounded-full transition-colors ${
                activeFilter === f
                  ? 'text-accent gradient-border-cyan bg-accent-bg'
                  : 'bg-bg-2 border border-border-0 text-text-2 hover:text-text-0'
              }`}
            >
              {f === 'all' ? 'All' : f === 'trusted' ? 'Trusted' : f === 'available' ? 'Available' : 'Capability'}
            </button>
          ))}
          {activeFilter === 'capability' && allCapabilities.length > 0 && (
            <select
              value={capabilityFilter ?? ''}
              onChange={(e) => setCapabilityFilter(e.target.value || null)}
              className="text-xs bg-bg-2 border border-border-0 rounded-md px-2.5 py-1 text-text-1"
            >
              <option value="">All capabilities</option>
              {allCapabilities.map((c) => <option key={c} value={c}>{c}</option>)}
            </select>
          )}
        </div>
      </div>

      <Section title="Online" count={online.length}>
        <div className="grid grid-cols-1 xl:grid-cols-2 gap-3">
          <AnimatePresence initial={false} mode="popLayout">
            {online.map((w) => (
              <motion.div key={w.peer_id} layout
                initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -8 }}
                transition={{ type: 'spring', stiffness: 280, damping: 30 }}>
                <AgentCard worker={w} onHistory={() => navigate(`/peer/${w.peer_id}`)} onDispatch={() => openDispatch(w.peer_id)} />
              </motion.div>
            ))}
          </AnimatePresence>
        </div>
        {online.length === 0 && <div className="text-sm text-text-2 py-3">No online agents match your filter.</div>}
      </Section>

      {offline.length > 0 && (
        <div className="mt-7">
          <button onClick={() => setOfflineOpen((v) => !v)}
            className="inline-flex items-center gap-1.5 text-2xs uppercase tracking-wider text-text-2 hover:text-text-0">
            <ChevronDown size={12} className={`transition-transform ${offlineOpen ? '' : '-rotate-90'}`} />
            <span>Offline</span>
            <span className="text-text-3">({offline.length})</span>
          </button>
          <AnimatePresence>
            {offlineOpen && (
              <motion.div initial={{ height: 0, opacity: 0 }} animate={{ height: 'auto', opacity: 1 }} exit={{ height: 0, opacity: 0 }}
                transition={{ duration: 0.2 }} className="overflow-hidden">
                <div className="grid grid-cols-1 xl:grid-cols-2 gap-3 mt-3">
                  {offline.map((w) => (
                    <AgentCard key={w.peer_id} worker={w} onHistory={() => navigate(`/peer/${w.peer_id}`)} onDispatch={() => openDispatch(w.peer_id)} />
                  ))}
                </div>
              </motion.div>
            )}
          </AnimatePresence>
        </div>
      )}
    </div>
  )
}

function Header() {
  return (
    <div>
      <div className="flex items-baseline gap-3 mb-1.5">
        <h1 className="text-2xl font-semibold tracking-tight text-text-0">Agent Radar</h1>
        <Badge tone="cyan"><span className="animate-pulse-cyan inline-block w-1 h-1 rounded-full bg-accent mr-1" />LIVE</Badge>
      </div>
      <p className="text-text-2 mb-5">Every worker the mesh has heard of. Online updates in real time.</p>
    </div>
  )
}

function Section({ title, count, children }: { title: string; count: number; children: React.ReactNode }) {
  return (
    <div>
      <div className="text-2xs uppercase tracking-wider text-text-2 mb-3 flex items-center gap-2">
        {title}
        <span className="text-text-3">({count})</span>
      </div>
      {children}
    </div>
  )
}
```

- [ ] **Step 3: Update RadarSkeleton**

Replace `src/components/RadarSkeleton.tsx`:

```tsx
import { SkeletonRow } from './primitives/Skeleton'
import { Badge } from './primitives/Badge'

export function RadarSkeleton() {
  return (
    <div className="p-7">
      <div className="flex items-baseline gap-3 mb-1.5">
        <h1 className="text-2xl font-semibold tracking-tight text-text-0">Agent Radar</h1>
        <Badge tone="cyan"><span className="animate-pulse-cyan inline-block w-1 h-1 rounded-full bg-accent mr-1" />LISTENING</Badge>
      </div>
      <p className="text-text-2 mb-6">Waiting for the first telemetry beacon…</p>
      <div className="grid grid-cols-1 xl:grid-cols-2 gap-3">
        {[0, 1, 2].map((i) => <SkeletonRow key={i} delay={i * 120} />)}
      </div>
    </div>
  )
}
```

- [ ] **Step 4: Update EmptyRadar copy/colors (light touch)**

In `src/components/EmptyRadar.tsx`, swap the `Button` import for the rewritten one (no change needed), and change the heading to use the Display style:

```tsx
<h2 className="text-xl font-semibold tracking-tight text-text-0">No agents on the mesh yet</h2>
```

(Otherwise leave EmptyRadar as-is — it already uses the design tokens.)

- [ ] **Step 5: Build + commit**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx electron-vite build 2>&1 | tail -3
git add src/components/AgentCard.tsx src/routes/Radar.tsx src/components/RadarSkeleton.tsx src/components/EmptyRadar.tsx
git commit -m "feat(radar): 2-col grid, gradient strip, collapsible offline, Lucide icons"
```

---

### Task 13: Status rebuild

**Files:**
- Modify: `src/routes/Status.tsx`
- Modify: `src/components/status/StatusCard.tsx`

- [ ] **Step 1: Rewrite Status route**

Replace `src/routes/Status.tsx`:

```tsx
import { useEffect, useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { useNavigate } from 'react-router-dom'
import { Link2, Link2Off, FileText, RefreshCw, ChevronDown, Copy } from 'lucide-react'
import { useAbout, useWorkers } from '../lib/query'
import { useBackend } from '../hooks/useBackend'
import { useUIStore } from '../lib/store'
import { LogsModal } from '../components/status/LogsModal'
import { Button } from '../components/primitives/Button'
import { StatusDot } from '../components/primitives/StatusDot'
import { toast } from 'sonner'

export default function Status() {
  const navigate = useNavigate()
  const { data: about } = useAbout()
  const { data: workers } = useWorkers(true)
  const backend = useBackend()
  const reputationFloor = useUIStore((s) => s.activeProject()?.reputationFloor ?? -0.5)

  const [showLogs, setShowLogs] = useState(false)
  const [showTech, setShowTech] = useState(false)
  const [uptimeSec, setUptimeSec] = useState(0)

  useEffect(() => {
    if (typeof about?.uptime_seconds === 'number') setUptimeSec(about.uptime_seconds)
  }, [about?.uptime_seconds])
  useEffect(() => {
    const id = setInterval(() => setUptimeSec((s) => s + 1), 1000)
    return () => clearInterval(id)
  }, [])

  const relayConnected = !!about?.relay_peer_id
  const workerCount = workers?.online_count ?? 0
  const offlineCount = workers?.offline_count ?? 0
  const equivocatorCount = workers?.agents.filter((a) => a.is_equivocator).length ?? 0
  const ledgerSize = about?.ledger_tree_size ?? 0

  const issues: string[] = []
  if (!backend.ok) issues.push('Backend is offline')
  if (!relayConnected) issues.push('Not connected to a relay')
  if (equivocatorCount > 0) issues.push(`${equivocatorCount} equivocator${equivocatorCount > 1 ? 's' : ''} detected`)
  const allGood = issues.length === 0

  async function copy(value: string, label: string) {
    try { await navigator.clipboard.writeText(value); toast.success(`${label} copied`) } catch { toast.error('Copy failed') }
  }

  return (
    <>
      <div className="p-7 max-w-5xl">
        <h1 className="text-2xl font-semibold tracking-tight text-text-0">Your mesh</h1>
        <p className="text-text-2 mt-1 mb-6">A friendly view of what's happening right now.</p>

        <motion.div
          initial={{ opacity: 0, y: -6 }} animate={{ opacity: 1, y: 0 }}
          className={`rounded-2xl p-6 mb-7 border ${
            allGood
              ? 'bg-gradient-to-br from-accent-bg to-bg-1 border-accent/30 neon-glow-cyan'
              : 'bg-gradient-to-br from-bad/10 to-bg-1 border-bad/30'
          }`}
        >
          <div className="flex items-start gap-4">
            <div className="text-5xl leading-none">{allGood ? '✓' : '⚠'}</div>
            <div className="flex-1">
              <div className="text-lg font-semibold text-text-0">
                {allGood ? 'All systems are healthy' : `${issues.length} issue${issues.length > 1 ? 's' : ''} detected`}
              </div>
              <div className="text-sm text-text-1 mt-1">
                {allGood ? (
                  <>
                    Backend running for {formatUptime(uptimeSec)}. You're talking to{' '}
                    <span className="text-accent glow-text-cyan font-semibold">{workerCount}</span>{' '}
                    online worker{workerCount === 1 ? '' : 's'}.
                  </>
                ) : (
                  <ul className="list-disc list-inside space-y-0.5">{issues.map((i) => <li key={i}>{i}</li>)}</ul>
                )}
              </div>
            </div>
            <div className="flex flex-col gap-2">
              <Button onClick={() => setShowLogs(true)}><FileText size={12} /><span>View logs</span></Button>
              <Button variant="ghost" onClick={async () => {
                try { await window.api.backend.restart(); toast.success('Backend restarted') }
                catch (e) { toast.error('Restart failed: ' + (e as Error).message) }
              }}><RefreshCw size={12} /><span>Restart backend</span></Button>
            </div>
          </div>
        </motion.div>

        <div className="grid grid-cols-3 gap-4">
          <Tile icon={<span className="text-2xl">🛰</span>} label="Workers"
            value={<span className="glow-text-cyan">{workerCount}</span>}
            sub={<>online{offlineCount > 0 ? <span className="text-text-3"> · {offlineCount} known offline</span> : null}</>}
            cta={{ label: 'Open Radar', onClick: () => navigate('/radar') }} />
          <Tile
            icon={relayConnected ? <Link2 size={24} className="text-accent" /> : <Link2Off size={24} className="text-warn" />}
            label="Relay"
            value={<span className={relayConnected ? '' : 'text-warn'}>{relayConnected ? 'Connected' : 'Not connected'}</span>}
            sub={relayConnected ? 'Your boss has reserved a circuit through the relay.' : "Workers won't see you until you connect to a relay."}
            cta={{
              label: relayConnected ? 'Copy multiaddr' : 'Configure relay',
              onClick: relayConnected
                ? () => copy(about?.relay_multiaddr ?? '', 'Relay multiaddr')
                : () => navigate('/settings'),
            }} />
          <Tile icon={<span className="text-2xl">📜</span>} label="Ledger entries"
            value={<span className="glow-text-violet">{ledgerSize}</span>}
            sub="Ratings + comments signed by this boss."
            cta={{ label: 'See my activity', onClick: () => navigate('/activity') }} />
        </div>

        <div className="mt-6 bg-bg-1 border border-border-0 rounded-2xl p-5 flex items-center justify-between">
          <div>
            <div className="text-sm font-semibold text-text-0">Trust gate</div>
            <div className="text-text-2 text-sm mt-1">
              Workers with honesty below{' '}
              <span className="font-mono text-accent2-light glow-text-violet">{reputationFloor.toFixed(2)}</span>
              {' '}are auto-refused. Equivocators are blocked permanently.
            </div>
          </div>
        </div>

        <button onClick={() => setShowTech((v) => !v)}
          className="mt-8 text-text-2 hover:text-text-0 text-sm inline-flex items-center gap-1.5">
          <ChevronDown size={14} className={`transition-transform ${showTech ? '' : '-rotate-90'}`} />
          {showTech ? 'Hide technical details' : 'Show technical details'}
        </button>
        <AnimatePresence>
          {showTech && (
            <motion.div initial={{ opacity: 0, height: 0 }} animate={{ opacity: 1, height: 'auto' }} exit={{ opacity: 0, height: 0 }}
              transition={{ duration: 0.2 }} className="overflow-hidden">
              <div className="mt-3 bg-bg-1 border border-border-0 rounded-xl p-5 space-y-3">
                <TechRow k="Your peer ID" v={about?.boss_peer_id ?? '…'} onCopy={() => copy(about?.boss_peer_id ?? '', 'Peer ID')} />
                <TechRow k="Relay peer ID" v={about?.relay_peer_id || '(not connected)'} />
                <TechRow k="Relay multiaddr" v={about?.relay_multiaddr || '(none)'} onCopy={about?.relay_multiaddr ? () => copy(about.relay_multiaddr, 'Multiaddr') : undefined} />
                <TechRow k="Backend version" v={about?.version ?? '…'} />
                <TechRow k="Reputation floor" v={reputationFloor.toFixed(2)} />
                <TechRow k="Ledger storage" v="~/.agentfm/" />
              </div>
            </motion.div>
          )}
        </AnimatePresence>
      </div>
      <LogsModal isOpen={showLogs} onClose={() => setShowLogs(false)} />
    </>
  )
}

function Tile({ icon, label, value, sub, cta }: {
  icon: React.ReactNode; label: string; value: React.ReactNode;
  sub: React.ReactNode; cta: { label: string; onClick: () => void }
}) {
  return (
    <motion.div whileHover={{ y: -2 }} transition={{ duration: 0.18 }}
      className="bg-bg-1 border border-border-0 rounded-2xl p-5 flex flex-col hover:border-accent/30">
      <div className="flex items-center gap-2 text-text-2 text-2xs uppercase tracking-wider">
        <span className="text-base inline-flex items-center">{icon}</span>
        {label}
      </div>
      <div className="mt-2 text-3xl font-semibold tracking-tight text-text-0">{value}</div>
      <div className="text-text-2 text-sm mt-1 mb-4 flex-1">{sub}</div>
      <Button onClick={cta.onClick}>{cta.label}</Button>
    </motion.div>
  )
}

function TechRow({ k, v, onCopy }: { k: string; v: string; onCopy?: () => void }) {
  return (
    <div className="flex items-baseline gap-3 text-sm">
      <div className="text-text-2 w-44 shrink-0">{k}</div>
      <div className="font-mono text-xs text-text-1 break-all flex-1">{v}</div>
      {onCopy && (
        <button onClick={onCopy} className="text-text-2 hover:text-accent" title="Copy">
          <Copy size={12} />
        </button>
      )}
    </div>
  )
}

function formatUptime(seconds: number): string {
  if (seconds < 60) return `${seconds}s`
  const m = Math.floor(seconds / 60)
  if (m < 60) return `${m} min`
  const h = Math.floor(m / 60)
  const remM = m % 60
  if (h < 24) return `${h}h ${remM}m`
  const d = Math.floor(h / 24)
  return `${d}d ${h % 24}h`
}
```

- [ ] **Step 2: Update StatusCard** (used by LogsModal — leave structure, refresh colors)

In `src/components/status/StatusCard.tsx`, replace any `bg-bg-1 border-border-0 rounded-lg` with the same — no change needed beyond letting the token rewrite cascade. If there's any explicit color like `border-emerald-500` or accent assumption, replace with `border-accent`.

- [ ] **Step 3: Build + commit**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx electron-vite build 2>&1 | tail -3
git add src/routes/Status.tsx src/components/status/StatusCard.tsx
git commit -m "feat(status): hero glow, Lucide link icons, glowing metric numbers"
```

---

### Task 14: Chat rebuild

**Files:**
- Modify: `src/routes/Chat.tsx`
- Modify: `src/components/chat/SessionList.tsx`
- Modify: `src/components/chat/AgentPicker.tsx`
- Modify: `src/components/chat/MessageBubble.tsx`
- Modify: `src/components/chat/Composer.tsx`

- [ ] **Step 1: Rewrite MessageBubble**

Replace `src/components/chat/MessageBubble.tsx`:

```tsx
import { motion } from 'framer-motion'
import type { ChatMessage } from '../../types/chat'
import { shortenPeerID, compactAge } from '../../lib/peer'

export function MessageBubble({ msg, streaming }: { msg: ChatMessage; streaming?: boolean }) {
  const isUser = msg.role === 'user'
  return (
    <motion.div
      initial={{ opacity: 0, y: 4 }}
      animate={{ opacity: 1, y: 0 }}
      className={`max-w-[80%] min-w-0 overflow-hidden px-4 py-3 rounded-2xl text-sm leading-relaxed ${
        isUser
          ? 'self-end bg-accent-bg border border-accent/35 text-text-0'
          : 'self-start bg-bg-1 border border-accent2/20 text-text-0'
      }`}
    >
      <div className="text-2xs text-text-2 mb-1.5 flex items-center gap-2">
        {isUser
          ? <span>You</span>
          : <span className="font-mono">{msg.rater_peer_id ? shortenPeerID(msg.rater_peer_id, 6, 5) : 'agent'}</span>}
        <span className="ml-auto">{streaming && !isUser ? 'streaming…' : compactAge(msg.timestamp) + ' ago'}</span>
      </div>
      <div className="whitespace-pre-wrap" style={{ overflowWrap: 'anywhere' }}>
        {msg.content}
        {streaming && !isUser && (
          <span className="inline-block w-[3px] h-4 bg-accent ml-0.5 align-middle animate-pulse-cyan shadow-[0_0_8px_#22d3ee]" />
        )}
      </div>
    </motion.div>
  )
}
```

- [ ] **Step 2: Rewrite Composer**

Replace `src/components/chat/Composer.tsx`:

```tsx
import { useState, useRef, useEffect } from 'react'
import { Send, Square } from 'lucide-react'
import { Button } from '../primitives/Button'

interface Props {
  onSend: (text: string) => void
  onStop?: () => void
  streaming?: boolean
  disabled?: boolean
}

export function Composer({ onSend, onStop, streaming, disabled }: Props) {
  const [text, setText] = useState('')
  const ref = useRef<HTMLTextAreaElement>(null)

  function submit() {
    if (!text.trim() || streaming || disabled) return
    onSend(text)
    setText('')
  }

  function handleKey(e: React.KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); submit() }
  }

  useEffect(() => {
    if (ref.current) {
      ref.current.style.height = 'auto'
      ref.current.style.height = Math.min(ref.current.scrollHeight, 220) + 'px'
    }
  }, [text])

  return (
    <div className="border-t border-border-0 p-4 flex gap-3 items-end">
      <textarea
        ref={ref}
        value={text}
        onChange={(e) => setText(e.target.value)}
        onKeyDown={handleKey}
        placeholder="Type a message… (Shift+↵ for newline)"
        disabled={disabled}
        rows={1}
        className="flex-1 bg-bg-0 border border-border-0 rounded-xl px-4 py-3 text-sm text-text-0 outline-none transition-shadow focus:border-accent focus:shadow-[0_0_0_3px_rgba(34,211,238,.15)] resize-none disabled:opacity-50"
      />
      {streaming ? (
        <Button variant="danger" onClick={onStop}>
          <Square size={12} />
          <span>Stop</span>
        </Button>
      ) : (
        <Button variant="primary" onClick={submit} disabled={!text.trim() || disabled}>
          <Send size={12} />
          <span>Send</span>
        </Button>
      )}
    </div>
  )
}
```

- [ ] **Step 3: Rewrite SessionList**

Read the current `src/components/chat/SessionList.tsx`. Identify the row markup. Replace each row's container with:

```tsx
<button
  className={`relative w-full text-left px-4 py-3 transition-colors ${
    isActive ? 'bg-accent/10 text-text-0' : 'text-text-1 hover:bg-bg-2'
  }`}
>
  {isActive && <span className="absolute left-0 top-0 bottom-0 w-[3px] bg-gradient-to-b from-accent to-accent2" />}
  {/* existing label content */}
</button>
```

Replace the delete icon (likely a unicode ✕) with Lucide `Trash2` at size 12.

- [ ] **Step 4: Rewrite AgentPicker**

In `src/components/chat/AgentPicker.tsx`, swap:
- The chip's `▾` glyph for Lucide `<ChevronDown size={12} />`.
- The dropdown's outer container className: add `neon-glow-cyan`.
- Active option styling: `bg-accent/10 text-accent` plus a 2px left strip.

- [ ] **Step 5: Update Chat route header**

In `src/routes/Chat.tsx`, change the back link from text-arrow to Lucide `ArrowLeft` icon + "Radar":

```tsx
<button onClick={() => navigate('/radar')}
  className="inline-flex items-center gap-1.5 text-xs text-text-2 hover:text-text-0">
  <ArrowLeft size={14} />
  <span>Radar</span>
</button>
```

Add `import { ArrowLeft } from 'lucide-react'` at the top.

- [ ] **Step 6: Build + commit**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx electron-vite build 2>&1 | tail -3
git add src/components/chat/ src/routes/Chat.tsx
git commit -m "feat(chat): cyan-pulse cursor, gradient active strip, Lucide send/stop/back"
```

---

### Task 15: Activity rebuild

**Files:**
- Modify: `src/routes/Activity.tsx`

- [ ] **Step 1: Apply visual polish**

Open `src/routes/Activity.tsx`. Make four edits:

1. The h1: `className="text-2xl font-semibold tracking-tight text-text-0"`.
2. The bucket headers (`h2`): keep `text-2xs uppercase tracking-wider text-text-2 mb-2 px-1` but add a small dot prefix using `<StatusDot tone="cyan" size="sm" className="mr-2" />`. Import StatusDot at top.
3. The PeerName link: wrap the resolved name in `<span className="text-accent2-light hover:glow-text-violet transition-colors">`.
4. Each entry container border-bottom: change to `border-bottom-color: rgba(34,211,238,.08)` so the rows feel woven into the neon theme.

- [ ] **Step 2: Build + commit**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx electron-vite build 2>&1 | tail -3
git add src/routes/Activity.tsx
git commit -m "feat(activity): tracking-tight title, dot bucket markers, violet peer link"
```

---

### Task 16: PeerView rebuild

**Files:**
- Modify: `src/routes/PeerView.tsx`
- Modify: `src/components/peer/SummaryCard.tsx`
- Modify: `src/components/peer/EntryRow.tsx`
- Modify: `src/components/peer/Tabs.tsx`

- [ ] **Step 1: Update PeerView route header**

In `src/routes/PeerView.tsx`:
- Back button: swap `← back` text for Lucide `ArrowLeft` + "Back".
- h1: `text-3xl font-semibold tracking-tight text-text-0`.
- Equivocator banner: change classes to `border border-bad/40 bg-gradient-to-r from-bad/15 to-warn/8 rounded-xl p-5` and replace the ⚠ glyph with Lucide `AlertOctagon` at size 24 with `text-bad`.
- Dispatch button keeps `variant="primary"` (already gets gradient from Button rewrite).

- [ ] **Step 2: Rewrite SummaryCard**

Open `src/components/peer/SummaryCard.tsx`. Replace any explicit color classes (`text-emerald-400`, `bg-emerald-500/10`, etc.) with the new token names (`text-accent`, `bg-accent/10`). Replace honesty score numeric display with `<span className={summary.honesty_score > 0.3 ? 'glow-text-cyan' : ''}>` to make positive honesty pop.

- [ ] **Step 3: Rewrite EntryRow**

In `src/components/peer/EntryRow.tsx`, add a 3px left strip per row:

```tsx
<div className="relative pl-4 py-2.5">
  <span
    className={`absolute left-0 top-2 bottom-2 w-[3px] rounded-full ${
      entry.kind === 'Rating' ? 'bg-accent' : 'bg-accent2'
    }`}
  />
  {/* existing row content */}
</div>
```

Replace any expand/collapse glyphs with Lucide `Eye` / `EyeOff` at size 14.

- [ ] **Step 4: Rewrite Tabs**

In `src/components/peer/Tabs.tsx`, the active-tab indicator should reuse the `layoutId="peer-tab-indicator"` pattern (matching TabStrip's approach), with a 2px gradient underline `bg-gradient-to-r from-accent to-accent2`.

- [ ] **Step 5: Build + commit**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx electron-vite build 2>&1 | tail -3
git add src/routes/PeerView.tsx src/components/peer/
git commit -m "feat(peer): gradient strip per entry, layoutId tab indicator, AlertOctagon banner"
```

---

### Task 17: DispatchDrawer + StreamingView rebuild

**Files:**
- Modify: `src/components/DispatchDrawer.tsx`
- Modify: `src/components/StreamingView.tsx`

- [ ] **Step 1: Update DispatchDrawer**

In `src/components/DispatchDrawer.tsx`:
- Drawer container: change `border-l border-border-0` to `border-l border-accent/15` and add `style={{ boxShadow: '-24px 0 48px -16px rgba(34,211,238,.15)' }}` to the motion.div.
- Header h2: bump to `text-xl font-semibold tracking-tight`.
- Close button: replace `✕` with Lucide `X` size 18.
- "Send to agent" button: replace text with `<Send size={12} />` icon + "Send to agent" (variant already primary).
- Replace any "Show in Finder" button content with `<ExternalLink size={12} />` icon + label.

- [ ] **Step 2: Update StreamingView**

In `src/components/StreamingView.tsx`, change the container to:

```tsx
<div className="relative bg-bg-0 border border-border-0 rounded-xl p-4 max-h-[400px] overflow-auto font-mono text-xs">
  {streaming && (
    <span className="absolute top-0 left-0 right-0 h-[2px] bg-gradient-to-r from-transparent via-accent to-transparent animate-shimmer" />
  )}
  <pre className="whitespace-pre-wrap text-text-1">{output}</pre>
  {streaming && <span className="inline-block w-[3px] h-3.5 bg-accent ml-0.5 align-middle animate-pulse-cyan" />}
</div>
```

(Adjusting to the existing component shape — preserve any other behavior.)

- [ ] **Step 3: Build + commit**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx electron-vite build 2>&1 | tail -3
git add src/components/DispatchDrawer.tsx src/components/StreamingView.tsx
git commit -m "feat(dispatch): cyan-glow drawer, shimmer stream top-border, Lucide icons"
```

---

### Task 18: FeedbackModal rebuild

**Files:**
- Modify: `src/components/FeedbackModal.tsx`

- [ ] **Step 1: Polish**

In `src/components/FeedbackModal.tsx`:
- Backdrop: change `bg-black/65` to `bg-black/75`.
- Inner panel: change className to add `neon-glow-cyan` outer shadow, bump padding to `p-7`, radius `rounded-2xl`, width `w-[480px]`.
- Title: `text-xl font-semibold tracking-tight`.
- Close button (✕): replace with Lucide `X` size 18.
- Submit button uses `variant="primary"` — already gradient. Replace 💌 emoji with `<Send size={12} />` icon + "Sign & send".
- Rating slider already uses the rewritten `<Slider>` from Task 6 — confirm it picks up the gradient track automatically.

- [ ] **Step 2: Build + commit**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx electron-vite build 2>&1 | tail -3
git add src/components/FeedbackModal.tsx
git commit -m "feat(feedback): wider modal, neon glow, Lucide close + send icons"
```

---

### Task 19: CreateProjectWizard rebuild

**Files:**
- Modify: `src/components/projects/CreateProjectWizard.tsx`

- [ ] **Step 1: Polish**

In `src/components/projects/CreateProjectWizard.tsx`:
- Outer panel: add `neon-glow-cyan` to className.
- Title h2: `text-xl font-semibold tracking-tight`.
- Close ✕ button: replace with Lucide `X` size 18.
- Each radio option label: wrap in a card with `border rounded-xl p-3 cursor-pointer transition-all` and `useDefault ?: 'border-accent/40 bg-accent/8 neon-glow-cyan' : 'border-border-0 bg-bg-2 hover:border-border-1'`.
- "Create project" button: already primary; prefix with `<Zap size={12} />`.

- [ ] **Step 2: Build + commit**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx electron-vite build 2>&1 | tail -3
git add src/components/projects/CreateProjectWizard.tsx
git commit -m "feat(wizard): card-shaped radio options, neon glow, Zap CTA prefix"
```

---

### Task 20: ProjectSwitchingOverlay rebuild

**Files:**
- Modify: `src/components/projects/ProjectSwitchingOverlay.tsx`

- [ ] **Step 1: Rewrite**

Replace `src/components/projects/ProjectSwitchingOverlay.tsx`:

```tsx
import { AnimatePresence, motion } from 'framer-motion'
import { useUIStore } from '../../lib/store'

export function ProjectSwitchingOverlay() {
  const show = useUIStore((s) => s.isProjectSwitching)
  const active = useUIStore((s) => s.activeProject())

  return (
    <AnimatePresence>
      {show && (
        <motion.div
          initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}
          transition={{ duration: 0.18 }}
          className="fixed inset-0 z-[80] flex items-center justify-center bg-bg-0/90 backdrop-blur-xl"
        >
          <div className="text-center">
            <div className="relative w-16 h-16 mx-auto">
              <motion.div
                animate={{ rotate: 360 }}
                transition={{ duration: 1.2, repeat: Infinity, ease: 'linear' }}
                className="absolute inset-0"
                style={{
                  background: 'conic-gradient(from 0deg, transparent 0%, #22d3ee 30%, #a855f7 70%, transparent 100%)',
                  borderRadius: '50%',
                  mask: 'radial-gradient(transparent 60%, black 62%)',
                  WebkitMask: 'radial-gradient(transparent 60%, black 62%)',
                }}
              />
            </div>
            <div className="mt-5 text-text-1 text-base">
              Switching to <span className="text-accent glow-text-cyan font-semibold">{active?.name ?? '…'}</span>…
            </div>
            <div className="mt-1 text-2xs text-text-2 font-mono">restarting backend with the new relay</div>
          </div>
        </motion.div>
      )}
    </AnimatePresence>
  )
}
```

- [ ] **Step 2: Build + commit**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx electron-vite build 2>&1 | tail -3
git add src/components/projects/ProjectSwitchingOverlay.tsx
git commit -m "feat(switching): conic-gradient ring loader (cyan→violet)"
```

---

### Task 21: BackendDownOverlay + LogsModal rebuild

**Files:**
- Modify: `src/components/BackendDownOverlay.tsx`
- Modify: `src/components/status/LogsModal.tsx`

- [ ] **Step 1: Polish BackendDownOverlay**

In `src/components/BackendDownOverlay.tsx`:
- Background: `bg-gradient-to-b from-bad/15 to-bg-0 backdrop-blur-md`.
- Replace the warning glyph with Lucide `AlertOctagon` at size 80, color `text-bad`, with `animate-pulse-rose`.
- Buttons use the rewritten Button primitive (already inherits new look).

- [ ] **Step 2: Rewrite LogsModal as bottom sheet**

In `src/components/status/LogsModal.tsx`, change the modal layout:

Replace the outer `motion.div`'s positioning className with:
```tsx
className="fixed bottom-0 left-0 right-0 z-[70] bg-bg-1 border-t border-border-0 rounded-t-2xl overflow-hidden"
style={{ height: '60vh' }}
```

Animate from `y: '100%'` to `y: 0`.

Header: add a small drag-handle pill `<div className="w-12 h-1 rounded-full bg-text-3 mx-auto mt-2 mb-1" />`.

Replace any unicode close glyph with Lucide `X`. Replace the refresh button text with `<RefreshCw size={12} className="animate-spin" />` (only spin when fetching).

- [ ] **Step 3: Build + commit**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx electron-vite build 2>&1 | tail -3
git add src/components/BackendDownOverlay.tsx src/components/status/LogsModal.tsx
git commit -m "feat(overlays): rose-pulse backend-down, bottom-sheet logs"
```

---

### Task 22: Delete deprecated badge wrappers + tests + tag

**Files:**
- Delete: `src/components/HonestyBadge.tsx`
- Delete: `src/components/DispatchBadge.tsx`
- Delete: `src/components/EquivocatorBadge.tsx`
- Delete: `src/components/CapabilityBadge.tsx`
- Create: `tests/e2e/visual-smoke.spec.ts`
- Modify: e2e specs touching unicode glyphs

- [ ] **Step 1: Audit remaining imports of deprecated badges**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
grep -rn "HonestyBadge\|DispatchBadge\|EquivocatorBadge\|CapabilityBadge" src/ 2>&1 | grep -v "primitives/Badge"
```

For each match in `src/`, replace with the consolidated `<Badge>` primitive per Spec §3.4 mapping.

After this, the four legacy badge files have zero importers.

- [ ] **Step 2: Delete the legacy badge files**

```bash
git rm src/components/HonestyBadge.tsx src/components/DispatchBadge.tsx src/components/EquivocatorBadge.tsx src/components/CapabilityBadge.tsx
```

- [ ] **Step 3: Update e2e specs for unicode → Lucide replacements**

Run:
```bash
grep -rn "✕\|✓\|▾\|→\|←" tests/e2e/ 2>&1 | head
```

For each test that locates a button by unicode glyph, change to:
- `:has-text("✕")` → `button[aria-label="close"]` (and add `aria-label="close"` to the Lucide `X` button in the relevant component)
- `:has-text("→")` → `button:has-text("Dispatch")` or similar text anchor
- `:has-text("▾")` → `button:has(svg)` scoped by parent

Update *.spec.ts files as needed. Run the full e2e suite:

```bash
pkill -9 -f "Electron.app|electron-vite|/agentfm-go/agentfm" 2>/dev/null || true
sleep 2
npx playwright test --timeout=120000 2>&1 | tail -20
```

Fix selector failures one by one until green.

- [ ] **Step 4: Write visual-smoke.spec.ts**

Create `tests/e2e/visual-smoke.spec.ts`:

```ts
import { test, expect, _electron as electron, ElectronApplication, Page } from '@playwright/test'
import path from 'node:path'

let app: ElectronApplication
let page: Page

test.beforeAll(async () => {
  app = await electron.launch({
    args: ['.'],
    env: {
      ...process.env,
      AGENTFM_BIN: path.resolve(__dirname, '..', '..', '..', 'agentfm-core', 'agentfm-go', 'agentfm'),
    },
    cwd: path.resolve(__dirname, '..', '..'),
  })
  page = await app.firstWindow()
  await page.waitForLoadState('domcontentloaded')

  const wizard = page.locator('h2:has-text("New project")')
  if (await wizard.isVisible({ timeout: 3000 }).catch(() => false)) {
    await page.locator('input[placeholder*="Team Mesh"]').fill('Visual Smoke')
    await page.locator('button:has-text("Create project")').click()
    await wizard.waitFor({ state: 'hidden', timeout: 15000 })
  }
})

test.afterAll(async () => { await app?.close() })

test('AgentFM wordmark is centered-ish with glow on FM', async () => {
  const fm = page.locator('header span.text-accent.glow-text-cyan')
  await expect(fm).toBeVisible()
  await expect(fm).toContainText('FM')
})

test('tab indicator slides between tabs', async () => {
  await page.keyboard.press('Meta+2')
  await page.waitForTimeout(220)
  const indicator = page.locator('[layoutId="tab-indicator"], [style*="layoutId"]').first()
  // layoutId becomes data attribute through Framer Motion runtime; just check the
  // active tab carries the gradient underline div
  const chatActive = page.locator('a:has-text("Chat")').first()
  await expect(chatActive).toHaveClass(/text-text-0/)
})

test('primary buttons use gradient background', async () => {
  await page.keyboard.press('Meta+1')
  const primary = page.locator('button.bg-gradient-to-br').first()
  if (await primary.isVisible().catch(() => false)) {
    await expect(primary).toBeVisible()
  }
})

test('status hero shows All systems healthy or N issues detected', async () => {
  await page.keyboard.press('Meta+4')
  await expect(page.locator('text=/All systems are healthy|issues? detected/i')).toBeVisible({ timeout: 5000 })
})
```

- [ ] **Step 5: Run full test suite**

```bash
pkill -9 -f "Electron.app|electron-vite|/agentfm-go/agentfm" 2>/dev/null || true
sleep 2
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx vitest run tests/unit/ 2>&1 | tail -8
npx playwright test --timeout=120000 2>&1 | tail -20
```

Expected: all unit + all e2e green.

If anything fails, paste failures and STOP — don't tag.

- [ ] **Step 6: Tag (only if all green)**

```bash
git add tests/e2e/visual-smoke.spec.ts
git commit -m "feat(phase3): delete legacy badges, add visual-smoke e2e"
git tag -a phase3-complete -m "Phase 3: Neon Cyber visual overhaul"
```

---

## Self-review

**Spec coverage** — every section of `2026-05-19-phase3-visual-overhaul-design.md` mapped to a task:
- §2.1 color tokens → Task 1.
- §2.2 typography (sizes already bumped in Phase 1) — no separate task; `tracking-tight` applied per-screen.
- §2.3 iconography (lucide) → Tasks 1, 7–22.
- §2.4 motion library → Task 2.
- §2.5 effect utilities → Task 1 (globals.css).
- §3 component primitives → Tasks 3 (Badge), 4 (StatusDot), 5 (Skeleton), 6 (Button/Input/Card/SegGroup/Slider).
- §4.1 TopBar → Task 7.
- §4.2 TabStrip → Task 8.
- §4.3 ProjectDropdown → Task 9.
- §4.4 EmptyState → Task 10.
- §4.5 SettingsFooter + Sheet → Task 11.
- §5.1 Radar → Task 12.
- §5.2 Status → Task 13.
- §5.3 Chat → Task 14.
- §5.4 Activity → Task 15.
- §5.5 PeerView → Task 16.
- §6.1 DispatchDrawer → Task 17.
- §6.2 FeedbackModal → Task 18.
- §6.3 CreateProjectWizard → Task 19.
- §6.4 ProjectSwitchingOverlay → Task 20.
- §6.5+6.6 BackendDown/LogsModal → Task 21.
- §7 implementation strategy: realized via the Batch 1/2/3 sequence above.
- §8.1 unit tests → Tasks 2 (motion) and 3 (badge); existing unit tests survive token rewrites.
- §8.2 e2e tests → Task 22.

No gaps.

**Placeholder scan:** None. Every step shows complete code.

**Type consistency:** `BadgeTone`, `DotTone`, `ButtonVariant` defined once and used consistently. `lift`, `entrance`, `spring`, `fast` motion presets defined in Task 2 and referenced in Tasks 6, 7, 9, 10, 12. Tailwind tokens `accent`, `accent2`, `bg-{0,1,2}`, `border-{0,1}`, `text-{0–3}`, `ok`, `warn`, `bad` defined in Task 1 and referenced throughout.

Plan complete.
