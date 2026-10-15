---
name: Directory Desk
description: A compact identity-operations workspace built on KiteFrame.
colors:
  primary: "#12344d"
  accent: "#2c5cc5"
  brand: "#00a886"
  paper: "#f5f7f9"
  surface: "#ffffff"
  copy: "#475867"
  rule: "#ebeff3"
typography:
  body:
    fontFamily: "Space Grotesk Variable, Space Grotesk, Segoe UI, system-ui, sans-serif"
    fontSize: "calc(11.5px * var(--kf-density))"
    fontWeight: 400
    lineHeight: 1.45
rounded:
  sm: "var(--kf-radius-sm)"
  md: "var(--kf-radius-md)"
  lg: "var(--kf-radius-lg)"
spacing:
  compact: "var(--kf-space-1)"
  control: "var(--kf-space-2)"
  section: "var(--kf-space-3)"
  page: "var(--kf-space-4)"
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.surface}"
    rounded: "{rounded.sm}"
  card:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.primary}"
    rounded: "{rounded.lg}"
---

# Design System: Directory Desk

## Overview

**Creative North Star: "The Control Ledger"**

Directory Desk is a dense, calm operations console. It uses the KiteFrame component system as its visual and behavioral authority: compact controls, a persistent navy rail, white working surfaces, precise rules, and teal or blue only where state or action requires them.

**Key Characteristics:** compact density; clear operational hierarchy; restrained semantic color; tabular data; explicit state.

## Colors

Navy carries structure and primary action, teal communicates healthy progress, cobalt identifies focus and navigation, and cool neutrals keep large data surfaces quiet.

**The Semantic Color Rule.** Accent colors communicate action, focus, or state; they are not decoration.

## Typography

Space Grotesk Variable with Space Grotesk, Segoe UI, and system fallbacks is used throughout. Hierarchy comes from weight and compact size steps rather than oversized display text. Operational values use tabular numerals where alignment matters.

## Layout

Use `AppLayout`, `Sidebar`, and `TopBar` for every primary surface. Desktop pages use a compact 1600px maximum work area. Grids collapse below 980px; the rail becomes an explicit mobile navigation below 700px; dense tables scroll within their own surface instead of widening the page.

## Elevation & Depth

The system is flat by default. Borders and tonal surfaces establish most depth; a single soft ambient shadow is reserved for the top-level status strip.

## Shapes

Controls use KiteFrame's small radius, grouped surfaces use medium or large radius, and pills are reserved for status or progress tracks.

## Components

Use `@kiteapps/ui` primitives before custom markup. Buttons, fields, status, tables, notifications, the shell, and cards inherit their states from the library. Custom dashboard graphics consume only `--kf-*` semantic tokens.

## Do's and Don'ts

### Do:
- **Do** keep controls compact and actions adjacent to the data they affect.
- **Do** label illustrative data and preserve keyboard-visible focus.
- **Do** use Lucide stroke icons through the component system.

### Don't:
- **Don't** hardcode feature-level colors outside the token block.
- **Don't** turn each row or setting into an isolated floating card.
- **Don't** use gradients, oversized marketing type, or decorative motion in operations screens.
