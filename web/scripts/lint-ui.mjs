#!/usr/bin/env node
// Design-system guard. Feature code (everything outside src/lib/ui and app.css)
// must build from the UI kit and tokens. See web/DESIGN.md.
//
// To allow a deliberate exception, put a comment on the line above:
//   <!-- ui-allow: reason -->   or   // ui-allow: reason
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'

const root = new URL('..', import.meta.url).pathname
const src = join(root, 'src')
const KIT = join(src, 'lib', 'ui')

const rules = [
  {
    id: 'raw-control',
    re: /<(button|input|select|textarea)\b/,
    msg: 'Use the UI kit (Button, IconButton, TextField, Select, TextArea, Switch, FilePicker…) instead of a raw <$1>.',
  },
  { id: 'inline-style-block', re: /<style\b/, msg: 'Feature components get no <style> blocks; put shared styles in app.css or a kit component.' },
  { id: 'arbitrary-value', re: /(?<![\w$])[a-z:!-]*-\[[^\]\s]+\]/, msg: 'Arbitrary Tailwind value. Use a token, or add a named token/utility in app.css.' },
  { id: 'arbitrary-property', re: /class(?:=|:)[^>]*\[[a-z-]+:[^\]]+\]/, msg: 'Arbitrary CSS property in a class. Add a named @utility in app.css.' },
  { id: 'hex-color', re: /(?<![\w&])#[0-9a-fA-F]{3}(?:[0-9a-fA-F]{3}(?:[0-9a-fA-F]{2})?)?\b/, msg: 'Raw color. Use a color token (bg-panel, text-ink-2, …).' },
  {
    id: 'palette-color',
    re: /\b(?:text|bg|border|ring|fill|stroke|divide|outline|decoration)-(?:white|black|(?:slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-\d{2,3})\b/,
    msg: 'Tailwind palette color. Only design tokens exist (canvas, panel, ink, pine, red…).',
  },
  { id: 'all-caps', re: /\b(?:uppercase|tracking-wide|tracking-wider|tracking-widest)\b/, msg: 'No all-caps or tracked-out labels. Use sentence case (MenuGroup, Section).' },
  { id: 'type-scale', re: /\btext-(?:[2-9]xl|xxs)\b|\bfont-(?:thin|light|bold|extrabold|black)\b|\bleading-\w+/, msg: 'Off-scale type. Use text-xs/sm/base/lg/xl and font-normal/medium/semibold.' },
  { id: 'radius-scale', re: /\brounded(?:-[trblxyse]{1,2})?-(?:xs|xl|2xl|3xl|4xl|none)\b/, msg: 'Off-scale radius. Use rounded-sm (inline), rounded-md (controls), rounded-lg (overlays).' },
  { id: 'shadow-scale', re: /\bshadow-(?!overlay\b)[a-z0-9]+\b/, msg: 'Only floating layers have a shadow: shadow-overlay.' },
  { id: 'animation', re: /\banimate-\w+/, msg: 'Use the motion-* utilities from app.css (motion-overlay, motion-spin…).' },
]

function walk(dir, out = []) {
  for (const name of readdirSync(dir)) {
    const p = join(dir, name)
    if (statSync(p).isDirectory()) {
      if (p !== KIT) walk(p, out)
    } else if (/\.(svelte|ts)$/.test(name)) out.push(p)
  }
  return out
}

let problems = 0
for (const file of walk(src)) {
  const lines = readFileSync(file, 'utf8').split('\n')
  lines.forEach((line, i) => {
    if (/ui-allow:/.test(lines[i - 1] ?? '') || /ui-allow:/.test(line)) return
    if (/^\s*(\/\/|\*|\/\*)/.test(line)) return // comments
    for (const r of rules) {
      const m = line.match(r.re)
      if (!m) continue
      if (r.id === 'raw-control' && /type="file"/.test(line + (lines[i + 1] ?? '') + (lines[i + 2] ?? ''))) continue
      problems++
      console.log(`${relative(root, file)}:${i + 1}  ${r.id}  ${r.msg.replace('$1', m[1] ?? '')}\n    ${line.trim().slice(0, 140)}`)
    }
  })
}

if (problems) {
  console.log(`\n${problems} design-system problem${problems === 1 ? '' : 's'}. See web/DESIGN.md.`)
  process.exit(1)
}
console.log('lint-ui: feature code uses the design system only.')
