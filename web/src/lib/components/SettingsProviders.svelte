<script lang="ts">
  import { api, type Provider } from '../api'
  import { app } from '../state.svelte'
  import { Button, IconButton, List, ListRow, PageHeader, Section, Select, StatusDot, Switch, TextField } from '../ui'

  const templates = [
    { name: 'Ollama', kind: 'openai', base_url: 'http://localhost:11434/v1', note: 'Local, no key' },
    { name: 'llama.cpp', kind: 'openai', base_url: 'http://localhost:8080/v1', note: 'Local, no key' },
    { name: 'LM Studio', kind: 'openai', base_url: 'http://localhost:1234/v1', note: 'Local, no key' },
    { name: 'OpenAI', kind: 'openai', base_url: 'https://api.openai.com/v1', note: 'Needs an API key' },
    { name: 'Anthropic', kind: 'anthropic', base_url: 'https://api.anthropic.com', note: 'Needs an API key' },
    { name: 'OpenRouter', kind: 'openai', base_url: 'https://openrouter.ai/api/v1', note: 'Needs an API key' },
    { name: 'Groq', kind: 'openai', base_url: 'https://api.groq.com/openai/v1', note: 'Needs an API key' },
    { name: 'Other', kind: 'openai', base_url: '', note: 'Any OpenAI-compatible server' },
  ] as const

  type Form = { id?: string; name: string; kind: 'openai' | 'anthropic'; base_url: string; api_key: string; has_key: boolean }
  let form = $state<Form | null>(null)
  let saving = $state(false)

  function addFrom(t: (typeof templates)[number]) {
    form = { name: t.name === 'Other' ? '' : t.name, kind: t.kind, base_url: t.base_url, api_key: '', has_key: false }
  }

  function edit(p: Provider) {
    form = { id: p.id, name: p.name, kind: p.kind, base_url: p.base_url, api_key: '', has_key: p.has_key }
  }

  async function save() {
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

  async function toggle(p: Provider) {
    await api.saveProvider({ enabled: !p.enabled }, p.id)
    app.providers = await api.providers()
    app.loadModels(true)
  }

  async function remove(p: Provider) {
    if (!confirm(`Remove ${p.name}? Chats that used its models are kept.`)) return
    await api.deleteProvider(p.id)
    app.providers = await api.providers()
    app.loadModels(true)
  }

  const count = (id: string) => app.models.filter((m) => m.provider_id === id).length
</script>

{#if form}
  <PageHeader title={form.id ? `Edit ${form.name || 'provider'}` : 'Add a provider'} />
  <div class="flex max-w-md flex-col gap-4">
    <TextField label="Name" bind:value={form.name} placeholder="My server" />
    <Select label="API type" bind:value={form.kind} options={[
      { value: 'openai', label: 'OpenAI-compatible' },
      { value: 'anthropic', label: 'Anthropic' },
    ]} hint="Ollama, llama.cpp, vLLM, LM Studio and OpenRouter are all OpenAI-compatible." />
    <TextField label="Base URL" bind:value={form.base_url} mono placeholder="https://api.example.com/v1" />
    <TextField label="API key" type="password" autocomplete="off" bind:value={form.api_key} mono
      placeholder={form.has_key ? 'Saved. Leave blank to keep it.' : 'Leave blank for local servers'}
      hint="Kept on the server. The browser never sees it." />
    <div class="mt-2 flex gap-2">
      <Button variant="primary" disabled={saving || !form.base_url} onclick={save}>{saving ? 'Saving' : 'Save provider'}</Button>
      <Button onclick={() => (form = null)}>Cancel</Button>
    </div>
  </div>
{:else}
  <PageHeader title="Providers" description="Where your models come from. Models are listed automatically once a provider is connected." />

  {#if app.providers.length}
    <Section title="Connected">
      <List>
        {#each app.providers as p (p.id)}
          {@const err = app.modelErrors[p.id]}
          <ListRow title={p.name} subtitle="{p.base_url}, {count(p.id)} model{count(p.id) === 1 ? '' : 's'}" error={p.enabled ? err : undefined} dim={!p.enabled}>
            {#snippet leading()}<StatusDot status={!p.enabled ? 'off' : err ? 'error' : 'ok'} label={!p.enabled ? 'Off' : err ? 'Not responding' : 'Connected'} />{/snippet}
            {#snippet trailing()}
              <Switch checked={p.enabled} ariaLabel={p.enabled ? `Turn off ${p.name}` : `Turn on ${p.name}`} onchange={() => toggle(p)} />
              <IconButton icon="edit" label="Edit {p.name}" onclick={() => edit(p)} />
              <IconButton icon="trash" label="Remove {p.name}" tone="danger" onclick={() => remove(p)} />
            {/snippet}
          </ListRow>
        {/each}
      </List>
    </Section>
  {/if}

  <Section title="Add a provider">
    <List>
      {#each templates as t (t.name)}
        <ListRow title={t.name} subtitle={t.note}>
          {#snippet trailing()}<Button size="sm" icon="plus" onclick={() => addFrom(t)}>Add</Button>{/snippet}
        </ListRow>
      {/each}
    </List>
  </Section>
{/if}
