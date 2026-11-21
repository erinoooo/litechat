<script lang="ts">
  import { onDestroy, onMount } from 'svelte'
  import { api, type SemanticStatus } from '../api'
  import { app } from '../state.svelte'
  import { Badge, Button, FilePicker, IconButton, Notice, PageHeader, Progress, Section, Select, TextField } from '../ui'

  let status = $state<SemanticStatus | null>(null)
  let importing = $state(false)

  // embedding providers: Anthropic has no embeddings API
  const providers = $derived(app.providers.filter((p) => p.kind === 'openai'))
  const current = $derived(app.prefs.embedding_model ?? '')
  let providerId = $state('')
  let modelId = $state('')

  $effect.pre(() => {
    // fill the pickers from saved settings once
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

  const chosen = $derived(`${providerId}/${modelId.trim()}`)

  async function enable() {
    if (!providerId || !modelId.trim()) return
    await app.savePrefs({ semantic_search: true, embedding_model: chosen })
    app.notify('Indexing your chats')
    poll()
  }

  async function disable() {
    await app.savePrefs({ semantic_search: false })
    poll()
  }

  async function doImport(files: File[]) {
    importing = true
    try {
      const r = await api.importChats(files[0])
      app.chats = await api.chats()
      app.notify(`Imported ${r.imported} chat${r.imported === 1 ? '' : 's'}`)
    } catch (e) {
      app.notify((e as Error).message, 'error')
    } finally {
      importing = false
    }
  }

  const pct = $derived(status?.total ? Math.round(((status.indexed ?? 0) / status.total) * 100) : 0)
</script>

<PageHeader title="Search and data" />

<Section title="Search by meaning" description="Find chats by what they were about, not only the exact words. An embedding model from one of your providers indexes each reply; the index lives in your database.">
  {#snippet actions()}{#if status?.enabled}<Badge tone="pine">On</Badge>{/if}{/snippet}
  {#if !providers.length}
    <Notice>Add an OpenAI-compatible provider first. Anthropic doesn't offer embeddings.</Notice>
  {:else}
    <div class="flex max-w-md flex-col gap-4">
      <Select label="Provider" bind:value={providerId} options={providers.map((p) => ({ value: p.id, label: p.name }))} />
      <TextField label="Embedding model" bind:value={modelId} mono list="embed-models" placeholder="nomic-embed-text"
        hint="For example nomic-embed-text on Ollama or text-embedding-3-small on OpenAI." />
      <datalist id="embed-models">
        {#each suggestions as m}<option value={m}></option>{/each}
      </datalist>
      <div class="flex gap-2">
        <Button variant="primary" disabled={!modelId.trim() || (status?.enabled && current === chosen)} onclick={enable}>
          {status?.enabled ? 'Switch model' : 'Turn on'}
        </Button>
        {#if status?.enabled}<Button onclick={disable}>Turn off</Button>{/if}
      </div>
    </div>

    {#if status?.enabled}
      <div class="mt-6 max-w-md">
        <div class="mb-2 flex items-center justify-between gap-4 text-sm">
          {#if status.error}
            <span class="text-red">Indexing stopped: {status.error}</span>
          {:else}
            <span class="text-ink-2">{status.running ? 'Indexing' : 'Indexed'} {status.indexed} of {status.total} replies</span>
          {/if}
          <IconButton icon="refresh" label="Index now" size="sm" spin={status.running} onclick={() => api.reindex().then(poll)} />
        </div>
        <Progress value={pct} label="Indexing progress" />
        <p class="mt-2 text-xs text-ink-3">Switching models rebuilds the index.</p>
      </div>
    {/if}
  {/if}
</Section>

<Section title="Import and export" description="Bring chats from liteChat, ChatGPT (conversations.json) or Open WebUI. Edited and regenerated versions are kept.">
  <div class="flex flex-wrap gap-2">
    <Button href="/api/export" download icon="download">Export all chats</Button>
    <FilePicker as="button" icon="upload" label={importing ? 'Importing' : 'Import chats'} accept=".json,application/json" disabled={importing} onfiles={doImport} />
  </div>
</Section>
