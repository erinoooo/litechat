<script lang="ts">
  import { api, type Preset, type Provider } from '../api'
  import { app } from '../state.svelte'
  import Icon from './Icon.svelte'
  import SettingsTools from './SettingsTools.svelte'
  import SettingsData from './SettingsData.svelte'

  const templates = [
    { name: 'Ollama', kind: 'openai', base_url: 'http://localhost:11434/v1', key: false },
    { name: 'OpenAI', kind: 'openai', base_url: 'https://api.openai.com/v1', key: true },
    { name: 'Anthropic', kind: 'anthropic', base_url: 'https://api.anthropic.com', key: true },
    { name: 'OpenRouter', kind: 'openai', base_url: 'https://openrouter.ai/api/v1', key: true },
    { name: 'LM Studio', kind: 'openai', base_url: 'http://localhost:1234/v1', key: false },
    { name: 'llama.cpp', kind: 'openai', base_url: 'http://localhost:8080/v1', key: false },
    { name: 'Groq', kind: 'openai', base_url: 'https://api.groq.com/openai/v1', key: true },
    { name: 'Custom', kind: 'openai', base_url: '', key: true },
  ] as const

  // ---------- providers ----------
  type Form = { id?: string; name: string; kind: 'openai' | 'anthropic'; base_url: string; api_key: string; has_key: boolean }
  let form = $state<Form | null>(null)
  let saving = $state(false)

  function addFrom(t: (typeof templates)[number]) {
    form = { name: t.name === 'Custom' ? '' : t.name, kind: t.kind, base_url: t.base_url, api_key: '', has_key: false }
  }

  function edit(p: Provider) {
    form = { id: p.id, name: p.name, kind: p.kind, base_url: p.base_url, api_key: '', has_key: p.has_key }
  }

  async function saveProvider() {
    if (!form) return
    saving = true
    try {
      const body: Record<string, unknown> = { name: form.name, kind: form.kind, base_url: form.base_url }
      if (!form.id || form.api_key) body.api_key = form.api_key
      await api.saveProvider(body, form.id)
      app.providers = await api.providers()
      form = null
      await app.loadModels(true)
      app.notify('Provider saved')
    } catch (e) {
      app.notify((e as Error).message, 'error')
    } finally {
      saving = false
    }
  }

  async function toggleProvider(p: Provider) {
    await api.saveProvider({ enabled: !p.enabled }, p.id)
    app.providers = await api.providers()
    app.loadModels(true)
  }

  async function removeProvider(p: Provider) {
    if (!confirm(`Remove ${p.name}?`)) return
    await api.deleteProvider(p.id)
    app.providers = await api.providers()
    app.loadModels(true)
  }

  // ---------- presets ----------
  let preset = $state<Partial<Preset> | null>(null)

  async function savePreset() {
    if (!preset?.name?.trim()) return
    try {
      await api.savePreset({ name: preset.name, prompt: preset.prompt ?? '' }, preset.id)
      app.presets = await api.presets()
      preset = null
    } catch (e) {
      app.notify((e as Error).message, 'error')
    }
  }

  async function removePreset(p: Preset) {
    if (!confirm(`Delete preset “${p.name}”?`)) return
    await api.deletePreset(p.id)
    app.presets = await api.presets()
  }

  // ---------- password ----------
  let pw = $state({ current: '', next: '' })
  async function changePassword() {
    try {
      await api.changePassword(pw.current, pw.next)
      pw = { current: '', next: '' }
      app.notify('Password changed. Other devices were logged out.')
    } catch (e) {
      app.notify((e as Error).message, 'error')
    }
  }

  const tabs = [
    { id: 'providers', label: 'Providers', icon: 'sparkle' },
    { id: 'tools', label: 'Tools', icon: 'tool' },
    { id: 'presets', label: 'Prompts', icon: 'message' },
    { id: 'data', label: 'Search & data', icon: 'database' },
    { id: 'general', label: 'General', icon: 'settings' },
  ] as const

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Escape' && !form && !preset) app.settingsOpen = false
  }
</script>

<svelte:window {onkeydown} />

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div class="fixed inset-0 z-50 grid place-items-center bg-black/40 p-3 backdrop-blur-[2px] sm:p-6" onclick={(e) => e.target === e.currentTarget && (app.settingsOpen = false)}>
  <div class="animate-pop flex h-[min(640px,92vh)] w-full max-w-3xl overflow-hidden rounded-2xl border border-line bg-elev shadow-pop max-sm:flex-col">
    <nav class="flex shrink-0 gap-1 border-line bg-sunk p-3 sm:w-48 sm:flex-col sm:border-r max-sm:overflow-x-auto max-sm:border-b">
      <h2 class="mb-2 px-2.5 pt-1 text-base font-semibold max-sm:hidden">Settings</h2>
      {#each tabs as t}
        <button class="menu-item max-sm:!w-auto" data-active={app.settingsTab === t.id} onclick={() => { app.settingsTab = t.id; form = null; preset = null }}>
          <Icon name={t.icon} size={16} /> {t.label}
        </button>
      {/each}
      <button class="btn-icon ml-auto sm:hidden" onclick={() => (app.settingsOpen = false)} aria-label="Close"><Icon name="x" /></button>
    </nav>

    <section class="relative min-w-0 flex-1 overflow-y-auto p-5 sm:p-6">
      <button class="btn-icon absolute right-3 top-3 max-sm:hidden" onclick={() => (app.settingsOpen = false)} aria-label="Close"><Icon name="x" /></button>

      {#if app.settingsTab === 'providers'}
        {#if form}
          <h3 class="mb-5 text-lg font-semibold">{form.id ? 'Edit provider' : 'Add provider'}</h3>
          <div class="space-y-4">
            <div>
              <label class="label" for="p-name">Name</label>
              <input id="p-name" class="input" bind:value={form.name} placeholder="My provider" />
            </div>
            <div>
              <label class="label" for="p-kind">API type</label>
              <select id="p-kind" class="input" bind:value={form.kind}>
                <option value="openai">OpenAI-compatible (Ollama, OpenRouter, vLLM, LM Studio…)</option>
                <option value="anthropic">Anthropic (Claude)</option>
              </select>
            </div>
            <div>
              <label class="label" for="p-url">Base URL</label>
              <input id="p-url" class="input font-mono text-[13px]" bind:value={form.base_url} placeholder="https://api.example.com/v1" />
            </div>
            <div>
              <label class="label" for="p-key">API key</label>
              <input id="p-key" class="input font-mono text-[13px]" type="password" autocomplete="off" bind:value={form.api_key}
                placeholder={form.has_key ? '•••••••• (saved; leave blank to keep)' : 'Optional for local servers'} />
              <p class="mt-1.5 text-xs text-faint">Stored on the server only; never sent to the browser.</p>
            </div>
          </div>
          <div class="mt-6 flex justify-end gap-2">
            <button class="btn-ghost" onclick={() => (form = null)}>Cancel</button>
            <button class="btn-primary" disabled={saving || !form.base_url} onclick={saveProvider}>{saving ? 'Saving…' : 'Save'}</button>
          </div>
        {:else}
          <h3 class="mb-1 text-lg font-semibold">Providers</h3>
          <p class="mb-5 text-sm text-muted">Connect any OpenAI-compatible server or Anthropic. Models are discovered automatically.</p>

          {#if app.providers.length}
            <div class="mb-6 divide-y divide-line overflow-hidden rounded-xl border border-line">
              {#each app.providers as p (p.id)}
                {@const err = app.modelErrors[p.id]}
                {@const count = app.models.filter((m) => m.provider_id === p.id).length}
                <div class="flex items-center gap-3 px-4 py-3 {p.enabled ? '' : 'opacity-55'}">
                  <span class="h-2 w-2 shrink-0 rounded-full {!p.enabled ? 'bg-faint' : err ? 'bg-danger' : 'bg-emerald-500'}" title={err ?? ''}></span>
                  <div class="min-w-0 flex-1">
                    <div class="truncate text-sm font-medium">{p.name}</div>
                    <div class="truncate text-xs text-faint">
                      {#if err}<span class="text-danger">{err}</span>{:else}{p.base_url} · {count} model{count === 1 ? '' : 's'}{/if}
                    </div>
                  </div>
                  <label class="relative inline-flex cursor-pointer items-center" title={p.enabled ? 'Disable' : 'Enable'}>
                    <input type="checkbox" class="peer sr-only" checked={p.enabled} onchange={() => toggleProvider(p)} />
                    <span class="h-5 w-9 rounded-full bg-line-strong transition-colors peer-checked:bg-accent"></span>
                    <span class="absolute left-0.5 top-0.5 h-4 w-4 rounded-full bg-white shadow transition-transform peer-checked:translate-x-4"></span>
                  </label>
                  <button class="btn-icon" onclick={() => edit(p)} aria-label="Edit"><Icon name="edit" size={16} /></button>
                  <button class="btn-icon hover:!text-danger" onclick={() => removeProvider(p)} aria-label="Delete"><Icon name="trash" size={16} /></button>
                </div>
              {/each}
            </div>
          {/if}

          <div class="label">Add a provider</div>
          <div class="grid grid-cols-2 gap-2 sm:grid-cols-4">
            {#each templates as t}
              <button class="rounded-xl border border-line px-3 py-2.5 text-left text-sm transition-colors hover:border-line-strong hover:bg-hover" onclick={() => addFrom(t)}>
                <div class="font-medium">{t.name}</div>
                <div class="text-xs text-faint">{t.key ? 'API key' : 'Local'}</div>
              </button>
            {/each}
          </div>
        {/if}

      {:else if app.settingsTab === 'tools'}
        <SettingsTools />

      {:else if app.settingsTab === 'data'}
        <SettingsData />

      {:else if app.settingsTab === 'presets'}
        {#if preset}
          <h3 class="mb-5 text-lg font-semibold">{preset.id ? 'Edit prompt' : 'New prompt'}</h3>
          <div class="space-y-4">
            <div>
              <label class="label" for="pr-name">Name</label>
              <input id="pr-name" class="input" bind:value={preset.name} placeholder="Code reviewer" />
            </div>
            <div>
              <label class="label" for="pr-prompt">System prompt</label>
              <textarea id="pr-prompt" class="input" rows="10" bind:value={preset.prompt} placeholder="You are a meticulous senior engineer…"></textarea>
            </div>
          </div>
          <div class="mt-6 flex justify-end gap-2">
            <button class="btn-ghost" onclick={() => (preset = null)}>Cancel</button>
            <button class="btn-primary" disabled={!preset.name?.trim()} onclick={savePreset}>Save</button>
          </div>
        {:else}
          <div class="mb-5 flex items-start justify-between gap-4">
            <div>
              <h3 class="mb-1 text-lg font-semibold">Prompts</h3>
              <p class="text-sm text-muted">Reusable system prompts, one click away on the new chat screen.</p>
            </div>
            <button class="btn-primary shrink-0" onclick={() => (preset = { name: '', prompt: '' })}><Icon name="plus" size={15} /> New</button>
          </div>
          {#each app.presets as p (p.id)}
            <div class="mb-2 flex items-center gap-3 rounded-xl border border-line px-4 py-3">
              <div class="min-w-0 flex-1">
                <div class="text-sm font-medium">{p.name}</div>
                <div class="truncate text-xs text-faint">{p.prompt}</div>
              </div>
              <button class="btn-icon" onclick={() => (preset = { ...p })} aria-label="Edit"><Icon name="edit" size={16} /></button>
              <button class="btn-icon hover:!text-danger" onclick={() => removePreset(p)} aria-label="Delete"><Icon name="trash" size={16} /></button>
            </div>
          {:else}
            <div class="rounded-xl border border-dashed border-line px-4 py-10 text-center text-sm text-faint">No prompts yet</div>
          {/each}
        {/if}

      {:else}
        <h3 class="mb-5 text-lg font-semibold">General</h3>
        <div class="space-y-6">
          <div>
            <label class="label" for="g-model">Default model for new chats</label>
            <select id="g-model" class="input" value={app.prefs.default_model ?? ''} onchange={(e) => { const v = e.currentTarget.value; app.savePrefs({ default_model: v }); app.draftModel = v || app.draftModel }}>
              <option value="">First available</option>
              {#each app.models as m (m.key)}<option value={m.key}>{m.provider_name} · {m.name}</option>{/each}
            </select>
          </div>
          <div>
            <label class="label" for="g-sys">Default system prompt</label>
            <textarea id="g-sys" class="input" rows="3" value={app.prefs.default_system_prompt ?? ''}
              onchange={(e) => app.savePrefs({ default_system_prompt: e.currentTarget.value })} placeholder="Applied to every new chat"></textarea>
          </div>

          {#snippet toggle(label: string, hint: string, on: boolean, set: (v: boolean) => void)}
            <label class="flex cursor-pointer items-center justify-between gap-4">
              <span><span class="block text-sm font-medium">{label}</span><span class="block text-xs text-faint">{hint}</span></span>
              <span class="relative inline-flex shrink-0 items-center">
                <input type="checkbox" class="peer sr-only" checked={on} onchange={(e) => set(e.currentTarget.checked)} />
                <span class="h-5 w-9 rounded-full bg-line-strong transition-colors peer-checked:bg-accent"></span>
                <span class="absolute left-0.5 top-0.5 h-4 w-4 rounded-full bg-white shadow transition-transform peer-checked:translate-x-4"></span>
              </span>
            </label>
          {/snippet}
          {@render toggle('Auto-generate titles', 'Ask the model to name each new chat', app.prefs.auto_title ?? true, (v) => app.savePrefs({ auto_title: v }))}
          {@render toggle('Enter sends', 'Off: Ctrl/⌘+Enter sends, Enter adds a new line', app.prefs.send_on_enter ?? true, (v) => app.savePrefs({ send_on_enter: v }))}

          <div>
            <div class="label">Theme</div>
            <div class="inline-flex rounded-xl border border-line p-1">
              {#each [['system', 'monitor', 'System'], ['light', 'sun', 'Light'], ['dark', 'moon', 'Dark']] as [v, icon, label]}
                <button class="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm {app.theme === v ? 'bg-hover font-medium' : 'text-muted'}" onclick={() => app.setTheme(v as 'system')}>
                  <Icon name={icon} size={15} /> {label}
                </button>
              {/each}
            </div>
          </div>

          <div class="border-t border-line pt-6">
            <div class="label">Change password</div>
            <div class="grid gap-2 sm:grid-cols-[1fr_1fr_auto]">
              <input class="input" type="password" placeholder="Current" autocomplete="current-password" bind:value={pw.current} />
              <input class="input" type="password" placeholder="New (8+ chars)" autocomplete="new-password" bind:value={pw.next} />
              <button class="btn-primary" disabled={!pw.current || pw.next.length < 8} onclick={changePassword}>Update</button>
            </div>
          </div>
        </div>
      {/if}
    </section>
  </div>
</div>
