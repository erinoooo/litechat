<script lang="ts">
  import DOMPurify from 'dompurify'
  import { api, type SearchHit } from '../api'
  import { app } from '../state.svelte'
  import { Dialog, Kbd, MenuGroup, MenuItem, SearchField, Spinner } from '../ui'

  let q = $state('')
  let hits = $state<SearchHit[]>([])
  let related = $state<SearchHit[]>([])
  let relatedLoading = $state(false)
  let active = $state(0)
  let ctrl: AbortController | null = null
  let timer: ReturnType<typeof setTimeout>

  const semanticOn = $derived(!!(app.prefs.semantic_search && app.prefs.embedding_model))

  type Row = SearchHit & { section: 'recent' | 'exact' | 'related' }
  // no query: recent chats; otherwise exact matches, then meaning matches not already listed
  const rows = $derived.by<Row[]>(() => {
    if (!q.trim()) return app.chats.slice(0, 12).map((c) => ({ chat_id: c.id, title: c.title, snippet: '', section: 'recent' }))
    const seen = new Set(hits.map((h) => h.chat_id))
    return [
      ...hits.map((h) => ({ ...h, section: 'exact' as const })),
      ...related.filter((h) => !seen.has(h.chat_id)).map((h) => ({ ...h, section: 'related' as const })),
    ]
  })
  const sections = $derived.by(() => {
    const out: [Row['section'], { row: Row; i: number }[]][] = []
    rows.forEach((row, i) => {
      if (out.at(-1)?.[0] !== row.section) out.push([row.section, []])
      out.at(-1)![1].push({ row, i })
    })
    return out
  })

  function oninput() {
    active = 0
    clearTimeout(timer)
    ctrl?.abort()
    if (!q.trim()) {
      hits = []
      related = []
      relatedLoading = false
      return
    }
    timer = setTimeout(() => {
      ctrl = new AbortController()
      const signal = ctrl.signal
      const query = q
      api.search(query, signal).then((r) => (hits = r)).catch(() => {})
      if (semanticOn && query.trim().length > 2) {
        relatedLoading = true
        api
          .semanticSearch(query, signal)
          .then((r) => (related = r))
          .catch(() => (related = []))
          .finally(() => {
            if (!signal.aborted) relatedLoading = false
          })
      }
    }, semanticOn ? 220 : 120)
  }

  function go(id: string) {
    app.searchOpen = false
    app.openChat(id)
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'ArrowDown') { e.preventDefault(); active = Math.min(active + 1, rows.length - 1) }
    else if (e.key === 'ArrowUp') { e.preventDefault(); active = Math.max(active - 1, 0) }
    else if (e.key === 'Enter' && rows[active]) go(rows[active].chat_id)
    else return
    queueMicrotask(() => document.querySelector('[role=dialog] [data-active]')?.scrollIntoView({ block: 'nearest' }))
  }

  const labels = { recent: 'Recent', exact: 'Matching words', related: 'Related by meaning' }
</script>

<Dialog title="Search chats" bare onclose={() => (app.searchOpen = false)}>
  <div class="border-b border-line">
    <SearchField size="lg" bind:value={q} {oninput} {onkeydown} autofocus placeholder={semanticOn ? 'Search by words or by what it was about' : 'Search chats'} />
  </div>
  <div class="min-h-0 flex-1 overflow-y-auto p-1">
    {#each sections as [section, items] (section)}
      <MenuGroup label={labels[section]} icon={section === 'related' ? 'sparkle' : undefined}>
        {#each items as { row, i } (row.chat_id)}
          <MenuItem label={row.title} active={i === active} onclick={() => go(row.chat_id)} onmousemove={() => (active = i)}>
            {#snippet detailHtml()}{@html DOMPurify.sanitize(row.snippet, { ALLOWED_TAGS: ['mark'] })}{/snippet}
          </MenuItem>
        {/each}
      </MenuGroup>
    {:else}
      {#if !relatedLoading}
        <p class="px-3 py-8 text-sm text-ink-3">{q.trim() ? `Nothing matches “${q.trim()}”.` : 'Your chats will show up here.'}</p>
      {/if}
    {/each}
    {#if relatedLoading && q.trim()}
      <div class="flex h-9 items-center gap-2 px-2.5 text-xs text-ink-3"><Spinner size={12} /> Searching by meaning</div>
    {/if}
  </div>
  <div class="flex items-center gap-4 border-t border-line px-4 py-2 text-xs text-ink-3">
    <span class="flex items-center gap-1.5"><Kbd keys="↑↓" /> Move</span>
    <span class="flex items-center gap-1.5"><Kbd keys="Enter" /> Open</span>
    <span class="flex items-center gap-1.5"><Kbd keys="Esc" /> Close</span>
  </div>
</Dialog>
