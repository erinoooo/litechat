<script lang="ts">
  import type { Snippet } from 'svelte'
  import Icon from './Icon.svelte'

  /** A bordered row that expands to show details (tool calls, logs…). */
  type Props = {
    open?: boolean
    tone?: 'default' | 'error'
    /** No chevron and not clickable (e.g. while still running). */
    disabled?: boolean
    /** Called when the reader opens or closes it. */
    onToggle?: (open: boolean) => void
    summary: Snippet
    children?: Snippet
  }
  let { open = $bindable(false), tone = 'default', disabled = false, onToggle, summary, children }: Props = $props()
</script>

<div class="rounded-md border {tone === 'error' ? 'border-red/40' : 'border-line'}">
  <button
    type="button"
    class="flex h-9 w-full items-center gap-2.5 rounded-md px-3 text-left text-sm transition-colors hover:bg-hover disabled:hover:bg-transparent"
    onclick={() => {
      open = !open
      onToggle?.(open)
    }}
    {disabled}
    aria-expanded={open}
  >
    {@render summary()}
    {#if !disabled}<Icon name="chevronDown" size={14} class="shrink-0 text-ink-3 transition-transform {open ? 'rotate-180' : ''}" />{/if}
  </button>
  {#if open && children}
    <div class="flex flex-col gap-3 border-t border-line bg-panel px-3 py-3 text-sm">{@render children()}</div>
  {/if}
</div>
