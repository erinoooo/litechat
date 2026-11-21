# liteChat design system

liteChat is a small, self-hosted tool you keep open all day. The UI should feel
like a well-made instrument: quiet, exact and consistent, with the conversation
as the only thing that draws the eye.

**Where things live**

| | |
|---|---|
| Tokens | `src/app.css` (colors, type, radius, shadow, motion, layout sizes) |
| Components | `src/lib/ui/*`, imported from `src/lib/ui` |
| Live reference | `/_ui` in the running app (`src/lib/components/Reference.svelte`) |
| Guard | `npm run lint:ui` (runs on every `npm run build` and `make test`) |

## Principles

1. **One accent, used for meaning.** Pine marks the primary action, focus,
   selection, "on" states and your own prompts. Nothing else is colored.
2. **Borders separate, shadows float.** In-flow surfaces never have shadows.
   Only things that float above the page (menus, dialogs, toasts) get
   `shadow-overlay`.
3. **Radius follows layer.** `rounded-sm` (4px) inline bits: kbd, badges,
   inline code. `rounded-md` (8px) controls and blocks: buttons, inputs,
   rows, code blocks, the composer. `rounded-lg` (12px) floating layers.
   `rounded-full` only for dots and switches.
4. **Lists, not cards.** Groups of items are hairline-separated rows (`List` +
   `ListRow`), not grids of boxed cards.
5. **The transcript is a document.** Prompts read as the heading of their
   turn (pine rule, medium weight); replies are body text. No chat bubbles.
6. **Motion answers an action.** Menus and dialogs fade in when opened;
   nothing animates on its own except the streaming caret and spinners.
   No entrance animations on content. `prefers-reduced-motion` is respected.
7. **Plain words.** Sentence case everywhere. No ALL-CAPS labels, no
   tracked-out eyebrows, no "A · B · C" meta strings, no trailing arrows.

## Tokens

### Color (semantic; light / dark)

| Token | Use |
|---|---|
| `canvas` | Reading surface |
| `panel` | Sidebar, composer well, code blocks, expanded details |
| `raised` | Menus, dialogs, toasts |
| `hover` | Hover and selected backgrounds |
| `line` / `line-strong` | Hairlines / control borders |
| `ink` / `ink-2` / `ink-3` | Primary text / secondary / tertiary and placeholders |
| `pine` / `pine-ink` / `pine-wash` | Accent / text on accent / tinted accent background |
| `red` / `red-wash` | Errors and destructive actions |
| `scrim` | Backdrop behind dialogs |

Use them as Tailwind utilities: `bg-panel`, `text-ink-2`, `border-line`,
`bg-pine/88`. Tailwind's own palette is removed, so `bg-gray-100` does not exist.

### Type

IBM Plex Sans for everything; IBM Plex Mono only for code and tool calls,
never for labels.

| Utility | Size / line | Use |
|---|---|---|
| `text-xl` | 22/28, semibold | Page heading (empty state, login) |
| `text-lg` | 17/24, semibold | Dialog and settings page titles |
| `text-base` | 15/24 | Body, messages, inputs in the composer |
| `text-sm` | 13/18 | UI controls, lists, descriptions |
| `text-xs` | 12/16 | Captions, hints, group labels |

Weights: `font-normal` (400), `font-medium` (500), `font-semibold` (600).
Headings use `tracking-tight`. Line height comes with the size.

### Spacing and layout

Tailwind's 4px scale (`p-2` = 8px…). Named sizes: `w-sidebar` (264px),
`max-w-column` (704px reading column), `max-w-dialog` / `max-w-dialog-wide`.
Content is left-aligned within the column.

## Components

| Need | Use |
|---|---|
| Any action | `Button` (primary, secondary, ghost, danger, title; `pressed` for toggles; `menu` for menu triggers) |
| Icon-only action | `IconButton` (label required; `tone="primary"` for send) |
| Text input | `TextField`, `TextArea`, `Select`, `SearchField` (inside menus and the palette) |
| On/off | `Switch` (with label and description, or `ariaLabel` inline) |
| One of a few | `Segmented` |
| File upload | `FilePicker` |
| Writing surface | `Well` + `AutoTextarea` |
| Attached file | `Attachment` |
| Popup list | `Menu` + `MenuGroup` + `MenuItem` |
| Modal | `Dialog` (title, description, `footer` snippet) |
| Sidebar/settings nav | `NavItem` |
| Rows of things | `List` + `ListRow` (leading `StatusDot`, trailing actions) |
| Page structure | `PageHeader`, `Section` |
| Expandable detail | `Disclosure` |
| Feedback | `Notice` (inline), `app.notify()` (toast), `Spinner`, `Progress`, `Badge`, `StatusDot`, `Kbd` |

## Writing

- Name things by what people do: "Add provider", "Turn on", "Import chats".
- A button says what happens, and the toast repeats it: "Save prompt" → "Prompt saved".
- Errors say what happened and what to do, without apologizing:
  "notes.bin can't be attached. Use images, PDF, DOCX or text files."
- Empty states invite action: "Your chats will show up here."

## Adding a feature

1. Build it from `src/lib/ui`. Layout utilities (`flex`, `gap-3`, `mt-6`…) and
   token utilities are fine in feature code.
2. If you need something the kit doesn't have, add it to `src/lib/ui` (using
   tokens), export it from `index.ts`, and show it on `/_ui`.
3. If you need a new value (a size, a color), add a token in `app.css` rather
   than an arbitrary value.
4. Run `npm run lint:ui`. For a justified exception, add
   `<!-- ui-allow: reason -->` on the line above.
5. Look at it in light, dark and at 390px wide.
