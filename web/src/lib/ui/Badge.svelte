<script lang="ts">
  import type { Snippet } from 'svelte'
  let { tone = 'neutral', children }: { tone?: 'neutral' | 'pine'; children: Snippet } = $props()
</script>

<span class="inline-flex h-5 items-center rounded-sm px-1.5 text-xs font-medium {tone === 'pine' ? 'bg-pine-wash text-pine' : 'bg-panel text-ink-2'}">
  {@render children()}
</span>
