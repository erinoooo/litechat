// Markdown rendering tuned for streaming:
//  - content is split into top-level blocks; finished blocks are cached, so
//    while a reply streams only the last block is re-parsed;
//  - highlight.js and KaTeX are loaded only when a message needs them.
import { Marked, type Token, type Tokens } from 'marked'
import DOMPurify from 'dompurify'

type HLJS = typeof import('highlight.js').default

let hljs: HLJS | null = null
let hljsLoading: Promise<void> | null = null
let katexReady = false
let katexLoading: Promise<void> | null = null

/** Bumped when a lazy library finishes loading so views can re-render. */
let version = 0
const listeners = new Set<() => void>()
export function onRendererChange(fn: () => void) {
  listeners.add(fn)
  return () => listeners.delete(fn)
}
function bump() {
  version++
  cache.clear()
  listeners.forEach((fn) => fn())
}

function escapeHtml(s: string) {
  return s.replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]!)
}

function loadHljs() {
  hljsLoading ??= import('highlight.js/lib/common').then((m) => {
    hljs = m.default
    bump()
  })
}

function loadKatex() {
  katexLoading ??= Promise.all([
    import('marked-katex-extension'),
    import('katex/dist/katex.min.css'),
  ]).then(([ext]) => {
    md.use(ext.default({ throwOnError: false, nonStandard: true }))
    katexReady = true
    bump()
  })
}

const md = new Marked({ gfm: true, breaks: false })
md.use({
  renderer: {
    code({ text, lang }: Tokens.Code) {
      const language = (lang || '').trim().split(/\s+/)[0].toLowerCase()
      let body: string
      if (hljs && language && hljs.getLanguage(language)) {
        body = hljs.highlight(text, { language, ignoreIllegals: true }).value
      } else if (hljs && !language && text.length < 20000) {
        body = hljs.highlightAuto(text).value
      } else {
        if (!hljs) loadHljs()
        body = escapeHtml(text)
      }
      return `<div class="code-block"><div class="code-head"><span>${escapeHtml(language || 'text')}</span><button class="copy-btn" type="button" data-copy>Copy</button></div><pre><code>${body}</code></pre></div>`
    },
  },
})

DOMPurify.addHook('afterSanitizeAttributes', (node) => {
  if (node.tagName === 'A') {
    node.setAttribute('target', '_blank')
    node.setAttribute('rel', 'noopener noreferrer')
  }
})

const MATH_HINT = /\$\$|\\\[|\\\(|\$[^\s$][^$\n]*?[^\s$]\$/

/** LLMs often write \(..\) and \[..\]; convert to $ / $$ outside of code. */
function normalizeMath(src: string) {
  if (!src.includes('\\(') && !src.includes('\\[')) return src
  return src
    .split(/(```[\s\S]*?(?:```|$)|`[^`\n]*`)/)
    .map((part, i) =>
      i % 2
        ? part
        : part
            .replace(/\\\[([\s\S]+?)\\\]/g, (_, m) => `$$${m}$$`)
            .replace(/\\\(([\s\S]+?)\\\)/g, (_, m) => `$${m.trim()}$`),
    )
    .join('')
}

const cache = new Map<string, string>()
const CACHE_MAX = 2000

function renderToken(tok: Token): string {
  const key = tok.raw
  const hit = cache.get(key)
  if (hit !== undefined) return hit
  const html = DOMPurify.sanitize(md.parser([tok]))
  if (cache.size > CACHE_MAX) cache.clear()
  cache.set(key, html)
  return html
}

export type Block = { key: string; html: string }

/** Render markdown into a list of independently-updatable blocks. */
export function renderBlocks(src: string): Block[] {
  if (!katexReady && MATH_HINT.test(src)) loadKatex()
  const tokens = md.lexer(katexReady ? normalizeMath(src) : src)
  const out: Block[] = []
  let i = 0
  for (const tok of tokens) {
    if (tok.type === 'space') continue
    out.push({ key: `${i++}`, html: renderToken(tok) })
  }
  return out
}

export type ToolBlock = { name: string; server: string; status: string; args: unknown; result: string }
export type Segment = { kind: 'think' | 'text' | 'tool'; text: string; open: boolean; tool?: ToolBlock }

const unescape = (s: string) =>
  s.replace(/&(amp|lt|gt|quot|#39|#34);/g, (_, e) => ({ amp: '&', lt: '<', gt: '>', quot: '"', '#39': "'", '#34': '"' })[e as string]!)

const OPEN = /<think>|<tool name="([^"]*)" server="([^"]*)" status="([^"]*)">\n/g

/** Split a reply into <think> sections, <tool> calls and normal text. */
export function segments(src: string): Segment[] {
  const out: Segment[] = []
  const text = (t: string) => {
    if (t.trim()) out.push({ kind: 'text', text: t, open: false })
  }
  let pos = 0
  OPEN.lastIndex = 0
  for (let m: RegExpExecArray | null; (m = OPEN.exec(src)); ) {
    text(src.slice(pos, m.index))
    const bodyStart = m.index + m[0].length
    if (m[0] === '<think>') {
      const end = src.indexOf('</think>', bodyStart)
      if (end < 0) {
        out.push({ kind: 'think', text: src.slice(bodyStart), open: true })
        return out
      }
      out.push({ kind: 'think', text: src.slice(bodyStart, end), open: false })
      pos = end + 8
    } else {
      const end = src.indexOf('\n</tool>', bodyStart)
      if (end < 0) return out // incomplete block (shouldn't happen: blocks arrive whole)
      let args: unknown = {}
      let result = ''
      try {
        const p = JSON.parse(src.slice(bodyStart, end))
        args = p.args
        result = p.result ?? ''
      } catch {}
      out.push({
        kind: 'tool',
        text: '',
        open: false,
        tool: { name: unescape(m[1]), server: unescape(m[2]), status: m[3], args, result },
      })
      pos = end + 8
    }
    OPEN.lastIndex = pos
  }
  text(src.slice(pos))
  return out
}

/** Plain text of a reply, for copying: no thinking, tool calls summarized. */
export function plainText(src: string) {
  return segments(src)
    .map((s) => (s.kind === 'text' ? s.text.trim() : s.kind === 'tool' ? `[used ${s.tool!.name}]` : ''))
    .filter(Boolean)
    .join('\n\n')
}

export const rendererVersion = () => version
