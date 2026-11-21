import {
  api,
  attachStream,
  streamMessage,
  setUnauthorizedHandler,
  type Chat,
  type DiscoveredServer,
  type MCPServer,
  type MCPStatus,
  type Message,
  type Model,
  type Preset,
  type Provider,
  type StreamHandlers,
  type ToolInfo,
} from './api'

export type Prefs = {
  default_model?: string
  default_system_prompt?: string
  auto_title?: boolean
  send_on_enter?: boolean
  tools_enabled?: boolean
  semantic_search?: boolean
  embedding_model?: string
}

/** Everything the UI knows about one open conversation. */
export class ChatState {
  chat = $state<Chat>() as Chat
  messages = $state<Message[]>([])
  streamingId = $state<string | null>(null)
  activeTool = $state<ToolInfo | null>(null)
  error = $state<string | null>(null)
  controller: AbortController | null = null
  /** a stream reader is currently attached to the running reply */
  reading = false

  constructor(chat: Chat, messages: Message[]) {
    this.chat = chat
    this.messages = messages
  }

  byId(id: string) {
    return this.messages.find((m) => m.id === id)
  }

  children(parentId: string) {
    return this.messages.filter((m) => m.parent_id === parentId)
  }

  /** Root-to-leaf path along the active branch. */
  get path(): Message[] {
    const byId = new Map(this.messages.map((m) => [m.id, m]))
    const out: Message[] = []
    let id = this.chat.leaf_id
    if (!byId.has(id)) id = this.latestLeaf('')
    while (id) {
      const m = byId.get(id)
      if (!m) break
      out.push(m)
      id = m.parent_id
    }
    return out.reverse()
  }

  /** Follow the newest child down to a leaf. */
  latestLeaf(fromId: string): string {
    let id = fromId
    for (;;) {
      const kids = this.children(id)
      if (!kids.length) return id
      id = kids[kids.length - 1].id
    }
  }
}

function getTheme(): 'system' | 'light' | 'dark' {
  try {
    return (localStorage.getItem('theme') as 'light' | 'dark') || 'system'
  } catch {
    return 'system'
  }
}

class App {
  auth = $state<'loading' | 'setup' | 'login' | 'ready'>('loading')
  chats = $state<Chat[]>([])
  models = $state<Model[]>([])
  modelErrors = $state<Record<string, string>>({})
  modelsLoading = $state(false)
  providers = $state<Provider[]>([])
  presets = $state<Preset[]>([])
  prefs = $state<Prefs>({})
  theme = $state(getTheme())

  current = $state<ChatState | null>(null)
  /** model chosen for the next new chat (when no chat is open) */
  draftModel = $state('')
  draftSystem = $state('')

  sidebarOpen = $state(typeof window !== 'undefined' && window.innerWidth >= 900)
  settingsOpen = $state(false)
  settingsTab = $state<'providers' | 'tools' | 'presets' | 'data' | 'general'>('providers')
  mcpServers = $state<MCPServer[]>([])
  mcpStatus = $state<MCPStatus>({})
  stdioAllowed = $state(false)
  discovered = $state<DiscoveredServer[]>([])
  discovering = $state<false | 'local' | 'network'>(false)
  networkScanned = $state(false)
  searchOpen = $state(false)
  toast = $state<{ text: string; kind: 'info' | 'error' } | null>(null)

  private cache = new Map<string, ChatState>()
  private toastTimer: ReturnType<typeof setTimeout> | undefined

  constructor() {
    setUnauthorizedHandler(() => (this.auth = 'login'))
    window.addEventListener('popstate', () => this.syncRoute())
    // phones drop connections when backgrounded; pick the reply back up
    const resume = () => {
      const cs = this.current
      if (document.visibilityState === 'visible' && cs?.streamingId && !cs.reading) this.attach(cs)
    }
    document.addEventListener('visibilitychange', resume)
    window.addEventListener('online', resume)
  }

  // ---------- boot / auth ----------

  async boot() {
    try {
      const s = await api.status()
      this.auth = !s.setup ? 'setup' : s.authed ? 'ready' : 'login'
      if (this.auth === 'ready') await this.loadAll()
    } catch (e) {
      this.notify(`Can't reach the server: ${(e as Error).message}`, 'error')
    }
  }

  async loadAll() {
    const [chats, providers, presets, prefs, mcp] = await Promise.all([
      api.chats(),
      api.providers(),
      api.presets(),
      api.settings(),
      api.mcpServers(),
    ])
    this.chats = chats
    this.providers = providers
    this.presets = presets
    this.prefs = prefs as Prefs
    this.mcpServers = mcp.servers
    this.stdioAllowed = mcp.stdio_allowed
    this.draftSystem = this.prefs.default_system_prompt ?? ''
    await this.syncRoute()
    this.loadModels()
    if (!providers.length) {
      this.settingsTab = 'providers'
      this.settingsOpen = true
    }
  }

  async loadModels(refresh = false) {
    this.modelsLoading = true
    try {
      const r = await api.models(refresh)
      this.models = r.models
      this.modelErrors = r.errors
      if (!this.draftModel || !this.models.some((m) => m.key === this.draftModel)) {
        const def = this.prefs.default_model
        // embedding models can't chat; don't pick one by default
        const chatty = this.models.find((m) => !/embed|rerank|bge-|e5-|minilm/i.test(m.id))
        this.draftModel = def && this.models.some((m) => m.key === def) ? def : ((chatty ?? this.models[0])?.key ?? '')
      }
    } catch {
      /* surfaced in settings */
    } finally {
      this.modelsLoading = false
    }
  }

  async logout() {
    await api.logout().catch(() => {})
    this.cache.clear()
    this.current = null
    this.auth = 'login'
  }

  // ---------- routing ----------

  private routeChatId() {
    const m = location.pathname.match(/^\/c\/([\w-]+)/)
    return m ? m[1] : null
  }

  async syncRoute() {
    const id = this.routeChatId()
    if (!id) {
      this.current = null
      return
    }
    await this.openChat(id, false)
  }

  async openChat(id: string, push = true) {
    if (push && this.routeChatId() !== id) history.pushState(null, '', `/c/${id}`)
    if (window.innerWidth < 900) this.sidebarOpen = false
    const cached = this.cache.get(id)
    if (cached) {
      this.current = cached
      if (cached.streamingId) return
    }
    try {
      const { chat, messages, generating } = await api.chat(id)
      let cs = this.cache.get(id)
      if (cs) {
        if (!cs.streamingId) {
          cs.chat = chat
          cs.messages = messages
        }
      } else {
        cs = new ChatState(chat, messages)
        this.cache.set(id, cs)
      }
      if (generating && !cs.reading) {
        cs.streamingId = generating
        this.attach(cs)
      }
      if (this.routeChatId() === id) this.current = cs
    } catch {
      this.notify('Chat not found', 'error')
      this.newChat()
    }
  }

  newChat() {
    if (location.pathname !== '/') history.pushState(null, '', '/')
    this.current = null
    this.draftSystem = this.prefs.default_system_prompt ?? ''
    if (window.innerWidth < 900) this.sidebarOpen = false
  }

  // ---------- chat actions ----------

  get activeModel() {
    return this.current?.chat.model || this.draftModel
  }

  setModel(key: string) {
    if (this.current) {
      this.current.chat.model = key
      api.updateChat(this.current.chat.id, { model: key }).catch(() => {})
    } else {
      this.draftModel = key
    }
  }

  async send(content: string, images: string[] = []) {
    const model = this.activeModel
    if (!model) {
      this.notify('Add a provider and pick a model first', 'error')
      this.settingsOpen = true
      return
    }
    let cs = this.current
    if (!cs) {
      const chat = await api.createChat({ model, system_prompt: this.draftSystem })
      cs = new ChatState(chat, [])
      this.cache.set(chat.id, cs)
      this.chats = [chat, ...this.chats]
      history.pushState(null, '', `/c/${chat.id}`)
      this.current = cs
    }
    const path = cs.path
    const parent = path.length ? path[path.length - 1].id : ''
    await this.run(cs, { parent_id: parent, content, images, model })
  }

  get toolsAvailable() {
    return this.mcpServers.some((s) => s.enabled)
  }

  get toolsOn() {
    return this.toolsAvailable && (this.prefs.tools_enabled ?? true)
  }

  /** Send an edited version of a user message as a new branch. */
  async edit(msg: Message, content: string) {
    const cs = this.current
    if (!cs) return
    await this.run(cs, { parent_id: msg.parent_id, content, images: msg.images, model: this.activeModel })
  }

  async regenerate(assistant: Message, model?: string) {
    const cs = this.current
    if (!cs) return
    await this.run(cs, { parent_id: assistant.parent_id, model: model || this.activeModel, regenerate: true })
  }

  switchBranch(msg: Message, dir: -1 | 1) {
    const cs = this.current
    if (!cs || cs.streamingId) return
    const sibs = cs.children(msg.parent_id)
    const i = sibs.findIndex((m) => m.id === msg.id)
    const next = sibs[i + dir]
    if (!next) return
    cs.chat.leaf_id = cs.latestLeaf(next.id)
    api.updateChat(cs.chat.id, { leaf_id: cs.chat.leaf_id }).catch(() => {})
  }

  /** Stop the reply on the server; the stream then ends with `done`. */
  async stop() {
    const cs = this.current
    if (!cs?.streamingId) return
    try {
      await api.stop(cs.chat.id)
    } catch {
      cs.controller?.abort()
    }
  }

  /** Handlers that apply stream events to `asst`, batching deltas per frame. */
  private handlers(cs: ChatState, getAsst: () => Message | undefined): StreamHandlers & { flush: () => void } {
    let pending = ''
    let raf = 0
    const flush = () => {
      cancelAnimationFrame(raf)
      raf = 0
      const a = getAsst()
      if (pending && a) a.content += pending
      pending = ''
    }
    return {
      flush,
      sync: (d) => {
        const a = getAsst()
        if (a) a.content = d.content
        pending = ''
        cs.activeTool = d.tool
      },
      delta: (t) => {
        pending += t
        if (t.includes('<tool ')) cs.activeTool = null
        if (!raf) raf = requestAnimationFrame(flush)
      },
      tool_start: (t) => {
        flush()
        cs.activeTool = t
      },
      done: (d) => {
        flush()
        const a = getAsst()
        if (a) {
          a.content = d.content
          if (d.error) a.error = d.error
        }
        cs.streamingId = null
        cs.activeTool = null
        this.bumpChat(cs.chat.id)
      },
      title: (title) => {
        cs.chat.title = title
        const c = this.chats.find((c) => c.id === cs.chat.id)
        if (c) c.title = title
      },
    }
  }

  /** Follow a reply that's already running on the server (after reload, tab switch, dropped connection). */
  async attach(cs: ChatState, tries = 0) {
    if (cs.reading || !cs.streamingId) return
    cs.reading = true
    const ctrl = new AbortController()
    cs.controller = ctrl
    const id = cs.streamingId
    const h = this.handlers(cs, () => cs.byId(id))
    let finished: boolean | null = false
    try {
      finished = await attachStream(cs.chat.id, h, ctrl.signal)
    } catch {
      finished = false
    } finally {
      h.flush()
      cs.reading = false
      cs.controller = null
    }
    if (finished === null || (!finished && tries >= 3)) {
      // it ended while we weren't looking: load the saved result
      cs.streamingId = null
      cs.activeTool = null
      const { chat, messages } = await api.chat(cs.chat.id).catch(() => ({ chat: cs.chat, messages: cs.messages }))
      cs.chat = chat
      cs.messages = messages
    } else if (!finished && !ctrl.signal.aborted) {
      setTimeout(() => this.attach(cs, tries + 1), 500 * (tries + 1))
    }
  }

  private async run(
    cs: ChatState,
    body: { parent_id: string; content?: string; images?: string[]; model: string; regenerate?: boolean; tools?: boolean },
  ) {
    if (cs.streamingId) return
    body.tools = this.toolsOn
    cs.error = null
    const now = Date.now()
    const tmpUser: Message | null = body.regenerate
      ? null
      : {
          id: 'tmp-u' + now,
          chat_id: cs.chat.id,
          parent_id: body.parent_id,
          role: 'user',
          content: body.content ?? '',
          images: body.images ?? [],
          model: '',
          created_at: now,
        }
    const tmpAsst: Message = {
      id: 'tmp-a' + now,
      chat_id: cs.chat.id,
      parent_id: tmpUser ? tmpUser.id : body.parent_id,
      role: 'assistant',
      content: '',
      images: [],
      model: body.model,
      created_at: now + 1,
    }
    if (tmpUser) cs.messages.push(tmpUser)
    cs.messages.push(tmpAsst)
    cs.chat.leaf_id = tmpAsst.id
    cs.streamingId = tmpAsst.id
    const ctrl = new AbortController()
    cs.controller = ctrl
    cs.reading = true
    const asst = cs.messages[cs.messages.length - 1] // the reactive proxy
    const h = this.handlers(cs, () => asst)

    let finished = false
    try {
      finished = await streamMessage(
        cs.chat.id,
        body,
        {
          ...h,
          meta: ({ user, assistant }) => {
            if (tmpUser) {
              const u = cs.messages.find((m) => m.id === tmpUser.id)
              if (u) Object.assign(u, user)
            }
            Object.assign(asst, { ...assistant, content: asst.content })
            cs.chat.leaf_id = assistant.id
            cs.streamingId = assistant.id
          },
        },
        ctrl.signal,
      )
    } catch (e) {
      h.flush()
      if ((e as Error).name !== 'AbortError' && asst.id.startsWith('tmp-')) {
        // rejected before it started: drop the optimistic messages
        cs.messages = cs.messages.filter((m) => m.id !== asst.id && m.id !== tmpUser?.id)
        cs.chat.leaf_id = cs.latestLeaf(body.parent_id)
        cs.streamingId = null
        this.notify((e as Error).message, 'error')
      }
    } finally {
      h.flush()
      cs.reading = false
      cs.controller = null
    }
    // connection dropped mid-reply: the server is still generating
    if (!finished && cs.streamingId && !cs.streamingId.startsWith('tmp-')) this.attach(cs)
  }

  private bumpChat(id: string) {
    const i = this.chats.findIndex((c) => c.id === id)
    if (i < 0) return
    const [c] = this.chats.splice(i, 1)
    c.updated_at = Date.now()
    const firstUnpinned = this.chats.findIndex((x) => !x.pinned)
    this.chats.splice(c.pinned ? 0 : firstUnpinned < 0 ? this.chats.length : firstUnpinned, 0, c)
  }

  async renameChat(id: string, title: string) {
    title = title.trim()
    if (!title) return
    const c = this.chats.find((c) => c.id === id)
    if (c) c.title = title
    const cs = this.cache.get(id)
    if (cs) cs.chat.title = title
    await api.updateChat(id, { title })
  }

  async togglePin(id: string) {
    const c = this.chats.find((c) => c.id === id)
    if (!c) return
    c.pinned = !c.pinned
    await api.updateChat(id, { pinned: c.pinned })
    this.chats = await api.chats()
  }

  async deleteChat(id: string) {
    this.cache.get(id)?.controller?.abort()
    this.cache.delete(id)
    this.chats = this.chats.filter((c) => c.id !== id)
    if (this.current?.chat.id === id) this.newChat()
    await api.deleteChat(id)
  }

  async setSystemPrompt(prompt: string) {
    if (this.current) {
      this.current.chat.system_prompt = prompt
      await api.updateChat(this.current.chat.id, { system_prompt: prompt })
    } else {
      this.draftSystem = prompt
    }
  }

  // ---------- settings ----------

  async savePrefs(p: Partial<Prefs>) {
    this.prefs = { ...this.prefs, ...p }
    await api.saveSettings(this.prefs)
  }

  setTheme(t: 'system' | 'light' | 'dark') {
    this.theme = t
    try {
      if (t === 'system') localStorage.removeItem('theme')
      else localStorage.setItem('theme', t)
    } catch {}
    if (t === 'system') delete document.documentElement.dataset.theme
    else document.documentElement.dataset.theme = t
  }

  notify(text: string, kind: 'info' | 'error' = 'info') {
    this.toast = { text, kind }
    clearTimeout(this.toastTimer)
    this.toastTimer = setTimeout(() => (this.toast = null), kind === 'error' ? 6000 : 2500)
  }

  modelName(key: string) {
    const m = this.models.find((m) => m.key === key)
    if (m) return m.name
    const i = key.indexOf('/')
    // keys are "<provider>/<model>"; imported chats may carry a bare model name
    return i > 0 && /^[0-9a-f]{20,}$/.test(key.slice(0, i)) ? key.slice(i + 1) : key
  }

  /** Look for AI servers on this machine, or also on the local network. */
  async discover(network = false) {
    if (this.discovering) return
    this.discovering = network ? 'network' : 'local'
    try {
      const r = await api.discover(network)
      // keep LAN results from an earlier network scan when refreshing local ones
      const lan = network ? [] : this.discovered.filter((s) => !s.local)
      this.discovered = [...r.servers, ...lan.filter((l) => !r.servers.some((s) => s.base_url === l.base_url))]
      if (network) this.networkScanned = true
    } catch (e) {
      this.notify((e as Error).message, 'error')
    } finally {
      this.discovering = false
    }
  }

  /** Add a discovered server as a provider, with a name that's unique and says where it is. */
  async addDiscovered(s: DiscoveredServer) {
    const taken = new Set(this.providers.map((p) => p.name))
    let name = s.local ? s.kind : `${s.kind} on ${s.host}`
    if (taken.has(name)) name = `${name} (port ${s.port})`
    try {
      const p = await api.saveProvider({ name, kind: 'openai', base_url: s.base_url, api_key: '' })
      this.providers = await api.providers()
      s.provider_id = p.id
      await this.loadModels(true)
      this.notify(`${name} added`)
    } catch (e) {
      this.notify((e as Error).message, 'error')
    }
  }

  async loadMCP(refresh = false) {
    const r = await api.mcpServers()
    this.mcpServers = r.servers
    this.stdioAllowed = r.stdio_allowed
    this.mcpStatus = await api.mcpStatus(refresh)
  }

  async exportChatMarkdown(id: string) {
    const { chat, messages } = await api.chat(id)
    const cs = new ChatState(chat, messages)
    const lines = [`# ${chat.title}`, '']
    if (chat.system_prompt) lines.push('> **System:** ' + chat.system_prompt.replace(/\n/g, '\n> '), '')
    for (const m of cs.path) {
      lines.push(m.role === 'user' ? '## You' : `## Assistant${m.model ? ` (${this.modelName(m.model)})` : ''}`, '')
      lines.push(m.content.replace(/<think>[\s\S]*?(<\/think>|$)/g, '').trim(), '')
    }
    const blob = new Blob([lines.join('\n')], { type: 'text/markdown' })
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = (chat.title.replace(/[^\w\- ]+/g, '').trim() || 'chat') + '.md'
    a.click()
    URL.revokeObjectURL(a.href)
  }
}

export const app = new App()
