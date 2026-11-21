<script lang="ts">
  import type { Snippet } from 'svelte'
  /** Title block at the top of a settings page or full view. */
  let { title, description, actions }: { title: string; description?: string; actions?: Snippet } = $props()
</script>

<header class="mb-6 flex items-start justify-between gap-4">
  <div class="min-w-0">
    <h2 class="text-lg font-semibold tracking-tight text-ink">{title}</h2>
    {#if description}<p class="mt-0.5 max-w-prose text-sm text-ink-2">{description}</p>{/if}
  </div>
  {#if actions}<div class="flex shrink-0 items-center gap-1">{@render actions()}</div>{/if}
</header>
