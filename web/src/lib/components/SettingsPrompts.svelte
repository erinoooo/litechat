<script lang="ts">
  import { api, type Preset } from '../api'
  import { app } from '../state.svelte'
  import { Button, IconButton, List, ListRow, PageHeader, TextArea, TextField } from '../ui'

  let form = $state<Partial<Preset> | null>(null)

  async function save() {
    if (!form?.name?.trim()) return
    try {
      await api.savePreset({ name: form.name, prompt: form.prompt ?? '' }, form.id)
      app.presets = await api.presets()
      form = null
      app.notify('Prompt saved')
    } catch (e) {
      app.notify((e as Error).message, 'error')
    }
  }

  async function remove(p: Preset) {
    if (!confirm(`Delete the prompt “${p.name}”?`)) return
    await api.deletePreset(p.id)
    app.presets = await api.presets()
  }
</script>

{#if form}
  <PageHeader title={form.id ? `Edit ${form.name}` : 'New prompt'} />
  <div class="flex max-w-xl flex-col gap-4">
    <TextField label="Name" bind:value={form.name} placeholder="Code reviewer" />
    <TextArea label="System prompt" bind:value={form.prompt} rows={10} placeholder="Review the code for bugs first, style second. Quote the lines you mean." />
    <div class="mt-2 flex gap-2">
      <Button variant="primary" disabled={!form.name?.trim()} onclick={save}>Save prompt</Button>
      <Button onclick={() => (form = null)}>Cancel</Button>
    </div>
  </div>
{:else}
  <PageHeader title="Prompts" description="Saved system prompts. Pick one when you start a new chat.">
    {#snippet actions()}<div class="mr-10"><Button icon="plus" onclick={() => (form = { name: '', prompt: '' })}>New prompt</Button></div>{/snippet}
  </PageHeader>
  {#if app.presets.length}
    <List>
      {#each app.presets as p (p.id)}
        <ListRow title={p.name} subtitle={p.prompt}>
          {#snippet trailing()}
            <IconButton icon="edit" label="Edit {p.name}" onclick={() => (form = { ...p })} />
            <IconButton icon="trash" label="Delete {p.name}" tone="danger" onclick={() => remove(p)} />
          {/snippet}
        </ListRow>
      {/each}
    </List>
  {:else}
    <p class="text-sm text-ink-3">No saved prompts yet.</p>
  {/if}
{/if}
