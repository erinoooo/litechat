<script lang="ts">
  import type { Chat } from '../api'
  import { app } from '../state.svelte'
  import { Button, IconButton, Kbd, Menu, MenuGroup, MenuItem, NavItem } from '../ui'

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
    if (confirm(`Delete “${c.title}”? This can't be undone.`)) await app.deleteChat(c.id)
  }

  function focusSelect(node: HTMLInputElement) {
    node.focus()
    node.select()
  }

  const themes = ['system', 'light', 'dark'] as const
  const themeIcon = { system: 'monitor', light: 'sun', dark: 'moon' }
</script>

<aside class="flex h-full w-sidebar flex-col border-r border-line bg-panel">
  <div class="flex h-14 shrink-0 items-center justify-between pr-2 pl-4">
    <a href="/" class="flex items-center gap-2 text-base font-semibold tracking-tight" onclick={(e) => { e.preventDefault(); app.newChat() }}>
      <img src="/icon.svg" alt="" class="size-5" />liteChat
    </a>
    <div class="flex">
      <IconButton icon="search" label="Search chats (Ctrl+K)" onclick={() => (app.searchOpen = true)} />
      <IconButton icon="sidebar" label="Hide sidebar (Ctrl+\)" onclick={() => (app.sidebarOpen = false)} />
    </div>
  </div>

  <div class="px-2 pb-2">
    <Button icon="plus" full onclick={() => app.newChat()}>
      <span class="flex-1 text-left">New chat</span><Kbd keys="Ctrl ⇧ O" />
    </Button>
  </div>

  <nav class="min-h-0 flex-1 overflow-y-auto px-2 pb-3" aria-label="Chats">
    {#each groups as [label, chats] (label)}
      <div class="mt-3">
        <MenuGroup {label}>
          {#each chats as c (c.id)}
            {#if renaming === c.id}
              <!-- ui-allow: inline rename needs a bare input sized like the row -->
              <input
                use:focusSelect
                bind:value={renameText}
                aria-label="Chat title"
                onblur={() => commitRename(c.id)}
                onkeydown={(e) => { if (e.key === 'Enter') commitRename(c.id); if (e.key === 'Escape') renaming = null }}
                class="h-8 w-full rounded-md border border-pine bg-canvas px-2.5 text-sm outline-none"
              />
            {:else}
              <NavItem
                label={c.title}
                href="/c/{c.id}"
                active={app.current?.chat.id === c.id}
                onselect={() => app.openChat(c.id)}
                ondblclick={() => startRename(c)}
                pinActions={menuFor === c.id}
              >
                {#snippet actions()}
                  <Menu align="end" open={menuFor === c.id} onclose={() => (menuFor = null)}>
                    {#snippet trigger()}
                      <IconButton icon="more" label="Chat options" size="sm" expanded={menuFor === c.id} onclick={() => (menuFor = menuFor === c.id ? null : c.id)} />
                    {/snippet}
                    <MenuItem icon="pin" label={c.pinned ? 'Unpin' : 'Pin'} onclick={() => { menuFor = null; app.togglePin(c.id) }} />
                    <MenuItem icon="edit" label="Rename" onclick={() => startRename(c)} />
                    <MenuItem icon="download" label="Download as Markdown" onclick={() => { menuFor = null; app.exportChatMarkdown(c.id) }} />
                    <MenuItem icon="trash" label="Delete" tone="danger" onclick={() => remove(c)} />
                  </Menu>
                {/snippet}
              </NavItem>
            {/if}
          {/each}
        </MenuGroup>
      </div>
    {:else}
      <p class="px-2.5 py-8 text-sm text-ink-3">Your chats will show up here.</p>
    {/each}
  </nav>

  <div class="flex items-center gap-1 border-t border-line p-2">
    <Button variant="ghost" icon="settings" onclick={() => { app.settingsTab = 'providers'; app.settingsOpen = true }}>Settings</Button>
    <span class="flex-1"></span>
    <IconButton
      icon={themeIcon[app.theme]}
      label="Theme: {app.theme}"
      onclick={() => app.setTheme(themes[(themes.indexOf(app.theme) + 1) % 3])}
    />
    <IconButton icon="logout" label="Log out" onclick={() => app.logout()} />
  </div>
</aside>
