<script lang="ts">
  import type { Snippet } from 'svelte'
  import Icon from './Icon.svelte'
  /** A labelled group inside a Menu or a list. Sentence-case label, never all caps. */
  let { label, icon, children }: { label: string; icon?: string; children: Snippet } = $props()
</script>

<div role="group" aria-label={label} class="not-first:mt-1">
  <div class="flex h-7 items-center gap-1.5 px-2.5 text-xs font-medium text-ink-3">
    {#if icon}<Icon name={icon} size={12} />{/if}{label}
  </div>
  {@render children()}
</div>
