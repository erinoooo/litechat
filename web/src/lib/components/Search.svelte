<script lang="ts">
  import DOMPurify from 'dompurify'
  import { api, type SearchHit } from '../api'
  import { app } from '../state.svelte'
  import Icon from './Icon.svelte'

  let q = $state('')
  let hits = $state<SearchHit[]>([])
  let active = $state(0)
  let ctrl: AbortController | null = null
  let timer: ReturnType<typeof setTimeout>

  // with no query, show recent chats
  const results = $derived<SearchHit[]>(
    q.trim() ? hits : app.chats.slice(0, 12).map((c) => ({ chat_id: c.id, title: c.title, snippet: '' })),
  )

  function oninput() {
    active = 0
    clearTimeout(timer)
    timer = setTimeout(async () => {
      ctrl?.abort()
      if (!q.trim()) return
      ctrl = new AbortController()
      try {
        hits = await api.search(q, ctrl.signal)
      } catch {}
    }, 120)
  }

  function go(id: string) {
    app.searchOpen = false
    app.openChat(id)
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'ArrowDown') { e.preventDefault(); active = Math.min(active + 1, results.length - 1) }
    else if (e.key === 'ArrowUp') { e.preventDefault(); active = Math.max(active - 1, 0) }
    else if (e.key === 'Enter' && results[active]) go(results[active].chat_id)
    else if (e.key === 'Escape') app.searchOpen = false
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div class="fixed inset-0 z-50 flex justify-center bg-black/40 p-4 pt-[12vh] backdrop-blur-[2px]" onclick={(e) => e.target === e.currentTarget && (app.searchOpen = false)}>
  <div class="animate-pop flex h-fit max-h-[70vh] w-full max-w-xl flex-col overflow-hidden rounded-2xl border border-line bg-elev shadow-pop">
    <div class="flex items-center gap-3 border-b border-line px-4">
      <Icon name="search" class="text-faint" />
      <!-- svelte-ignore a11y_autofocus -->
      <input bind:value={q} {oninput} {onkeydown} autofocus placeholder="Search conversations…" class="h-14 w-full bg-transparent outline-none placeholder:text-faint" />
      <kbd class="rounded border border-line px-1.5 text-[11px] text-faint">Esc</kbd>
    </div>
    <div class="overflow-y-auto p-1.5">
      {#if !q.trim()}<div class="px-2.5 pb-1 pt-1.5 text-[11px] font-semibold uppercase tracking-wider text-faint">Recent</div>{/if}
      {#each results as h, i (h.chat_id)}
        <button class="menu-item !items-start flex-col !gap-0.5" data-active={i === active} onclick={() => go(h.chat_id)} onmousemove={() => (active = i)}>
          <span class="w-full truncate font-medium">{h.title}</span>
          {#if h.snippet}<span class="line-clamp-2 w-full text-xs text-muted">{@html DOMPurify.sanitize(h.snippet, { ALLOWED_TAGS: ['mark'] })}</span>{/if}
        </button>
      {:else}
        <div class="px-3 py-8 text-center text-sm text-muted">{q.trim() ? 'No results' : 'No conversations yet'}</div>
      {/each}
    </div>
  </div>
</div>
