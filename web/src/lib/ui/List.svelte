<script lang="ts">
  import type { Snippet } from 'svelte'
  /** A run of ListRows separated by hairlines. Not a card: no fill, no radius, no shadow. */
  let { children, empty }: { children: Snippet; empty?: string } = $props()
</script>

<div class="divide-y divide-line border-y border-line">
  {@render children()}
</div>
{#if empty}<p class="py-6 text-sm text-ink-3">{empty}</p>{/if}
