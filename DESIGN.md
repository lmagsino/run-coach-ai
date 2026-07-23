# RunCoach — Design Reference

The visual + UX reference for the chat UI. Locked in Phase 1 (Design); implemented in Phase 4 (Frontend, Vue 3 + Tailwind). The working reference mockup lives at `design/mockup.html` — when in doubt, match it.

**Direction: "Field Notes".** Quiet-luxury running coach. One confident color, generous whitespace, refined type, hairline detail, no clutter. Assistant replies read as flowing text, not chat bubbles — the deliberate anti-chatbot move. Deliberately avoids the AI-default looks (cream + terracotta + serif, near-black + acid accent, broadsheet hairlines).

---

## 1. Principles (the "why", so choices stay consistent)

1. **Restraint reads as expensive.** One accent color, doing all the work. No gradients, no second accent, no decorative flourish. When unsure, remove.
2. **Prose over chrome.** The assistant answers in plain flowing text. Only the *user's* messages get a container. This is what makes it feel like reading, not messaging.
3. **Less data, well placed.** Never a stat table. Weave numbers into the sentence; promote at most **one** figure per answer to the pull-quote treatment.
4. **The multi-source nature shows through words, not loud colors.** "Garmin sleep", "Strava load" in the copy and status lines — no Strava-orange / Garmin-blue branding in v1.
5. **Grounded, or silent.** Copy references what the tools actually found. No generic training advice filler.

---

## 2. Color palette

| Token | Hex | Role |
|---|---|---|
| `paper` | `#EDECE6` | App background (soft greige — warm-neutral, no yellow-cream) |
| `paper-hi` | `#F4F3EE` | Raised surfaces: composer, send-button glyph |
| `ink` | `#1C201C` | Primary text (forest-charcoal, warmer than pure black) |
| `muted` | `#6C6F67` | Secondary text, labels, status steps, captions |
| `faint` | `#DFDDD4` | Faint fills |
| `line` | `#DAD8CE` | Hairline dividers, borders |
| `user` | `#E3E1D8` | User message bubble background |
| `accent` | `#2E5D4E` | Deep pine green — the ONE accent (figures, active dot, send button, checks, live pulse) |
| `accent-soft` | `#3D7161` | Accent hover state |

**Usage rules**
- The accent appears sparingly: the emphasized figure, the send button, the active/live status dot, done-step checkmarks, focus rings. Never as a background fill for large areas.
- Text contrast: `ink` on `paper` and `muted` on `paper` both clear AA for body sizes.
- **Dark mode:** not in v1 scope, but structure tokens so a dark theme can be added later (see mockup's earlier dark exploration in `design/palettes.html` history if revisited).

---

## 3. Typography

**Families**
- **Display** — `Space Grotesk` (400/500/600/700). Wordmark, numbers/figures, eyebrows, labels, tracked uppercase utility text. Carries the personality; deliberately not Inter.
- **Body** — `Hanken Grotesk` (400/500/600). All prose (messages, answers, input). Warm, readable.
- Fallback stack: `ui-sans-serif, system-ui, -apple-system, sans-serif`.
- Load via Google Fonts (see mockup `<link>`). In Phase 4, prefer self-hosting for the local-only build.

**Scale & treatment**
| Element | Family | Size | Weight | Notes |
|---|---|---|---|---|
| Wordmark | Display | 17px | 700 | `-0.02em` tracking; "Coach" in accent |
| Eyebrow / tag / connection | Display | 10–11px | 500–600 | UPPERCASE, `.14–.20em` letter-spacing |
| Agent label ("RUNCOACH") | Display | 10px | 600 | UPPERCASE, `.2em` tracking, muted |
| Answer body | Body | 16px | 400 | line-height 1.62 |
| Answer lede (first line) | Body | 16px | 600 | |
| User message | Body | 15px | 400 | line-height 1.5 |
| Pull-quote figure (number) | Display | 34px | 600 | `-0.02em`, tabular-nums, accent color |
| Figure caption | Body | 13.5px | 400 | muted, max-width ~280px |
| Status step | Body | 14.5px | 400 | muted; active step uses ink |
| Composer input | Body | 15.5px | 400 | |
| Hint line | Display | 11.5px | 400 | centered, muted |

- **Always use `tabular-nums` (`font-variant-numeric: tabular-nums`) for any figures** so numbers align and don't jitter.

---

## 4. Layout

- **Full-height app shell:** fixed `header` → scrollable `main` → fixed `footer` (composer). Flex column, `height: 100%`.
- **Reading measure:** `--measure: 660px`. The thread and composer are centered within this max-width. Header spans full width with its own padding.
- **Header:** brand left (wordmark + "Field Notes" tag), connection status right ("● Strava & Garmin" with live pulse). `padding: 18px 28px`, bottom hairline.
- **Thread:** vertical flex, `gap: 40px` between turns. `main` padding `44px 28px 30px`.
- **Composer:** floating rounded field pinned in footer, `padding: 14px 28px 26px` on footer. Centered hint line beneath.
- **Responsive:** at ≤560px, reduce paddings (`main` → `30px 18px 22px`, header → `15px 18px`), figure number → 28px, answer body → 15px. Body never scrolls horizontally.

---

## 5. Component patterns

### User message
Right-aligned. Background `user`, `border-radius: 16px 16px 5px 16px` (clipped bottom-right corner points back to sender), padding `12px 16px`, max-width 82%.

### Agent answer
Full width, no bubble. Structure:
1. `who` label — "RUNCOACH", uppercase Display, muted, 12px bottom margin.
2. Optional **lede** paragraph (weight 600) — the headline answer in one line.
3. Body paragraphs — 14px gap between, line-height 1.62.
4. Optional **one** figure pull-quote (see below).
5. Closing paragraph.

### Figure pull-quote (the ONLY data treatment)
- Left border `2px solid accent`, `padding-left: 18px`.
- Large Display number in accent + a muted caption beside it (`display:flex; align-items:baseline; gap:14px`).
- **Max one per answer.** If an answer has no single hero number, omit entirely — don't force it.

### Status / thinking indicator (the demo signature)
A vertical list of steps, `gap: 11px`, replacing the answer body while the agent works. Three states:
- **`done`** — green check icon + muted text. e.g. "Checked your Garmin sleep & HRV".
- **`active`** — pulsing accent dot (`@keyframes pulse`, 1.3s) + ink text. e.g. "Reading Strava training load…".
- **`pending`** — hollow muted dot, `opacity: .45` + muted text. e.g. "Comparing against your last block".

Steps are short, first-person-adjacent, and **name the source** ("Garmin sleep", "Strava load"). This is what makes the multi-source reasoning visible. On completion, the whole status block is replaced by the answer.

### Composer
- `paper-hi` background, `1px line` border, `border-radius: 18px`, padding `12px 12px 12px 18px`.
- Focus-within: border → accent, soft accent-tinted shadow.
- Send button: 38×38, `border-radius: 12px`, accent bg, `paper-hi` up-arrow glyph. Hover → `accent-soft`; active → `translateY(1px)`.
- Placeholder: "Ask about your training…".

### Connection indicator
Top-right. Muted uppercase Display label + 6px accent dot with a slow expanding-ring pulse (`@keyframes live`, 2.6s). Quiet signal of the dual-source connection — no dashboard.

---

## 6. Spacing scale

Loosely 4px-based; the recurring values from the mockup:
`5 · 8 · 9 · 10 · 11 · 12 · 14 · 18 · 22 · 28 · 40 · 44` px.
- Turn gap: 40px. Paragraph gap: 14px. Figure vertical margin: ~20–22px. Section/edge padding: 28px (18px mobile).

## 7. Motion

Subtle, purposeful, one at a time:
- **Live connection dot:** slow expanding ring, 2.6s loop.
- **Active status dot:** gentle scale+opacity pulse, 1.3s loop.
- **Composer focus:** 0.18s ease border/shadow transition.
- **Send button:** 0.12–0.18s hover/active.
- **Answer arrival (Phase 4):** subtle fade/slide-in on new agent turns; keep it under ~250ms.
- **`prefers-reduced-motion: reduce`** disables all animation and transition. Non-negotiable.

## 8. Accessibility floor

- Visible keyboard focus: `2px solid accent`, `outline-offset: 2px`.
- All icon-only controls have `aria-label` (send button, etc.).
- Inputs have associated labels.
- Respect reduced motion (above).
- Maintain AA contrast for all text.

---

## 9. Files
- `design/mockup.html` — the canonical reference screen (build to match this).
- `design/palettes.html` — the three-way palette exploration that led here (Topographic / Blue Hour / Chlorophyll). Kept for history.
