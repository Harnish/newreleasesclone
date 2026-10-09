---
name: Release Tracker
description: Quiet dark ledger of software releases; blue marks the one thing to act on.
colors:
  ink-night: "#0f172a"
  slate-surface: "#1e293b"
  slate-panel: "#1a2840"
  slate-hover: "#263548"
  slate-border: "#334155"
  panel-border: "#2d4a6e"
  text-primary: "#e2e8f0"
  text-secondary: "#94a3b8"
  text-muted: "#8796ad"
  signal-blue: "#60a5fa"
  action-blue: "#3b82f6"
  action-blue-deep: "#2563eb"
  success-green: "#059669"
  danger-red: "#dc2626"
  error-text: "#f87171"
  error-bg: "#3b0000"
  ok-text: "#4ade80"
  warn-text: "#fbbf24"
  warn-bg: "#422006"
typography:
  title:
    fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif"
    fontSize: "1.1rem"
    fontWeight: 700
  body:
    fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif"
    fontSize: "0.875rem"
    fontWeight: 400
    lineHeight: 1.55
  label:
    fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif"
    fontSize: "0.72rem"
    fontWeight: 700
    letterSpacing: "0.06em"
rounded:
  xs: "3px"
  sm: "4px"
  md: "5px"
  lg: "8px"
  xl: "10px"
  pill: "20px"
spacing:
  xs: "0.25rem"
  sm: "0.5rem"
  md: "0.875rem"
  lg: "1.1rem"
  xl: "1.5rem"
components:
  button-primary:
    backgroundColor: "{colors.action-blue}"
    textColor: "#ffffff"
    rounded: "{rounded.md}"
    padding: "0.45rem 0.85rem"
  button-primary-hover:
    backgroundColor: "{colors.action-blue-deep}"
  button-outline:
    backgroundColor: "transparent"
    textColor: "{colors.text-secondary}"
    rounded: "{rounded.md}"
    padding: "0.45rem 0.85rem"
  card:
    backgroundColor: "{colors.slate-surface}"
    rounded: "{rounded.lg}"
  input:
    backgroundColor: "{colors.ink-night}"
    textColor: "{colors.text-primary}"
    rounded: "{rounded.md}"
    padding: "0.55rem 0.75rem"
---

# Design System: Release Tracker

## Overview

**Creative North Star: "The Release Ledger"**

A dark, quiet ledger of versions. Everything sits on stepped slate surfaces with hairline borders; nothing shouts. Blue is the single voice: version tags, links, and the primary action. The interface is compact and dense because the job is scanning many projects fast, on a phone or a wide screen.

Platform identity lives in small tinted badges, not in layout. The system is flat: depth comes from tonal steps and 1px borders, never shadows.

**Key Characteristics:**
- Dark slate, one blue accent
- Compact type (0.67–0.95rem) and small radii
- Flat tonal layering, no shadows
- System font stack, no webfonts
- Slide-over panels from the right for add, settings, account

## Colors

Cool slate neutrals with a single sky/azure accent and semantic green/red/amber for state.

### Primary
- **Signal Blue** (#60a5fa): version tags, links, app title. Text-on-dark accent.
- **Action Blue** (#3b82f6): filled primary buttons, checked toggles, focus borders, release-note left rule. Hover deepens to #2563eb.

### Secondary
- **Success Green** (#059669) and **Danger Red** (#dc2626): filled confirm/destructive buttons only. State text uses Ok Green (#4ade80), Error Red (#f87171), Warn Amber (#fbbf24) on tinted dark backgrounds (#052e16, #3b0000, #422006).

### Tertiary
- Platform badge tints, each a dark tint with a bright text pair: GitHub blue (#1e3a5f / #60a5fa), GitLab purple (#2e1f50 / #c084fc), NPM green (#103a26 / #4ade80), PyPI orange (#3b2000 / #fb923c), Docker cyan (#0c2a42 / #38bdf8), Helm greens (#1a2e1a / #4ade80, #86efac).

### Neutral
- **Ink Night** (#0f172a): page background, inputs, release-note wells.
- **Slate Surface** (#1e293b): header, cards, auth card.
- **Slate Panel** (#1a2840): slide-over panels, pagination and select controls (border #2d4a6e).
- **Slate Hover** (#263548): project header hover.
- **Slate Border** (#334155): all hairlines.
- **Text:** primary #e2e8f0, secondary #94a3b8, muted #8796ad (metadata, hints, empty states; ≥4.5:1 on all surfaces).

### Named Rules
**The One Voice Rule.** Blue is the only accent. A screen with two competing accent hues is wrong; platform badges are the sole exception and stay tiny.
**The Tint Pair Rule.** Any colored label is a dark tint background with a bright same-hue text, never a saturated fill.

## Typography

**Display Font:** system stack (-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif)
**Body Font:** same
**Label/Mono Font:** none; no monospace is used today.

**Character:** Neutral OS-native voice; hierarchy comes from weight and tone, not size jumps.

### Hierarchy
- **Title** (700, 1.1rem): app title in header; auth title 1.25rem.
- **Headline** (600, 0.95rem): project names.
- **Body** (400, 0.875rem, 1.55): form fields, version tags (600, blue).
- **Small** (0.75–0.8rem): metadata, dates (#8796ad), release notes (pre-wrap, 1.55).
- **Label** (700, 0.72rem, 0.06em, uppercase): settings section titles; badges 0.67rem, 600, 0.04em.

### Named Rules
**The Tone Not Size Rule.** Separate levels with color step (primary → secondary → muted) before enlarging type.

## Layout

Single column, `max-width: 960px` centered, 1.5rem padding. Cards stack with 0.875rem gap; project header row is name + badge left, count and inline controls right. Controls (refresh/delete) appear on header hover and on focus. Slide-over panels (300px add/account, 380px settings) overlay from the right at z-index 100 rather than reflow content. Spacing rhythm is rem-based in 0.25/0.5/0.75/1.1/1.5 steps. Known gap (see TODO.md): on wide screens the add panel could sit beside the list.

## Elevation & Depth

Flat by default. Depth is tonal: page #0f172a, cards #1e293b, panels #1a2840, with 1px borders (#334155 / #2d4a6e). No box-shadows exist. Panels convey layering by sliding over content with a 0.25s transform; toasts fade/translate in 0.2s.

### Named Rules
**The Flat-By-Default Rule.** Surfaces are separated by tone and hairline only. Do not add shadows.

## Shapes

Small, practical radii: 3px badges, 4px chips and compact controls, 5px buttons and inputs, 6px toasts, 8px cards, 10px auth card, 20px pill toggle track, circle toggle knob. Release notes use a 2px blue left rule with a flat-left, 4px-right corner. Borders are always 1px.

## Components

### Buttons
- **Shape:** 5px radius, `0.45rem 0.85rem` padding, 0.78rem / 500.
- **Primary:** Action Blue fill, white text; hover #2563eb.
- **Green / Red:** confirm and destructive fills (#059669 / #dc2626) with darker hover.
- **Outline:** transparent, #94a3b8 text, 1px #334155; hover fills #1e293b.
- **Ghost:** no border, muted text; hover to primary text. Disabled: 45% opacity.

### Chips
Inline version line: 5 most recent as blue links with age, then a "+N more" text button that expands the stacked list. No pill chrome.

### Cards / Containers
- **Corner Style:** 8px. **Background:** #1e293b. **Border:** 1px #334155. **Shadow:** none. **Padding:** 0.875rem 1.1rem on headers, 0.75rem 1.1rem on version rows.

### Inputs / Fields
- **Style:** #0f172a fill, 1px #334155, 5px radius, 0.875rem.
- **Focus:** outline removed, border shifts to Action Blue. Keyboard focus adds a 2px Signal Blue outline.

### Navigation
Header bar on Slate Surface with bottom hairline: emoji + title left, account, bell, sign out as ghost buttons right. Auth uses a two-tab segmented control on an Ink Night track.

### Toggle
36×20 pill, #334155 track, white 14px knob; checked track Action Blue.

### Toast
Fixed top-right, tinted semantic background and border (ok / err / inf), fades and slides 6px.

## Do's and Don'ts

### Do:
- **Do** separate surfaces by tone step and a 1px #334155 border.
- **Do** use Signal Blue for versions and links, Action Blue for filled actions.
- **Do** give every colored label a dark-tint background with bright same-hue text.
- **Do** keep type compact (0.75–0.95rem) and radii between 3px and 10px.
- **Do** keep touch targets usable on phones even though visuals are compact.

### Don't:
- **Don't** add box-shadows or gradients.
- **Don't** introduce a second accent hue outside platform badges.
- **Don't** use saturated fills for labels or badges.
- **Don't** hide controls behind hover only; they are always visible on touch and on focus.
