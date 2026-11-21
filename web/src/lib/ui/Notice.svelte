<script lang="ts">
  import type { Snippet } from 'svelte'
  /** Inline message: what happened and what to do. Errors don't apologize. */
  let { tone = 'neutral', children, action }: { tone?: 'neutral' | 'error'; children: Snippet; action?: Snippet } = $props()
</script>

<div role={tone === 'error' ? 'alert' : 'note'} class="flex items-start gap-3 rounded-md px-3 py-2 text-sm {tone === 'error' ? 'bg-red-wash text-red' : 'bg-panel text-ink-2'}">
  <div class="min-w-0 flex-1 break-words">{@render children()}</div>
  {#if action}<div class="shrink-0">{@render action()}</div>{/if}
</div>
