<script lang="ts">
  import type { Snippet } from 'svelte'
  let { id, label, hint, error, children }: { id: string; label?: string; hint?: string; error?: string; children: Snippet } = $props()
</script>

<div class="flex min-w-0 flex-col gap-1.5">
  {#if label}<label for={id} class="text-sm font-medium text-ink">{label}</label>{/if}
  {@render children()}
  {#if error}
    <p class="text-xs text-red">{error}</p>
  {:else if hint}
    <p class="text-xs text-ink-3">{hint}</p>
  {/if}
</div>
