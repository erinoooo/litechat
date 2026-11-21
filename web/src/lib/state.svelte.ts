import { api, streamMessage, setUnauthorizedHandler, type Chat, type Message, type Model, type Preset, type Provider } from './api'

export type Prefs = {
  default_model?: string
  default_system_prompt?: string
  auto_title?: boolean
  send_on_enter?: boolean
}

/** Everything the UI knows about one open conversation. */
export class ChatState {
  chat = $state<Chat>() as Chat
  messages = $state<Message[]>([])
  streamingId = $state<string | null>(null)
  error = $state<string | null>(null)
  controller: AbortController | null = null

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
  settingsTab = $state<'providers' | 'presets' | 'general'>('providers')
  searchOpen = $state(false)
  toast = $state<{ text: string; kind: 'info' | 'error' } | null>(null)

  private cache = new Map<string, ChatState>()
  private toastTimer: ReturnType<typeof setTimeout> | undefined

  constructor() {
    setUnauthorizedHandler(() => (this.auth = 'login'))
    window.addEventListener('popstate', () => this.syncRoute())
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
    const [chats, providers, presets, prefs] = await Promise.all([
      api.chats(),
      api.providers(),
      api.presets(),
      api.settings(),
    ])
    this.chats = chats
    this.providers = providers
    this.presets = presets
    this.prefs = prefs as Prefs
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
        this.draftModel = def && this.models.some((m) => m.key === def) ? def : (this.models[0]?.key ?? '')
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
      const { chat, messages } = await api.chat(id)
      const cs = this.cache.get(id)
      if (cs) {
        if (!cs.streamingId) {
          cs.chat = chat
          cs.messages = messages
        }
      } else {
        this.cache.set(id, new ChatState(chat, messages))
      }
      if (this.routeChatId() === id) this.current = this.cache.get(id)!
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

  stop() {
    this.current?.controller?.abort()
  }

  private async run(
    cs: ChatState,
    body: { parent_id: string; content?: string; images?: string[]; model: string; regenerate?: boolean },
  ) {
    if (cs.streamingId) return
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

    // batch deltas into one update per frame
    let pending = ''
    let raf = 0
    let asst = cs.messages[cs.messages.length - 1] // the reactive proxy
    const flush = () => {
      raf = 0
      if (pending) {
        asst.content += pending
        pending = ''
      }
    }

    try {
      await streamMessage(
        cs.chat.id,
        body,
        {
          meta: ({ user, assistant }) => {
            if (tmpUser) {
              const u = cs.messages.find((m) => m.id === tmpUser.id)
              if (u) Object.assign(u, user)
            }
            Object.assign(asst, { ...assistant, content: asst.content })
            cs.chat.leaf_id = assistant.id
            cs.streamingId = assistant.id
          },
          delta: (t) => {
            pending += t
            if (!raf) raf = requestAnimationFrame(flush)
          },
          done: (d) => {
            cancelAnimationFrame(raf)
            asst.content = d.content
            if (d.error) asst.error = d.error
            cs.streamingId = null
            this.bumpChat(cs.chat.id)
          },
          title: (title) => {
            cs.chat.title = title
            const c = this.chats.find((c) => c.id === cs.chat.id)
            if (c) c.title = title
          },
        },
        ctrl.signal,
      )
    } catch (e) {
      cancelAnimationFrame(raf)
      flush()
      if ((e as Error).name !== 'AbortError') {
        // request rejected before streaming: drop the optimistic messages
        if (asst.id.startsWith('tmp-')) {
          cs.messages = cs.messages.filter((m) => m.id !== asst.id && m.id !== tmpUser?.id)
          cs.chat.leaf_id = cs.latestLeaf(body.parent_id)
          this.notify((e as Error).message, 'error')
        } else {
          asst.error = (e as Error).message
        }
      }
    } finally {
      cs.streamingId = null
      cs.controller = null
    }
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
    return this.models.find((m) => m.key === key)?.name ?? key.split('/').slice(1).join('/')
  }
}

export const app = new App()
