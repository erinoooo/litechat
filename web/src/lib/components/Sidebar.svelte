<script lang="ts">
  import type { Chat } from '../api'
  import { app } from '../state.svelte'
  import Icon from './Icon.svelte'

  let menuFor = $state<string | null>(null)
  let renaming = $state<string | null>(null)
  let renameText = $state('')

  const DAY = 86_400_000
  const groups = $derived.by(() => {
    const startToday = new Date().setHours(0, 0, 0, 0)
    const buckets: [string, Chat[]][] = [
      ['Pinned', []], ['Today', []], ['Yesterday', []], ['Previous 7 days', []], ['Previous 30 days', []], ['Older', []],
    ]
    for (const c of app.chats) {
      const t = c.updated_at
      const i = c.pinned ? 0 : t >= startToday ? 1 : t >= startToday - DAY ? 2 : t >= startToday - 7 * DAY ? 3 : t >= startToday - 30 * DAY ? 4 : 5
      buckets[i][1].push(c)
    }
    return buckets.filter(([, list]) => list.length)
  })

  function startRename(c: Chat) {
    menuFor = null
    renaming = c.id
    renameText = c.title
  }

  function commitRename(id: string) {
    if (renaming !== id) return
    renaming = null
    app.renameChat(id, renameText)
  }

  async function remove(c: Chat) {
    menuFor = null
    if (confirm(`Delete “${c.title}”?`)) await app.deleteChat(c.id)
  }

  function focusSelect(node: HTMLInputElement) {
    node.focus()
    node.select()
  }

  function outside(node: HTMLElement) {
    const h = (e: PointerEvent) => { if (!node.contains(e.target as Node)) menuFor = null }
    document.addEventListener('pointerdown', h, true)
    return { destroy: () => document.removeEventListener('pointerdown', h, true) }
  }

  function cycleTheme() {
    app.setTheme(app.theme === 'system' ? 'light' : app.theme === 'light' ? 'dark' : 'system')
  }
</script>

<aside class="flex h-full w-[272px] flex-col border-r border-line bg-sunk">
  <div class="flex items-center justify-between px-3 pb-2 pt-3">
    <div class="flex items-center gap-2 pl-1.5 font-semibold tracking-tight">
      <img src="/icon.svg" alt="" class="h-6 w-6" />
      liteChat
    </div>
    <div class="flex">
      <button class="btn-icon" onclick={() => (app.searchOpen = true)} title="Search (Ctrl+K)" aria-label="Search chats"><Icon name="search" /></button>
      <button class="btn-icon" onclick={() => (app.sidebarOpen = false)} title="Close sidebar" aria-label="Close sidebar"><Icon name="sidebar" /></button>
    </div>
  </div>

  <div class="px-3 pb-2">
    <button
      class="flex w-full items-center gap-2.5 rounded-xl border border-line bg-elev px-3 py-2 text-sm font-medium shadow-soft transition-colors hover:bg-hover"
      onclick={() => app.newChat()}
    >
      <Icon name="plus" size={16} /> New chat
      <kbd class="ml-auto font-sans text-[11px] text-faint">Ctrl ⇧ O</kbd>
    </button>
  </div>

  <nav class="min-h-0 flex-1 overflow-y-auto px-2 pb-3">
    {#each groups as [label, chats] (label)}
      <div class="px-2.5 pb-1 pt-4 text-[11px] font-semibold uppercase tracking-wider text-faint">{label}</div>
      {#each chats as c (c.id)}
        {@const active = app.current?.chat.id === c.id}
        <div class="group relative" use:outside>
          {#if renaming === c.id}
            <input
              use:focusSelect
              bind:value={renameText}
              onblur={() => commitRename(c.id)}
              onkeydown={(e) => { if (e.key === 'Enter') commitRename(c.id); if (e.key === 'Escape') renaming = null }}
              class="w-full rounded-lg border border-accent bg-elev px-2.5 py-1.5 text-sm outline-none"
            />
          {:else}
            <a
              href="/c/{c.id}"
              onclick={(e) => { if (!e.metaKey && !e.ctrlKey) { e.preventDefault(); app.openChat(c.id) } }}
              ondblclick={() => startRename(c)}
              class="flex items-center gap-2 rounded-lg px-2.5 py-1.5 pr-8 text-sm transition-colors {active ? 'bg-hover font-medium text-fg' : 'text-muted hover:bg-hover/70 hover:text-fg'}"
            >
              <span class="truncate">{c.title}</span>
            </a>
            <button
              class="btn-icon absolute right-0.5 top-1/2 !h-7 !w-7 -translate-y-1/2 {menuFor === c.id || active ? 'opacity-100' : 'opacity-0 group-hover:opacity-100 max-md:opacity-100'}"
              onclick={() => (menuFor = menuFor === c.id ? null : c.id)}
              aria-label="Chat options"
            ><Icon name="more" size={16} stroke={2.5} /></button>
            {#if menuFor === c.id}
              <div class="menu animate-pop absolute right-0 top-full z-30 mt-1 w-44">
                <button class="menu-item" onclick={() => { menuFor = null; app.togglePin(c.id) }}><Icon name="pin" size={15} /> {c.pinned ? 'Unpin' : 'Pin'}</button>
                <button class="menu-item" onclick={() => startRename(c)}><Icon name="edit" size={15} /> Rename</button>
                <button class="menu-item !text-danger" onclick={() => remove(c)}><Icon name="trash" size={15} /> Delete</button>
              </div>
            {/if}
          {/if}
        </div>
      {/each}
    {:else}
      <div class="px-3 py-10 text-center text-sm text-faint">No conversations yet</div>
    {/each}
  </nav>

  <div class="flex items-center gap-1 border-t border-line px-3 py-2.5">
    <button class="menu-item !w-auto flex-1 text-muted" onclick={() => { app.settingsTab = 'providers'; app.settingsOpen = true }}>
      <Icon name="settings" size={16} /> Settings
    </button>
    <button class="btn-icon" onclick={cycleTheme} title="Theme: {app.theme}" aria-label="Switch theme">
      <Icon name={app.theme === 'light' ? 'sun' : app.theme === 'dark' ? 'moon' : 'monitor'} size={17} />
    </button>
    <button class="btn-icon" onclick={() => app.logout()} title="Log out" aria-label="Log out"><Icon name="logout" size={17} /></button>
  </div>
</aside>
