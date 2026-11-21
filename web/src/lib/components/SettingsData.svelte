<script lang="ts">
  import { onDestroy, onMount } from 'svelte'
  import { api, type SemanticStatus } from '../api'
  import { app } from '../state.svelte'
  import Icon from './Icon.svelte'

  let status = $state<SemanticStatus | null>(null)
  let importing = $state(false)
  let fileInput = $state<HTMLInputElement>()

  // embedding providers: Anthropic has no embeddings API
  const providers = $derived(app.providers.filter((p) => p.kind === 'openai'))
  const current = $derived(app.prefs.embedding_model ?? '')
  let providerId = $state('')
  let modelId = $state('')

  $effect.pre(() => {
    // initialize the pickers from saved prefs once
    if (!providerId && current) {
      const i = current.indexOf('/')
      providerId = current.slice(0, i)
      modelId = current.slice(i + 1)
    }
    if (!providerId && providers.length) providerId = providers[0].id
  })

  const suggestions = $derived(
    app.models
      .filter((m) => m.provider_id === providerId)
      .map((m) => m.id)
      .sort((a, b) => Number(/embed/i.test(b)) - Number(/embed/i.test(a))),
  )

  let timer: ReturnType<typeof setInterval>
  async function poll() {
    try {
      status = await api.semanticStatus()
    } catch {}
  }
  onMount(() => {
    poll()
    timer = setInterval(poll, 2000)
  })
  onDestroy(() => clearInterval(timer))

  async function saveEmbedding() {
    if (!providerId || !modelId.trim()) return
    await app.savePrefs({ semantic_search: true, embedding_model: `${providerId}/${modelId.trim()}` })
    app.notify('Indexing your chats in the background…')
    poll()
  }

  async function disable() {
    await app.savePrefs({ semantic_search: false })
    poll()
  }

  async function doImport(f: File) {
    importing = true
    try {
      const r = await api.importChats(f)
      app.chats = await api.chats()
      app.notify(`Imported ${r.imported} chat${r.imported === 1 ? '' : 's'} (${r.format})`)
    } catch (e) {
      app.notify((e as Error).message, 'error')
    } finally {
      importing = false
    }
  }

  const pct = $derived(status?.total ? Math.round(((status.indexed ?? 0) / status.total) * 100) : 0)
</script>

<h3 class="mb-1 text-lg font-semibold">Search &amp; data</h3>
<p class="mb-6 text-sm text-muted">Find chats by meaning, and move your history in and out.</p>

<section class="mb-8">
  <div class="mb-1 flex items-center gap-2">
    <h4 class="text-sm font-semibold">Search by meaning</h4>
    {#if status?.enabled}<span class="rounded-full bg-accent-soft px-2 py-0.5 text-[11px] font-medium text-accent">On</span>{/if}
  </div>
  <p class="mb-4 text-sm text-muted">
    Indexes every exchange with an embedding model so Ctrl+K also finds chats when you only remember what they were about.
    Runs through your own provider; try <code class="font-mono text-xs">nomic-embed-text</code> on Ollama or <code class="font-mono text-xs">text-embedding-3-small</code> on OpenAI.
  </p>

  {#if !providers.length}
    <div class="rounded-xl border border-dashed border-line px-4 py-4 text-sm text-faint">Add an OpenAI-compatible provider first.</div>
  {:else}
    <div class="grid gap-2 sm:grid-cols-[minmax(0,1fr)_minmax(0,1.3fr)_auto]">
      <select class="input" bind:value={providerId} aria-label="Embedding provider">
        {#each providers as p (p.id)}<option value={p.id}>{p.name}</option>{/each}
      </select>
      <input class="input font-mono text-[13px]" list="embed-models" bind:value={modelId} placeholder="Embedding model" aria-label="Embedding model" />
      <datalist id="embed-models">
        {#each suggestions as m}<option value={m}></option>{/each}
      </datalist>
      <button class="btn-primary" disabled={!modelId.trim() || (status?.enabled && current === `${providerId}/${modelId.trim()}`)} onclick={saveEmbedding}>
        {status?.enabled ? 'Switch' : 'Enable'}
      </button>
    </div>

    {#if status?.enabled}
      <div class="mt-4 rounded-xl border border-line px-4 py-3">
        <div class="mb-2 flex items-center justify-between text-sm">
          <span>
            {#if status.error}<span class="text-danger">Indexing failed: {status.error}</span>
            {:else if status.running}Indexing… {status.indexed} of {status.total}
            {:else}{status.indexed} of {status.total} replies indexed{/if}
          </span>
          <span class="flex gap-1">
            <button class="btn-icon" title="Index now" aria-label="Index now" onclick={() => api.reindex().then(poll)}><Icon name="refresh" size={15} class={status.running ? 'animate-spin' : ''} /></button>
            <button class="btn-ghost !h-7 !px-2.5 !text-xs" onclick={disable}>Turn off</button>
          </span>
        </div>
        <div class="h-1.5 overflow-hidden rounded-full bg-sunk">
          <div class="h-full rounded-full bg-accent transition-[width] duration-500" style="width: {pct}%"></div>
        </div>
        <p class="mt-2 text-xs text-faint">Changing the model rebuilds the index. Vectors stay in your SQLite file.</p>
      </div>
    {/if}
  {/if}
</section>

<section>
  <h4 class="mb-1 text-sm font-semibold">Import &amp; export</h4>
  <p class="mb-4 text-sm text-muted">Import from liteChat, ChatGPT (<code class="font-mono text-xs">conversations.json</code>) or Open WebUI exports. Branches are kept.</p>
  <div class="flex flex-wrap gap-2">
    <a class="btn-ghost" href="/api/export" download><Icon name="download" size={15} /> Export all chats</a>
    <button class="btn-ghost" disabled={importing} onclick={() => fileInput?.click()}>
      <Icon name="upload" size={15} /> {importing ? 'Importing…' : 'Import…'}
    </button>
    <input bind:this={fileInput} type="file" accept=".json,application/json" class="hidden"
      onchange={(e) => { const f = e.currentTarget.files?.[0]; if (f) doImport(f); e.currentTarget.value = '' }} />
  </div>
</section>
