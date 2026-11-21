<script lang="ts">
  import type { ToolBlock } from '../markdown'
  import { Disclosure, Icon, Spinner } from '../ui'

  let { tool, running = false }: { tool: ToolBlock | { name: string; server: string; args?: unknown }; running?: boolean } = $props()

  const done = $derived('status' in tool ? tool : null)
  const failed = $derived(done?.status === 'error')
  const argText = $derived.by(() => {
    const a = tool.args
    if (!a || (typeof a === 'object' && !Object.keys(a).length)) return ''
    return JSON.stringify(a, null, 2)
  })
  // Shown like a call: search("weather in Ankara")
  const call = $derived.by(() => {
    const a = tool.args as Record<string, unknown> | undefined
    const first = a && typeof a === 'object' ? Object.values(a).find((v) => typeof v === 'string' || typeof v === 'number') : undefined
    const s = first === undefined ? '' : JSON.stringify(first)
    return `${tool.name}(${s.length > 60 ? s.slice(0, 59) + '…"' : s})`
  })
</script>

<div class="my-3">
  <Disclosure tone={failed ? 'error' : 'default'} disabled={running || !done}>
    {#snippet summary()}
      {#if running}
        <Spinner size={14} label="Running tool" />
      {:else}
        <Icon name={failed ? 'x' : 'tool'} size={14} class={failed ? 'text-red' : 'text-pine'} />
      {/if}
      <code class="min-w-0 flex-1 truncate font-mono text-sm {failed ? 'text-red' : 'text-ink'}">{call}</code>
      <span class="shrink-0 text-xs text-ink-3 max-sm:hidden">{running ? 'Running' : failed ? 'Failed' : tool.server}</span>
    {/snippet}
    {#if done}
      {#if argText}
        <div>
          <p class="mb-1 text-xs font-medium text-ink-3">Input</p>
          <pre class="max-h-48 overflow-auto font-mono text-xs whitespace-pre-wrap break-words text-ink-2">{argText}</pre>
        </div>
      {/if}
      <div>
        <p class="mb-1 text-xs font-medium text-ink-3">{failed ? 'Error' : `Result from ${done.server}`}</p>
        <pre class="max-h-72 overflow-auto font-mono text-xs whitespace-pre-wrap break-words {failed ? 'text-red' : 'text-ink-2'}">{done.result || 'No output'}</pre>
      </div>
    {/if}
  </Disclosure>
</div>
