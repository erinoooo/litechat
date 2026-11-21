<script lang="ts">
  import type { Snippet } from 'svelte'

  type Props = {
    title: string
    subtitle?: string
    error?: string
    dim?: boolean
    leading?: Snippet
    trailing?: Snippet
    /** Makes the text area clickable (e.g. to expand details). */
    onclick?: () => void
    /** Content shown under the row (e.g. expanded details). */
    children?: Snippet
  }
  let { title, subtitle, error, dim = false, leading, trailing, onclick, children }: Props = $props()
</script>

{#snippet text()}
  <span class="block truncate text-sm font-medium text-ink">{title}</span>
  {#if error}
    <span class="block truncate text-xs text-red">{error}</span>
  {:else if subtitle}
    <span class="block truncate text-xs text-ink-3">{subtitle}</span>
  {/if}
{/snippet}

<div class="py-2.5 {dim ? 'opacity-55' : ''}">
  <div class="flex min-h-9 items-center gap-3">
    {#if leading}{@render leading()}{/if}
    {#if onclick}
      <button type="button" class="min-w-0 flex-1 text-left" {onclick}>{@render text()}</button>
    {:else}
      <div class="min-w-0 flex-1">{@render text()}</div>
    {/if}
    {#if trailing}<div class="flex shrink-0 items-center gap-1">{@render trailing()}</div>{/if}
  </div>
  {#if children}<div class="mt-2">{@render children()}</div>{/if}
</div>
