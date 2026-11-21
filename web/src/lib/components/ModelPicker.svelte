<script lang="ts">
  import { app } from '../state.svelte'
  import Icon from './Icon.svelte'

  let open = $state(false)
  let q = $state('')
  let active = $state(0)
  let input = $state<HTMLInputElement>()
  let root = $state<HTMLDivElement>()

  const current = $derived(app.activeModel)
  const filtered = $derived.by(() => {
    const s = q.trim().toLowerCase()
    return s
      ? app.models.filter((m) => (m.name + ' ' + m.id + ' ' + m.provider_name).toLowerCase().includes(s))
      : app.models
  })
  const groups = $derived.by(() => {
    const g = new Map<string, typeof filtered>()
    for (const m of filtered) {
      if (!g.has(m.provider_name)) g.set(m.provider_name, [])
      g.get(m.provider_name)!.push(m)
    }
    return [...g.entries()]
  })
  const errors = $derived(Object.entries(app.modelErrors))

  function toggle() {
    open = !open
    if (open) {
      q = ''
      active = Math.max(0, filtered.findIndex((m) => m.key === current))
      queueMicrotask(() => {
        input?.focus()
        root?.querySelector('[data-active="true"]')?.scrollIntoView({ block: 'nearest' })
      })
    }
  }

  function pick(key: string) {
    app.setModel(key)
    open = false
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'ArrowDown') { e.preventDefault(); active = Math.min(active + 1, filtered.length - 1) }
    else if (e.key === 'ArrowUp') { e.preventDefault(); active = Math.max(active - 1, 0) }
    else if (e.key === 'Enter') { e.preventDefault(); if (filtered[active]) pick(filtered[active].key) }
    else if (e.key === 'Escape') open = false
    else return
    queueMicrotask(() => root?.querySelector('[data-active="true"]')?.scrollIntoView({ block: 'nearest' }))
  }

  function outside(node: HTMLElement) {
    const h = (e: PointerEvent) => { if (!node.contains(e.target as Node)) open = false }
    document.addEventListener('pointerdown', h, true)
    return { destroy: () => document.removeEventListener('pointerdown', h, true) }
  }
</script>

<div class="relative min-w-0" use:outside bind:this={root}>
  <button
    class="flex min-w-0 max-w-full items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-[15px] font-semibold transition-colors hover:bg-hover"
    onclick={toggle}
    aria-haspopup="listbox"
    aria-expanded={open}
  >
    <span class="truncate">{current ? app.modelName(current) : app.modelsLoading ? 'Loading models…' : 'Select a model'}</span>
    <Icon name="chevronDown" size={15} class="text-faint" />
  </button>

  {#if open}
    <div class="menu animate-pop absolute left-0 top-full z-40 mt-1.5 flex max-h-[min(70vh,480px)] w-[min(92vw,360px)] flex-col">
      <div class="flex items-center gap-2 border-b border-line px-2.5 pb-1.5 pt-1">
        <Icon name="search" size={15} class="text-faint" />
        <input bind:this={input} bind:value={q} {onkeydown} oninput={() => (active = 0)} placeholder="Search models" class="h-8 w-full bg-transparent text-sm outline-none placeholder:text-faint" />
        <button class="btn-icon !h-7 !w-7" title="Refresh model list" aria-label="Refresh model list" onclick={() => app.loadModels(true)}>
          <Icon name="refresh" size={14} class={app.modelsLoading ? 'animate-spin' : ''} />
        </button>
      </div>
      <div class="overflow-y-auto py-1" role="listbox">
        {#each groups as [provider, models]}
          <div class="px-2.5 pb-1 pt-2 text-[11px] font-semibold uppercase tracking-wider text-faint">{provider}</div>
          {#each models as m (m.key)}
            {@const idx = filtered.indexOf(m)}
            <button
              class="menu-item justify-between"
              data-active={idx === active}
              role="option"
              aria-selected={m.key === current}
              onclick={() => pick(m.key)}
              onmousemove={() => (active = idx)}
            >
              <span class="min-w-0 truncate">{m.name}</span>
              {#if m.key === current}<Icon name="check" size={15} class="text-accent" />{/if}
            </button>
          {/each}
        {:else}
          <div class="px-3 py-6 text-center text-sm text-muted">
            {app.models.length ? 'No matching models' : 'No models yet'}
          </div>
        {/each}
        {#each errors as [pid, err]}
          <div class="mx-1 mt-1 rounded-lg bg-danger/8 px-2.5 py-2 text-xs text-danger">
            <b>{app.providers.find((p) => p.id === pid)?.name ?? 'Provider'}:</b> {err}
          </div>
        {/each}
      </div>
      <div class="border-t border-line pt-1">
        <button class="menu-item text-muted" onclick={() => { open = false; app.settingsTab = 'providers'; app.settingsOpen = true }}>
          <Icon name="settings" size={15} /> Manage providers
        </button>
      </div>
    </div>
  {/if}
</div>
