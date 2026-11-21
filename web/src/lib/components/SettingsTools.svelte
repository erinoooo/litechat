<script lang="ts">
  import { onMount } from 'svelte'
  import { api, type MCPServer } from '../api'
  import { app } from '../state.svelte'
  import { Button, IconButton, List, ListRow, Notice, PageHeader, Section, StatusDot, Switch, TextArea, TextField } from '../ui'

  type Form = {
    id?: string
    name: string
    transport: 'http' | 'stdio'
    url: string
    headers: string // "Key: value" lines
    command: string
    args: string // one per line
    env: string // KEY=value lines
  }

  let form = $state<Form | null>(null)
  let saving = $state(false)
  let loading = $state(false)
  let expanded = $state<string | null>(null)

  onMount(() => refresh(false))

  async function refresh(force: boolean) {
    loading = true
    try {
      await app.loadMCP(force)
    } catch (e) {
      app.notify((e as Error).message, 'error')
    } finally {
      loading = false
    }
  }

  const toLines = (m: Record<string, string> | null, sep: string) =>
    Object.entries(m ?? {})
      .map(([k, v]) => `${k}${sep}${v}`)
      .join('\n')

  function fromLines(s: string, sep: string) {
    const out: Record<string, string> = {}
    for (const line of s.split('\n')) {
      const i = line.indexOf(sep)
      if (i > 0) out[line.slice(0, i).trim()] = line.slice(i + sep.length).trim()
    }
    return out
  }

  function add(transport: 'http' | 'stdio') {
    form = { name: '', transport, url: '', headers: '', command: '', args: '', env: '' }
  }

  function edit(s: MCPServer) {
    form = {
      id: s.id,
      name: s.name,
      transport: s.transport,
      url: s.url,
      headers: toLines(s.headers, ': '),
      command: s.command,
      args: (s.args ?? []).join('\n'),
      env: toLines(s.env, '='),
    }
  }

  async function save() {
    if (!form) return
    saving = true
    try {
      const existing = app.mcpServers.find((s) => s.id === form!.id)
      await api.saveMCP(
        {
          name: form.name,
          transport: form.transport,
          url: form.url,
          headers: fromLines(form.headers, ':'),
          command: form.command,
          args: form.args.split('\n').map((a) => a.trim()).filter(Boolean),
          env: fromLines(form.env, '='),
          enabled: existing?.enabled ?? true,
        },
        form.id,
      )
      form = null
      await refresh(true)
      app.notify('Server saved')
    } catch (e) {
      app.notify((e as Error).message, 'error')
    } finally {
      saving = false
    }
  }

  async function toggle(s: MCPServer) {
    await api.saveMCP({ ...s, enabled: !s.enabled }, s.id)
    await refresh(false)
  }

  async function remove(s: MCPServer) {
    if (!confirm(`Remove ${s.name}?`)) return
    await api.deleteMCP(s.id)
    await refresh(false)
  }

  function where(s: MCPServer) {
    const st = app.mcpStatus[s.id]
    const loc = s.transport === 'http' ? s.url : [s.command, ...(s.args ?? [])].join(' ')
    return st ? `${loc}, ${st.tools.length} tool${st.tools.length === 1 ? '' : 's'}` : loc
  }
</script>

{#if form}
  <PageHeader title={form.id ? `Edit ${form.name}` : form.transport === 'http' ? 'Add a remote server' : 'Add a local server'} />
  <div class="flex max-w-md flex-col gap-4">
    {#if form.transport === 'stdio' && !app.stdioAllowed}
      <Notice>Local servers run commands on the machine hosting liteChat, so they're off by default. Restart liteChat with LITECHAT_MCP_STDIO=1 to allow them.</Notice>
    {/if}
    <TextField label="Name" bind:value={form.name} placeholder="GitHub" />
    {#if form.transport === 'http'}
      <TextField label="Server URL" bind:value={form.url} mono placeholder="https://example.com/mcp" />
      <TextArea label="Headers" bind:value={form.headers} rows={3} mono placeholder="Authorization: Bearer …" hint="Optional. One per line." />
    {:else}
      <TextField label="Command" bind:value={form.command} mono placeholder="npx" />
      <TextArea label="Arguments" bind:value={form.args} rows={3} mono placeholder={'-y\n@modelcontextprotocol/server-filesystem\n/home/me/notes'} hint="One per line." />
      <TextArea label="Environment variables" bind:value={form.env} rows={2} mono placeholder="GITHUB_TOKEN=…" hint="Optional. KEY=value, one per line." />
    {/if}
    <p class="text-xs text-ink-3">Header and variable values stay on the server and show masked here.</p>
    <div class="mt-2 flex gap-2">
      <Button variant="primary" disabled={saving || !form.name || (form.transport === 'http' ? !form.url : !form.command)} onclick={save}>
        {saving ? 'Connecting' : 'Save server'}
      </Button>
      <Button onclick={() => (form = null)}>Cancel</Button>
    </div>
  </div>
{:else}
  <PageHeader title="Tools" description="Connect MCP servers to let models search the web, read files, use GitHub and more. Turn tools on or off from the message box.">
    {#snippet actions()}<div class="mr-10"><IconButton icon="refresh" label="Reconnect all" spin={loading} onclick={() => refresh(true)} /></div>{/snippet}
  </PageHeader>

  {#if app.mcpServers.length}
    <Section title="Servers">
      <List>
        {#each app.mcpServers as s (s.id)}
          {@const st = app.mcpStatus[s.id]}
          <ListRow title={s.name} subtitle={where(s)} error={s.enabled ? st?.error : undefined} dim={!s.enabled}
            onclick={st?.tools.length ? () => (expanded = expanded === s.id ? null : s.id) : undefined}>
            {#snippet leading()}
              <StatusDot status={!s.enabled ? 'off' : !st ? 'pending' : st.error ? 'error' : 'ok'} label={!s.enabled ? 'Off' : !st ? 'Connecting' : st.error ? 'Not connected' : 'Connected'} />
            {/snippet}
            {#snippet trailing()}
              <Switch checked={s.enabled} ariaLabel={s.enabled ? `Turn off ${s.name}` : `Turn on ${s.name}`} onchange={() => toggle(s)} />
              <IconButton icon="edit" label="Edit {s.name}" onclick={() => edit(s)} />
              <IconButton icon="trash" label="Remove {s.name}" tone="danger" onclick={() => remove(s)} />
            {/snippet}
            {#if expanded === s.id && st?.tools.length}
              <ul class="ml-5 flex flex-col gap-1.5 border-l border-line pl-3">
                {#each st.tools as t}
                  <li class="text-sm"><code class="font-mono text-ink">{t.name}</code> <span class="text-ink-2">{t.description}</span></li>
                {/each}
              </ul>
            {/if}
          </ListRow>
        {/each}
      </List>
    </Section>
  {/if}

  <Section title="Add a server">
    <List>
      <ListRow title="Remote server" subtitle="Connect by URL, like a hosted MCP endpoint">
        {#snippet trailing()}<Button size="sm" icon="plus" onclick={() => add('http')}>Add</Button>{/snippet}
      </ListRow>
      <ListRow title="Local server" subtitle={app.stdioAllowed ? 'Run a command on this machine, like npx or uvx' : 'Needs LITECHAT_MCP_STDIO=1 on the server'}>
        {#snippet trailing()}<Button size="sm" icon="plus" onclick={() => add('stdio')}>Add</Button>{/snippet}
      </ListRow>
    </List>
  </Section>
{/if}
