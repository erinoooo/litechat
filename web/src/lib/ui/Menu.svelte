<script lang="ts">
  import type { Snippet } from 'svelte'

  type Props = {
    open?: boolean
    align?: 'start' | 'end'
    /** 'auto' sizes to content (min 176px); 'wide' is the popover token width. */
    width?: 'auto' | 'wide'
    trigger: Snippet<[{ toggle: () => void; open: boolean }]>
    children: Snippet
    /** Rendered above the scrollable list (e.g. a search field). */
    header?: Snippet
    footer?: Snippet
    onopen?: () => void
    onclose?: () => void
  }
  let { open = $bindable(false), align = 'start', width = 'auto', trigger, children, header, footer, onopen, onclose }: Props = $props()

  let root = $state<HTMLDivElement>()

  function close() {
    open = false
    onclose?.()
  }

  function toggle() {
    open = !open
    if (open) onopen?.()
  }

  $effect(() => {
    if (!open) return
    const onDown = (e: PointerEvent) => {
      if (root && !root.contains(e.target as Node)) close()
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.stopPropagation()
        close()
      }
    }
    document.addEventListener('pointerdown', onDown, true)
    document.addEventListener('keydown', onKey, true)
    return () => {
      document.removeEventListener('pointerdown', onDown, true)
      document.removeEventListener('keydown', onKey, true)
    }
  })
</script>

<div class="relative min-w-0" bind:this={root}>
  {@render trigger({ toggle, open })}
  {#if open}
    <div
      role="menu"
      tabindex="-1"
      class="motion-overlay absolute top-full z-40 mt-1 flex max-h-[min(70vh,480px)] flex-col overflow-hidden rounded-lg border border-line bg-raised shadow-overlay {align === 'end' ? 'right-0' : 'left-0'} {width === 'wide' ? 'w-[min(92vw,var(--spacing-popover))]' : 'min-w-44'}"
    >
      {#if header}<div class="border-b border-line p-1">{@render header()}</div>{/if}
      <div class="min-h-0 overflow-y-auto p-1">{@render children()}</div>
      {#if footer}<div class="border-t border-line p-1">{@render footer()}</div>{/if}
    </div>
  {/if}
</div>
