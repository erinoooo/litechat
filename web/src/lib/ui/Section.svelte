<script lang="ts">
  import type { Snippet } from 'svelte'
  /** A titled block inside a page. Sections are separated by space, not rules (lists bring their own hairlines). */
  let { title, description, actions, children }: { title: string; description?: string; actions?: Snippet; children: Snippet } = $props()
</script>

<section class="pt-8 first-of-type:pt-0">
  <div class="mb-4 flex items-start justify-between gap-4">
    <div class="min-w-0">
      <h3 class="text-sm font-semibold text-ink">{title}</h3>
      {#if description}<p class="mt-0.5 max-w-prose text-sm text-ink-2">{description}</p>{/if}
    </div>
    {#if actions}<div class="flex shrink-0 items-center gap-2">{@render actions()}</div>{/if}
  </div>
  {@render children()}
</section>
