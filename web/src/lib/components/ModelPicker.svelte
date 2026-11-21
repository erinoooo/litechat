<script lang="ts">
  import { app } from '../state.svelte'
  import { Button, Menu, MenuGroup, MenuItem, Notice, SearchField } from '../ui'

  let open = $state(false)
  let q = $state('')
  let active = $state(0)

  const current = $derived(app.activeModel)
  const filtered = $derived.by(() => {
    const s = q.trim().toLowerCase()
    return s ? app.models.filter((m) => (m.name + ' ' + m.id + ' ' + m.provider_name).toLowerCase().includes(s)) : app.models
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

  function onopen() {
    q = ''
    active = Math.max(0, filtered.findIndex((m) => m.key === current))
    queueMicrotask(() => document.querySelector('[role=menu] [data-active]')?.scrollIntoView({ block: 'nearest' }))
  }

  function pick(key: string) {
    app.setModel(key)
    open = false
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'ArrowDown') { e.preventDefault(); active = Math.min(active + 1, filtered.length - 1) }
    else if (e.key === 'ArrowUp') { e.preventDefault(); active = Math.max(active - 1, 0) }
    else if (e.key === 'Enter') { e.preventDefault(); if (filtered[active]) pick(filtered[active].key) }
    else return
    queueMicrotask(() => document.querySelector('[role=menu] [data-active]')?.scrollIntoView({ block: 'nearest' }))
  }
</script>

<Menu bind:open width="wide" {onopen}>
  {#snippet trigger({ toggle })}
    <Button variant="title" menu expanded={open} onclick={toggle}>
      <span class="truncate">{current ? app.modelName(current) : app.modelsLoading ? 'Loading models' : 'Choose a model'}</span>
    </Button>
  {/snippet}
  {#snippet header()}
    <SearchField bind:value={q} placeholder="Find a model" autofocus {onkeydown} oninput={() => (active = 0)} />
  {/snippet}

  {#each groups as [provider, models] (provider)}
    <MenuGroup label={provider}>
      {#each models as m (m.key)}
        {@const idx = filtered.indexOf(m)}
        <MenuItem label={m.name} selected={m.key === current} active={idx === active} onclick={() => pick(m.key)} onmousemove={() => (active = idx)} />
      {/each}
    </MenuGroup>
  {:else}
    <p class="px-2.5 py-6 text-sm text-ink-3">{app.models.length ? 'No models match.' : 'No models yet. Add a provider to get started.'}</p>
  {/each}
  {#each errors as [pid, err]}
    <div class="mt-1"><Notice tone="error">{app.providers.find((p) => p.id === pid)?.name ?? 'A provider'} didn't respond: {err}</Notice></div>
  {/each}

  {#snippet footer()}
    <MenuItem icon="refresh" label="Refresh model list" onclick={() => app.loadModels(true)} />
    <MenuItem icon="settings" label="Manage providers" onclick={() => { open = false; app.settingsTab = 'providers'; app.settingsOpen = true }} />
  {/snippet}
</Menu>
