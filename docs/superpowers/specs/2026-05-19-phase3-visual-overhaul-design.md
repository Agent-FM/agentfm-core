# AgentFM Desktop — Phase 3: Neon Cyber Visual Overhaul

**Status:** Approved 2026-05-19.
**Authors:** Saif (product), Claude (design).
**Scope:** Comprehensive visual rebuild of all 12 screens in the Neon Cyber direction (cyan primary + violet secondary, medium-intensity motion). No functional changes; no backend changes.
**Out of scope:** Phase 4 features (starred agents, chat reset, chat inspector, assets folder, analytics dashboards).

## 1. Why

Phases 1–2 fixed the data model, navigation, and bug surface — but left a flat, monochrome, low-affordance look. The user feedback was consistent: *dull, no animation, text too small, doesn't feel premium.* Phase 3 makes the app *look like the thing it is* — a peer-to-peer agent mesh that's alive with live workers, streaming responses, and signed reputation events.

The brand language we converged on through the visual brainstorm:

- **Neon Cyber** aesthetic. Pitch-black base, electric cyan and violet accents, sharp glows on live elements. Reads as "technical, current, mesh-native."
- **Balanced palette.** Cyan = action / primary. Violet = info / state / live signal. Semantic colors (lime / amber / rose) reserved for status badges.
- **Medium motion.** Spring physics on interactives, pulse on live indicators, lift + soft glow on hover. Alive but not noisy.
- **Comprehensive scope.** Every screen gets bespoke treatment, not just a token swap.

## 2. Design language fundamentals

### 2.1 Color tokens

The Tailwind `extend.colors` block is rewritten. Existing class names mostly survive; values shift.

```js
colors: {
  bg:     { 0: '#07090d', 1: '#0d1117', 2: '#131922' },
  border: { 0: '#1a2030', 1: '#283047' },
  text:   { 0: '#f3f6fa', 1: '#c4cdd9', 2: '#7a8595', 3: '#4a5566' },
  accent: {
    DEFAULT: '#22d3ee',        // cyan-400
    dim:     '#06b6d4',        // cyan-500
    light:   '#67e8f9',        // cyan-300
    high:    '#a5f3fc',        // cyan-200
    fg:      '#07090d',
    bg:      '#062a36',        // accent-tinted background
  },
  accent2: {
    DEFAULT: '#a855f7',        // violet-500
    dim:     '#7e22ce',
    light:   '#d8b4fe',
    bg:      '#1f0a36',
  },
  ok:    '#84cc16',            // lime
  warn:  '#f59e0b',
  bad:   '#f43f5e',
}
```

The accent CSS variables in `tokens.css` get replaced:

```css
:root {
  --accent: #22d3ee;
  --accent-fg: #07090d;
  --accent-bg: #062a36;
  --accent-2: #a855f7;
  --accent-2-bg: #1f0a36;
}
```

The Phase 1 `data-accent` attribute (emerald / violet / rose) is **removed** — the new system is single-palette by design.

### 2.2 Typography

| Role | Family | Size | Notes |
|------|--------|------|-------|
| Display (page h1) | system-ui, Inter | 26px | `letter-spacing: -0.02em`, `font-feature-settings: "ss01","cv11"` |
| Section (h2, h3) | system-ui, Inter | 18px / 15px | semibold |
| Body | system-ui, Inter | 16px | line-height 1.55 |
| Small | system-ui, Inter | 13px | metadata, badges, captions |
| Mono (code, peer IDs, multiaddr) | JetBrains Mono, SF Mono | 12.5px | used for any structural identifier |

Tailwind `fontSize` map updated to match. Existing `text-xs / text-sm / text-base / text-lg / text-xl / text-2xl` classes survive; values shift one notch larger than Phase 1.

### 2.3 Iconography

Two-track icon system:

- **Personality (emoji, kept):** project icon (📁), empty-state hero (🛰), chat (💬), settings (⚙), receipts/ledger (📜). Carries the brand voice.
- **Functional (Lucide React, new):** close (`X`), copy (`Copy`), expand (`ChevronDown`), arrow (`ArrowRight`), search (`Search`), download (`Download`), eye (`Eye`), refresh (`RefreshCw`), and similar. Single-stroke, 14–16px, currentColor.

**New dependency:** `lucide-react` (tree-shakable, ~2 KB per icon imported).

Existing ad-hoc unicode glyphs (`✕`, `▾`, `→`, `▸`, `←`) get systematically replaced with Lucide equivalents in scope-touched files. Out-of-scope files keep their unicode glyphs until they're rewritten in a later phase.

### 2.4 Motion system

A small Framer Motion preset library at `src/lib/motion.ts`:

```ts
export const fast    = { duration: 0.15, ease: [0.4, 0, 0.2, 1] }
export const spring  = { type: 'spring', stiffness: 320, damping: 28 } as const
export const entrance = {
  initial: { opacity: 0, y: 6 },
  animate: { opacity: 1, y: 0 },
  transition: { type: 'spring', stiffness: 280, damping: 30 },
} as const
export const lift = {
  whileHover: { y: -2 },
  whileTap:   { scale: 0.98 },
  transition: { type: 'spring', stiffness: 380, damping: 26 },
} as const
```

Used uniformly across cards, buttons, list rows. Cards stop importing `whileHover={{ y: -1 }}` directly — they spread `lift`.

CSS-level utilities (added to `globals.css`):

```css
.pulse-cyan   { animation: pulseCyan   2.2s ease-in-out infinite; }
.pulse-violet { animation: pulseViolet 2.2s ease-in-out infinite; }
@keyframes pulseCyan   { 0%,100% { opacity: 1; transform: scale(1) } 50% { opacity: .55; transform: scale(.85) } }
@keyframes pulseViolet { 0%,100% { opacity: 1; transform: scale(1) } 50% { opacity: .55; transform: scale(.85) } }

.shimmer {
  position: relative; overflow: hidden;
}
.shimmer::after {
  content: ''; position: absolute; inset: 0;
  background: linear-gradient(90deg, transparent, rgba(34,211,238,.08), transparent);
  transform: translateX(-100%);
  animation: shimmer 1.8s cubic-bezier(.4,0,.6,1) infinite;
}
@keyframes shimmer { 100% { transform: translateX(100%) } }
```

### 2.5 Effect utilities

Tailwind plugin or `globals.css` utilities:

- `.neon-glow-cyan` — `box-shadow: 0 0 0 1px rgba(34,211,238,.3), 0 0 18px -2px rgba(34,211,238,.35)`
- `.neon-glow-violet` — equivalent for accent-2
- `.glow-text-cyan` — `text-shadow: 0 0 8px rgba(34,211,238,.5)` (used on streaming counts, live numbers)
- `.gradient-border-cyan` — bordered element with an animated `linear-gradient(135deg, cyan→violet)` border using `border-image` or a `::before` overlay. Used on the primary CTA (`Dispatch task`, `Send`) and the active TabStrip indicator.

## 3. Component primitives

Each primitive gets a focused rewrite. Same exported API; new internals.

### 3.1 `<Button>`

- Variants: `default`, `primary`, `ghost`. Add `danger`.
- `primary` uses a cyan→violet gradient on the background with a subtle inner shadow; on hover the gradient hue rotates ~6deg; on tap scales 0.98.
- `default` uses bg-2 with border-1, hover bumps to border-cyan/40 + soft glow.
- `ghost` is text-only with hover underline (cyan).
- `danger` uses bg-rose/15 with border-rose/40, hover deepens.

All variants animate via the `lift` preset.

### 3.2 `<Input>`

- Border-0 default; on focus, border-cyan/50 + outer ring `0 0 0 3px rgba(34,211,238,.15)`.
- Inline `LoaderIcon` (cyan, spinning) when `loading` prop is set.
- New `<Field>` wrapper component standardizing label + helper text + error states (no separate `<label>` calls scattered everywhere).

### 3.3 `<Card>`

- bg-1, border-0 by default; `hover:border-cyan/35` and `hover:shadow-[0_10px_30px_-12px_rgba(34,211,238,.18)]`.
- New `density` prop (`default | compact | spacious`) controlling padding scale.
- New `live` prop — when true, the top-left corner gets a 1px cyan-pulse halo (used on the active backend health card).

### 3.4 Badges

Refactored into one `<Badge>` primitive with `tone` prop (`cyan | violet | lime | amber | rose | neutral`). Replaces:

- `HonestyBadge` → `<Badge tone={honesty>0.3?'lime':honesty<-0.5?'rose':'neutral'} mono>`
- `DispatchBadge` → `<Badge tone={allowed?'lime':'rose'}>`
- `EquivocatorBadge` → `<Badge tone="rose">⚠ equivocator</Badge>`
- `CapabilityBadge` → `<Badge tone="violet" mono>`
- Status pills, model pills, etc.

The original badge files are kept as thin wrappers for backward compat during the transition, then deleted at the end of the implementation.

### 3.5 `<Slider>` and `<SegGroup>`

- `<Slider>` track gets a cyan→violet gradient fill; thumb is a glowing cyan dot.
- `<SegGroup>` active option uses a `gradient-border-cyan` underline; inactive options are text-2.

### 3.6 New: `<StatusDot>` and `<PulseDot>`

Replaces the inline `<span className="w-2 h-2 rounded-full bg-..." />` pattern scattered across AgentCard, MessageBubble, etc.

```tsx
<StatusDot tone="cyan" pulse />     // live, pulsing
<StatusDot tone="neutral" />        // sleeping
<StatusDot tone="rose" />           // equivocator
```

### 3.7 New: `<Skeleton>` and `<SkeletonRow>`

A shared shimmer-based loader. Replaces RadarSkeleton's bespoke implementation with composable primitives.

## 4. Chrome screens

### 4.1 TopBar

```
┌──────────────────────────────────────────────────────────┐
│  AgentFM   [📁 My Project ▾]              relay: 127… 📋 │
└──────────────────────────────────────────────────────────┘
```

- Background: `bg-0` with a 1px bottom border that is a cyan→transparent gradient (`background: linear-gradient(90deg, rgba(34,211,238,.3) 0%, rgba(34,211,238,0) 50%, rgba(168,92,247,.2) 100%);`).
- Wordmark "AgentFM" gets a subtle glow on the "FM" suffix only (`glow-text-cyan` opacity 0.7).
- ProjectPill chip: bg-1, `gradient-border-cyan` on the chip's left edge (4px wide vertical accent strip indicating active project). Chevron is Lucide `ChevronDown`.
- Relay text: mono, 12.5px. Hover surface gets a `Copy` Lucide icon that fades in. Clicking copies and triggers a 600ms cyan flash on the text.

### 4.2 TabStrip

```
─Radar──┬─Chat─┬─Activity─┬─Status─
        █████  
```

- Each tab: 44px height (was 36), regular weight inactive, semibold + cyan on active.
- Active tab indicator is a 2px gradient (cyan→violet) at the bottom, animated with Framer Motion `layoutId="tab-indicator"` so it slides between tabs on click. Smooth feel matches Linear / Raycast tab bars.
- Hover state on inactive tabs: text fades from text-2 → text-0; no underline movement.

### 4.3 ProjectDropdown

- Chip: replaced unicode `▾` with Lucide `ChevronDown` (rotates 180° when open).
- Menu surface: bg-1, border-0, 12px radius (up from 6), `neon-glow-cyan` outer shadow at low intensity.
- Each project row: 36px tall, hover bg-2 with a 1px cyan left-border that animates in from `width: 0`. Active row carries the same left-border permanently.
- "+ New project" row: cyan text, Lucide `Plus` icon, hover glow.
- "Delete current project" row: rose text, Lucide `Trash2` icon, hover bg-rose/8 background tint.
- Entrance animation: spring from `y: -8, opacity: 0` (overrides current `duration: 0.12`).

### 4.4 EmptyState

```
              ╭──────────────────────╮
              │                      │
              │         🛰           │  ← animated, drifting
              │                      │
              │  Welcome to AgentFM  │
              │                      │
              │ [Create project ⚡]  │  ← cyan→violet gradient
              ╰──────────────────────╯
                  ◌◌◌◌◌◌◌◌◌◌◌◌◌◌       ← faint scanlines
```

- Centered card on bg-0 with **scanlines** background pattern at 4% opacity over a radial-gradient cyan→bg-0 (top) and violet→bg-0 (bottom). Drift the gradients slowly (60s loop) to give the page a living feel even when idle.
- Hero 🛰 emoji at 80px, gentle 6px float animation (4s ease-in-out infinite).
- Title in Display type.
- CTA: primary Button (cyan→violet gradient), Lucide `Zap` icon prefix.

### 4.5 SettingsFooter + SettingsSheet

- Footer button gets a left-side dot indicator: a tiny cyan pulse dot if the backend is healthy, amber if degraded, rose if down. Replaces the topbar's current `relay ✓ stable` status text (cleaner separation).
- SettingsSheet: same slide-in, but width 420px (was 380). Theme picker is a 3-button `<SegGroup>`; selected option gets `gradient-border-cyan`. Add a **backend status block** at the top of the sheet: live worker count, relay status, backend uptime, with a `<Button>` to view logs.

## 5. Content screens

### 5.1 Radar

The hero screen of the app. Currently a flat list; the rewrite gives it a sense of place.

- **Header strip:** Title "Agent Radar" in Display; `<Badge tone="cyan" pulse>LIVE</Badge>` to the right; subtitle copy unchanged.
- **Filters row:** Same 4 pills (All / Trusted / Available / Capability). Active pill uses `gradient-border-cyan` instead of background fill — cleaner.
- **Search input:** Lucide `Search` icon prefix; cyan focus ring as defined in 3.2.
- **Agent grid → animated cards:**
  - Card layout shifts from horizontal row to a 2-column responsive grid (1 column at < 900px).
  - Each card has a top-left 8px cyan→violet vertical strip when the worker is online. Strip uses `pulse-cyan` when streaming.
  - Worker name in Display style at 17px; capability `<Badge tone="violet" mono>`; honesty score with `glow-text-cyan` if > 0.3 (positive feedback feels good visually).
  - Hover state: card lifts 2px, neon-glow-cyan applies, the cyan strip widens to 12px (Framer Motion `whileHover`).
  - "Dispatch ↵" button uses primary gradient variant.
- **Online/Offline sections:** kept, but Offline section auto-collapses behind a "Show offline (N)" expander. Faded card style for offline (opacity 65%, no strip).
- **Empty state (no agents):** `<EmptyRadar>` rewritten to use the new visual language — the worker command block gets a cyan border + Lucide `Copy` button.

### 5.2 Status — "Your mesh"

Phase 1's friendly redesign survives; visual layer rebuilt.

- Hero banner: gradient from `accent-bg` (cyan tint) through `bg-1` for healthy state; `bg-rose/10 → bg-1` for issues. Icon glows.
- Workers tile: large number with `glow-text-cyan`; tiny pulse dot beside the number.
- Relay tile: when connected, a 16px Lucide `Link2` icon glows cyan; when disconnected, `Link2Off` icon glows amber.
- Ledger tile: counter animates up from 0→current with a 400ms spring on first render.
- Trust gate strip: the floor number gets `glow-text-violet` (since trust is an info/state signal, not action).
- Technical details collapsible — chevron rotates, content slides in with stagger (each row entering 30ms after the previous).

### 5.3 Chat

- Two-column shell (sessions / messages) gets a vertical divider that fades in cyan during streaming.
- Session list row: 48px tall, hover cyan-tinted bg, active session gets a left-side gradient strip.
- AgentPicker chip: same neon-glow-cyan halo on hover; dropdown reuses ProjectDropdown's animation.
- **Message bubble:**
  - User bubbles: bg with a faint cyan diagonal-stripe pattern at 3% opacity. Border cyan/30.
  - Assistant bubbles: bg-1, border violet/20 when from a peer with high honesty (>0.3), border-0 otherwise.
  - Streaming indicator (the `▌` blinking cursor) becomes a 2px cyan vertical bar with `pulse-cyan` glow.
  - Long messages: cap height at 60vh with a gradient-fade overflow at the bottom + "Show more" affordance.
- **Composer:**
  - Textarea border switches to cyan on focus with the standard 3px ring.
  - Send button: primary gradient. Lucide `Send` icon. Disabled state: 40% opacity, no gradient.
  - Stop button: rose tone with Lucide `Square` icon while streaming.
- Back-to-Radar link: stays. Promote to a Lucide `ArrowLeft` icon + "Radar" text.

### 5.4 Activity ("My activity")

- Date-bucket headers (Today / Yesterday / Older) typeset in Display style.
- Each entry row gets a left-side 3px gradient strip whose tone signals the entry kind: cyan for ratings (`accent`), violet for comments (`accent2`).
- PeerName link gets `glow-text-cyan` on hover.
- Empty state: 📜 emoji at 64px with a 4s drift float.

### 5.5 PeerView

- Header pushes the peer's display name into Display type at 28px; full peer ID in mono text-2.
- Action buttons ("Open in chat", "Dispatch task") right-aligned; "Dispatch task" is the primary gradient CTA.
- **Equivocator banner** (already added in Phase 1): visual upgrade to a `rose/15` background with a 1px gradient border (rose→amber) — communicates urgency without screaming.
- **SummaryCard:** 3-column grid; each column header in `Badge tone="cyan"` style. Honesty score gets `glow-text-cyan` if positive.
- **Tabs (All / Ratings / Comments):** reuse `<Tabs>` primitive; active underline is `gradient-border-cyan`.
- **EntryRow:** denser, with the left-side gradient strip pattern from §5.4. Comment expansion (CID body fetch) gets a small Lucide `Eye` icon that rotates to `EyeOff` when expanded.

## 6. Overlays

### 6.1 DispatchDrawer

- 52% wide drawer; spring entrance from the right (unchanged).
- Header: worker name in Display type at 20px; status badges row below.
- "Prompt" textarea: large (16px font), focus ring cyan.
- Send button: primary gradient with Lucide `Send` icon.
- **Live stream block:** mono font; streaming cursor uses the new cyan-pulse bar; chunk arrivals animate text fade-in for the most recent token. The container has a thin gradient top-border that animates (shimmer) while streaming.
- Artifact row: bg-1 card with Lucide `FileArchive` icon (cyan); "Show in Finder" button gets a Lucide `ExternalLink` icon.
- Post-completion CTAs: "Dispatch another" (default) + "Leave feedback 💌" (primary gradient).

### 6.2 FeedbackModal

- 480px modal (slight bump). Backdrop dim deeper (`black/75`) with a `bg-2/30` overlay for color depth.
- Title in Display type.
- Comment textarea: same cyan focus ring.
- Rating slider: track is a cyan→neutral→violet gradient; thumb glows in the current rating's tone.
- "Sign & send 💌" primary gradient button; disabled state visibly dimmer.

### 6.3 CreateProjectWizard

- 460px modal (unchanged).
- Title in Display type.
- Name input: large, focus ring cyan.
- Relay radio cards: each card 60px tall with a subtle hover lift; selected card gets `neon-glow-cyan` + cyan left-strip.
- Custom-multiaddr input slides in when "Custom multiaddr" is selected (height animation, 220ms).
- "Create project" primary gradient button with Lucide `Zap` prefix.

### 6.4 ProjectSwitchingOverlay

- Full-screen `bg-0/90` with a 24px backdrop blur.
- Center: a 64px ring loader using SVG with a conic gradient cyan→violet stroke that rotates at 1.2s linear.
- Text: "Switching to {name}…" in Display type; subtext mono.

### 6.5 BackendDownOverlay

- Visual upgrade: rose tint (rose-950/40 → bg-0 gradient). Hero icon `AlertOctagon` (Lucide) at 80px, animated `pulse-rose` (custom keyframes, defined inline). Restart and View Logs buttons.

### 6.6 LogsModal

- Modal becomes a slide-up sheet from the bottom (60% viewport height) — fits more lines.
- Log lines mono 12.5px. ANSI escape rendering stays stripped (already implemented).
- Auto-scroll to bottom toggle.
- "Refresh" button uses Lucide `RefreshCw` icon, spins while fetching.

## 7. Implementation strategy

Implementation lands in three commits-batches against this spec:

1. **Foundation (Tasks 1–6).** Tokens, type scale, motion lib, lucide-react install, primitive components, badge consolidation, status dot, skeleton. App still looks ~90% Phase 2 because no screen has been rewritten yet — but every primitive renders with the new look.
2. **Chrome (Tasks 7–11).** TopBar, TabStrip, ProjectDropdown, EmptyState, SettingsFooter+Sheet. Every screen suddenly looks ~70% new because the chrome surrounds them.
3. **Content + Overlays (Tasks 12–22).** Each remaining screen rewritten one at a time, tested via the existing e2e suite. The suite should mostly survive — selectors change for renamed badges and Lucide-replaced unicode glyphs.

## 8. Testing

### 8.1 Unit
- Existing unit tests survive (none test visuals).
- Add a small `motion.test.ts` that verifies the exported preset objects have the expected shape (snapshot).
- Add `badge.test.tsx` covering the consolidated `<Badge>` primitive (tone → class mapping).

### 8.2 E2E
- Update selectors where unicode glyphs (✕ ▾ etc.) were replaced with Lucide icons. Where possible, target `aria-label="close"` etc. instead of the glyph itself.
- Tab indicator selector switches from `:has-text("X").has-class(border-accent)` to looking for the `[layoutId="tab-indicator"]` element under the active tab.
- Add `e2e/visual-smoke.spec.ts` — launches, navigates each route, asserts visible primary CTAs (cyan-gradient buttons), and snapshots the rendered HTML to detect accidental regressions.

### 8.3 Manual visual smoke
- Walk each screen.
- Confirm: pulse dots are visible on live elements, gradient buttons have hue-rotate on hover, tab indicator slides smoothly between tabs, empty-state scanlines and drift gradients are subtle (don't dominate).
- Reduce-motion check: respect `prefers-reduced-motion` by neutralizing the pulse / drift / shimmer animations to a static state.

## 9. Out of scope (Phase 4+)

- Starred agents per project
- Chat reset action
- In-chat agent inspector panel (capabilities, hardware)
- Assets folder browser
- Per-project analytics via `/metrics`
- Light theme polish (theme tokens exist; light variant of the neon palette is reserved for Phase 4)
- Custom user-pickable accent colors (single-palette is intentional for Phase 3)
- Project icon/color picker (Phase 2 explicitly removed these)

## 10. Risks

- **Animation budget on lower-end machines.** Idle pulses and shimmers cost frames. Mitigation: all idle animations honor `prefers-reduced-motion`; pulse rate capped at 2.2s (not 1s).
- **Visual fatigue at 8-hour sessions.** Neon glows can become tiring. Mitigation: glow intensity is intentionally restrained (15–35% alpha shadows, not high-intensity); scanline + drift on EmptyState only (where the user lingers least).
- **Lucide bundle size.** Imported per-icon, ~2 KB each. Mitigation: limit to ~15 icons across the app; tree-shaken at build time.
- **e2e selector churn.** Glyph replacements break selectors. Mitigation: every replaced glyph gets a stable `aria-label` or `data-testid`; tests use those.
- **Theme variable inheritance during transition.** Phase 1's `data-accent` attribute drove violet/rose alternates — being removed. Mitigation: a one-commit cleanup at the end of the implementation grep-removes any remaining references.
