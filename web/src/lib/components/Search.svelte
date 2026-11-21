<script lang="ts">
  import DOMPurify from 'dompurify'
  import { api, type SearchHit } from '../api'
  import { app } from '../state.svelte'
  import Icon from './Icon.svelte'

  let q = $state('')
  let hits = $state<SearchHit[]>([])
  let related = $state<SearchHit[]>([])
  let relatedLoading = $state(false)
  let active = $state(0)
  let ctrl: AbortController | null = null
  let timer: ReturnType<typeof setTimeout>

  const semanticOn = $derived(!!(app.prefs.semantic_search && app.prefs.embedding_model))

  type Row = SearchHit & { section: 'recent' | 'exact' | 'related' }
  // with no query, show recent chats; otherwise exact matches, then meaning matches not already listed
  const rows = $derived.by<Row[]>(() => {
    if (!q.trim()) return app.chats.slice(0, 12).map((c) => ({ chat_id: c.id, title: c.title, snippet: '', section: 'recent' }))
    const seen = new Set(hits.map((h) => h.chat_id))
    return [
      ...hits.map((h) => ({ ...h, section: 'exact' as const })),
      ...related.filter((h) => !seen.has(h.chat_id)).map((h) => ({ ...h, section: 'related' as const })),
    ]
  })

  function oninput() {
    active = 0
    clearTimeout(timer)
    ctrl?.abort()
    if (!q.trim()) {
      hits = []
      related = []
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
    else if (e.key === 'Escape') app.searchOpen = false
    else return
    queueMicrotask(() => document.querySelector('[data-search-active="true"]')?.scrollIntoView({ block: 'nearest' }))
  }

  const labels = { recent: 'Recent', exact: 'Matching words', related: 'Related by meaning' }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div class="fixed inset-0 z-50 flex justify-center bg-black/40 p-4 pt-[12vh] backdrop-blur-[2px]" onclick={(e) => e.target === e.currentTarget && (app.searchOpen = false)}>
  <div class="animate-pop flex h-fit max-h-[70vh] w-full max-w-xl flex-col overflow-hidden rounded-2xl border border-line bg-elev shadow-pop">
    <div class="flex items-center gap-3 border-b border-line px-4">
      <Icon name="search" class="text-faint" />
      <!-- svelte-ignore a11y_autofocus -->
      <input bind:value={q} {oninput} {onkeydown} autofocus placeholder={semanticOn ? 'Search by words or by what it was about…' : 'Search conversations…'} class="h-14 w-full bg-transparent outline-none placeholder:text-faint" />
      <kbd class="rounded border border-line px-1.5 text-[11px] text-faint">Esc</kbd>
    </div>
    <div class="overflow-y-auto p-1.5">
      {#each rows as h, i (h.section + h.chat_id)}
        {#if i === 0 || rows[i - 1].section !== h.section}
          <div class="flex items-center gap-1.5 px-2.5 pb-1 pt-2 text-[11px] font-semibold uppercase tracking-wider text-faint">
            {#if h.section === 'related'}<Icon name="sparkle" size={12} />{/if}{labels[h.section]}
          </div>
        {/if}
        <button class="menu-item !items-start flex-col !gap-0.5" data-active={i === active} data-search-active={i === active} onclick={() => go(h.chat_id)} onmousemove={() => (active = i)}>
          <span class="w-full truncate font-medium">{h.title}</span>
          {#if h.snippet}<span class="line-clamp-2 w-full text-xs text-muted">{@html DOMPurify.sanitize(h.snippet, { ALLOWED_TAGS: ['mark'] })}</span>{/if}
        </button>
      {:else}
        {#if !relatedLoading}
          <div class="px-3 py-8 text-center text-sm text-muted">{q.trim() ? 'No results' : 'No conversations yet'}</div>
        {/if}
      {/each}
      {#if relatedLoading && q.trim()}
        <div class="flex items-center gap-2 px-3 py-2.5 text-xs text-faint">
          <span class="h-3 w-3 animate-spin rounded-full border-2 border-current border-t-transparent"></span> Searching by meaning…
        </div>
      {/if}
    </div>
  </div>
</div>
