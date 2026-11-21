export type Provider = {
  id: string
  name: string
  kind: 'openai' | 'anthropic'
  base_url: string
  has_key: boolean
  enabled: boolean
  created_at: number
}

export type Model = { key: string; id: string; name: string; provider_id: string; provider_name: string }

export type Chat = {
  id: string
  title: string
  system_prompt: string
  model: string
  leaf_id: string
  pinned: boolean
  created_at: number
  updated_at: number
}

export type Message = {
  id: string
  chat_id: string
  parent_id: string
  role: 'user' | 'assistant'
  content: string
  images: string[]
  model: string
  error?: string
  created_at: number
}

export type Preset = { id: string; name: string; prompt: string; created_at: number }
export type SearchHit = { chat_id: string; title: string; snippet: string }

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message)
  }
}

let onUnauthorized: () => void = () => {}
export function setUnauthorizedHandler(fn: () => void) {
  onUnauthorized = fn
}

async function req<T>(method: string, path: string, body?: unknown, signal?: AbortSignal): Promise<T> {
  const res = await fetch('/api' + path, {
    method,
    headers: { 'X-LiteChat': '1', ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}) },
    body: body !== undefined ? JSON.stringify(body) : undefined,
    signal,
  })
  if (!res.ok) {
    let msg = res.statusText
    try {
      msg = (await res.json()).error ?? msg
    } catch {}
    if (res.status === 401 && !path.startsWith('/auth/')) onUnauthorized()
    throw new ApiError(res.status, msg)
  }
  return res.json()
}

export const api = {
  status: () => req<{ setup: boolean; authed: boolean }>('GET', '/auth/status'),
  setup: (password: string) => req('POST', '/auth/setup', { password }),
  login: (password: string) => req('POST', '/auth/login', { password }),
  logout: () => req('POST', '/auth/logout'),
  changePassword: (current: string, next: string) => req('POST', '/auth/password', { current, new: next }),

  providers: () => req<Provider[]>('GET', '/providers'),
  saveProvider: (p: Partial<Provider> & { api_key?: string }, id?: string) =>
    req<Provider>(id ? 'PUT' : 'POST', id ? `/providers/${id}` : '/providers', p),
  deleteProvider: (id: string) => req('DELETE', `/providers/${id}`),
  models: (refresh = false) =>
    req<{ models: Model[]; errors: Record<string, string> }>('GET', '/models' + (refresh ? '?refresh=1' : '')),

  chats: () => req<Chat[]>('GET', '/chats'),
  createChat: (c: Partial<Chat>) => req<Chat>('POST', '/chats', c),
  chat: (id: string) => req<{ chat: Chat; messages: Message[] }>('GET', `/chats/${id}`),
  updateChat: (id: string, fields: Partial<Chat>) => req<Chat>('PATCH', `/chats/${id}`, fields),
  deleteChat: (id: string) => req('DELETE', `/chats/${id}`),

  presets: () => req<Preset[]>('GET', '/presets'),
  savePreset: (p: Partial<Preset>, id?: string) => req<Preset>(id ? 'PUT' : 'POST', id ? `/presets/${id}` : '/presets', p),
  deletePreset: (id: string) => req('DELETE', `/presets/${id}`),

  settings: () => req<Record<string, unknown>>('GET', '/settings'),
  saveSettings: (s: Record<string, unknown>) => req('PUT', '/settings', s),

  search: (q: string, signal?: AbortSignal) => req<SearchHit[]>('GET', '/search?q=' + encodeURIComponent(q), undefined, signal),
}

export type StreamHandlers = {
  meta: (d: { user: Message; assistant: Message }) => void
  delta: (text: string) => void
  done: (d: { content: string; error?: string }) => void
  title: (title: string) => void
}

/** POST a message and consume the server-sent event stream. */
export async function streamMessage(
  chatId: string,
  body: { parent_id: string; content?: string; images?: string[]; model: string; regenerate?: boolean },
  h: StreamHandlers,
  signal: AbortSignal,
) {
  const res = await fetch(`/api/chats/${chatId}/messages`, {
    method: 'POST',
    headers: { 'X-LiteChat': '1', 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
    signal,
  })
  if (!res.ok || !res.body) {
    let msg = res.statusText
    try {
      msg = (await res.json()).error ?? msg
    } catch {}
    throw new ApiError(res.status, msg)
  }
  const reader = res.body.pipeThrough(new TextDecoderStream()).getReader()
  let buf = ''
  for (;;) {
    const { value, done } = await reader.read()
    if (done) break
    buf += value
    let idx: number
    while ((idx = buf.indexOf('\n\n')) >= 0) {
      const block = buf.slice(0, idx)
      buf = buf.slice(idx + 2)
      let event = 'message'
      let data = ''
      for (const line of block.split('\n')) {
        if (line.startsWith('event:')) event = line.slice(6).trim()
        else if (line.startsWith('data:')) data += line.slice(5).trim()
      }
      if (!data) continue
      const d = JSON.parse(data)
      if (event === 'meta') h.meta(d)
      else if (event === 'delta') h.delta(d.t)
      else if (event === 'done') h.done(d)
      else if (event === 'title') h.title(d.title)
    }
  }
}
