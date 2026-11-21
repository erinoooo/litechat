<script lang="ts">
  import { onMount } from 'svelte'
  import { api, type MCPServer } from '../api'
  import { app } from '../state.svelte'
  import Icon from './Icon.svelte'

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
</script>

{#if form}
  <h3 class="mb-5 text-lg font-semibold">{form.id ? 'Edit' : 'Add'} {form.transport === 'http' ? 'remote' : 'local'} MCP server</h3>
  <div class="space-y-4">
    <div>
      <label class="label" for="m-name">Name</label>
      <input id="m-name" class="input" bind:value={form.name} placeholder="GitHub" />
    </div>
    {#if form.transport === 'http'}
      <div>
        <label class="label" for="m-url">Server URL</label>
        <input id="m-url" class="input font-mono text-[13px]" bind:value={form.url} placeholder="https://example.com/mcp" />
      </div>
      <div>
        <label class="label" for="m-h">Headers <span class="font-normal text-faint">(optional, one per line)</span></label>
        <textarea id="m-h" class="input font-mono text-[13px]" rows="3" bind:value={form.headers} placeholder="Authorization: Bearer …"></textarea>
      </div>
    {:else}
      {#if !app.stdioAllowed}
        <div class="rounded-xl border border-line bg-sunk px-3.5 py-3 text-sm text-muted">
          Local servers run commands on the machine hosting liteChat, so they're off by default.
          Start liteChat with <code class="font-mono text-fg">LITECHAT_MCP_STDIO=1</code> to allow them.
        </div>
      {/if}
      <div>
        <label class="label" for="m-cmd">Command</label>
        <input id="m-cmd" class="input font-mono text-[13px]" bind:value={form.command} placeholder="npx" />
      </div>
      <div>
        <label class="label" for="m-args">Arguments <span class="font-normal text-faint">(one per line)</span></label>
        <textarea id="m-args" class="input font-mono text-[13px]" rows="3" bind:value={form.args} placeholder={'-y\n@modelcontextprotocol/server-filesystem\n/home/me/notes'}></textarea>
      </div>
      <div>
        <label class="label" for="m-env">Environment <span class="font-normal text-faint">(optional, KEY=value per line)</span></label>
        <textarea id="m-env" class="input font-mono text-[13px]" rows="2" bind:value={form.env} placeholder="GITHUB_TOKEN=…"></textarea>
      </div>
    {/if}
    <p class="text-xs text-faint">Header and environment values are stored on the server and shown masked.</p>
  </div>
  <div class="mt-6 flex justify-end gap-2">
    <button class="btn-ghost" onclick={() => (form = null)}>Cancel</button>
    <button class="btn-primary" disabled={saving || !form.name || (form.transport === 'http' ? !form.url : !form.command)} onclick={save}>
      {saving ? 'Connecting…' : 'Save'}
    </button>
  </div>
{:else}
  <div class="mb-5 flex items-start justify-between gap-4 pr-8">
    <div>
      <h3 class="mb-1 text-lg font-semibold">Tools</h3>
      <p class="text-sm text-muted">Connect <a class="text-accent underline underline-offset-2" href="https://modelcontextprotocol.io" target="_blank" rel="noreferrer">MCP</a> servers to give models tools like web search, files or GitHub. Toggle them per message with the Tools button.</p>
    </div>
    <button class="btn-icon shrink-0" title="Reconnect all" aria-label="Reconnect all" onclick={() => refresh(true)}>
      <Icon name="refresh" size={16} class={loading ? 'animate-spin' : ''} />
    </button>
  </div>

  {#if app.mcpServers.length}
    <div class="mb-6 divide-y divide-line overflow-hidden rounded-xl border border-line">
      {#each app.mcpServers as s (s.id)}
        {@const st = app.mcpStatus[s.id]}
        <div class={s.enabled ? '' : 'opacity-55'}>
          <div class="flex items-center gap-3 px-4 py-3">
            <span class="h-2 w-2 shrink-0 rounded-full {!s.enabled ? 'bg-faint' : !st ? 'bg-line-strong animate-pulse' : st.error ? 'bg-danger' : 'bg-emerald-500'}"></span>
            <button class="min-w-0 flex-1 text-left" onclick={() => (expanded = expanded === s.id ? null : s.id)}>
              <div class="truncate text-sm font-medium">{s.name}</div>
              <div class="truncate text-xs text-faint">
                {#if st?.error}<span class="text-danger">{st.error}</span>
                {:else}{s.transport === 'http' ? s.url : [s.command, ...(s.args ?? [])].join(' ')}{st ? ` · ${st.tools.length} tool${st.tools.length === 1 ? '' : 's'}` : ''}{/if}
              </div>
            </button>
            <label class="relative inline-flex cursor-pointer items-center" title={s.enabled ? 'Disable' : 'Enable'}>
              <input type="checkbox" class="peer sr-only" checked={s.enabled} onchange={() => toggle(s)} />
              <span class="h-5 w-9 rounded-full bg-line-strong transition-colors peer-checked:bg-accent"></span>
              <span class="absolute left-0.5 top-0.5 h-4 w-4 rounded-full bg-white shadow transition-transform peer-checked:translate-x-4"></span>
            </label>
            <button class="btn-icon" onclick={() => edit(s)} aria-label="Edit"><Icon name="edit" size={16} /></button>
            <button class="btn-icon hover:!text-danger" onclick={() => remove(s)} aria-label="Delete"><Icon name="trash" size={16} /></button>
          </div>
          {#if expanded === s.id && st?.tools.length}
            <div class="space-y-1.5 border-t border-line bg-sunk px-4 py-3">
              {#each st.tools as t}
                <div class="text-xs"><span class="font-mono font-medium">{t.name}</span> <span class="text-muted">{t.description}</span></div>
              {/each}
            </div>
          {/if}
        </div>
      {/each}
    </div>
  {/if}

  <div class="label">Add an MCP server</div>
  <div class="grid gap-2 sm:grid-cols-2">
    <button class="rounded-xl border border-line px-3.5 py-3 text-left text-sm transition-colors hover:border-line-strong hover:bg-hover" onclick={() => add('http')}>
      <div class="font-medium">Remote (HTTP)</div>
      <div class="text-xs text-faint">A server URL, e.g. a hosted MCP endpoint</div>
    </button>
    <button class="rounded-xl border border-line px-3.5 py-3 text-left text-sm transition-colors hover:border-line-strong hover:bg-hover" onclick={() => add('stdio')}>
      <div class="font-medium">Local (command)</div>
      <div class="text-xs text-faint">{app.stdioAllowed ? 'Runs a process, e.g. npx or uvx' : 'Needs LITECHAT_MCP_STDIO=1'}</div>
    </button>
  </div>
{/if}
