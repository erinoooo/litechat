<script lang="ts">
  import type { ToolBlock } from '../markdown'
  import Icon from './Icon.svelte'

  let { tool, running = false }: { tool: ToolBlock | { name: string; server: string; args?: unknown }; running?: boolean } = $props()

  let open = $state(false)
  const done = $derived('status' in tool ? tool : null)
  const failed = $derived(done?.status === 'error')
  const argText = $derived.by(() => {
    const a = tool.args
    if (!a || (typeof a === 'object' && !Object.keys(a).length)) return ''
    return JSON.stringify(a, null, 2)
  })
  // one-line summary of the arguments for the collapsed header
  const argSummary = $derived.by(() => {
    const a = tool.args as Record<string, unknown> | undefined
    if (!a || typeof a !== 'object') return ''
    const vals = Object.values(a).filter((v) => typeof v === 'string' || typeof v === 'number')
    return vals.length ? String(vals[0]).slice(0, 80) : ''
  })
</script>

<div class="my-3 overflow-hidden rounded-xl border text-sm {failed ? 'border-danger/40' : 'border-line'}">
  <button
    class="flex w-full items-center gap-2.5 px-3 py-2 text-left transition-colors hover:bg-hover disabled:hover:bg-transparent"
    onclick={() => (open = !open)}
    disabled={running}
  >
    <span class="grid h-6 w-6 shrink-0 place-items-center rounded-md {failed ? 'bg-danger/10 text-danger' : 'bg-accent-soft text-accent'}">
      {#if running}
        <span class="h-3 w-3 animate-spin rounded-full border-2 border-current border-t-transparent"></span>
      {:else}
        <Icon name={failed ? 'x' : 'tool'} size={14} stroke={2} />
      {/if}
    </span>
    <span class="min-w-0 flex-1 truncate">
      <span class="font-medium">{running ? 'Running' : failed ? 'Failed' : 'Used'} {tool.name}</span>
      {#if argSummary}<span class="text-muted"> · {argSummary}</span>{/if}
      {#if tool.server}<span class="text-faint"> · {tool.server}</span>{/if}
    </span>
    {#if !running}<Icon name="chevronDown" size={15} class="text-faint transition-transform {open ? 'rotate-180' : ''}" />{/if}
  </button>
  {#if open && done}
    <div class="space-y-2 border-t border-line bg-sunk px-3 py-2.5">
      {#if argText}
        <div>
          <div class="mb-1 text-[11px] font-semibold uppercase tracking-wider text-faint">Input</div>
          <pre class="max-h-48 overflow-auto whitespace-pre-wrap break-words font-mono text-xs">{argText}</pre>
        </div>
      {/if}
      <div>
        <div class="mb-1 text-[11px] font-semibold uppercase tracking-wider text-faint">{failed ? 'Error' : 'Result'}</div>
        <pre class="max-h-72 overflow-auto whitespace-pre-wrap break-words font-mono text-xs {failed ? 'text-danger' : ''}">{done.result || '(empty)'}</pre>
      </div>
    </div>
  {/if}
</div>
