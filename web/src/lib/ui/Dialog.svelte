<script lang="ts">
  import type { Snippet } from 'svelte'
  import IconButton from './IconButton.svelte'

  type Props = {
    title: string
    description?: string
    width?: 'md' | 'wide'
    /** Fixed height (for dialogs whose content changes, like settings). */
    tall?: boolean
    onclose: () => void
    children: Snippet
    footer?: Snippet
    /** Replaces the default body padding/scroll container (e.g. settings with its own nav). */
    bare?: boolean
  }
  let { title, description, width = 'md', tall = false, onclose, children, footer, bare = false }: Props = $props()

  let panel = $state<HTMLDivElement>()
  let restore: HTMLElement | null = null

  $effect(() => {
    restore = document.activeElement as HTMLElement | null
    queueMicrotask(() => {
      const target = panel?.querySelector<HTMLElement>('[autofocus], input, textarea, select') ?? panel
      target?.focus()
    })
    return () => restore?.focus?.()
  })

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      e.stopPropagation()
      onclose()
    }
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="fixed inset-0 z-50 flex items-start justify-center p-3 pt-[8vh] sm:p-6 sm:pt-[10vh]" {onkeydown}>
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <div class="motion-scrim absolute inset-0 bg-scrim" onclick={onclose}></div>
  <div
    bind:this={panel}
    role="dialog"
    aria-modal="true"
    aria-label={title}
    tabindex="-1"
    class="motion-overlay relative flex max-h-[84vh] w-full flex-col overflow-hidden rounded-lg border border-line bg-raised shadow-overlay outline-none {width === 'wide' ? 'max-w-dialog-wide' : 'max-w-dialog'} {tall ? 'h-[84vh] sm:h-[600px]' : ''}"
  >
    {#if bare}
      {@render children()}
    {:else}
      <header class="flex items-start justify-between gap-4 px-5 pt-4 pb-3">
        <div class="min-w-0">
          <h2 class="text-lg font-semibold tracking-tight">{title}</h2>
          {#if description}<p class="mt-0.5 text-sm text-ink-2">{description}</p>{/if}
        </div>
        <div class="-mr-2"><IconButton icon="x" label="Close" onclick={onclose} /></div>
      </header>
      <div class="min-h-0 flex-1 overflow-y-auto px-5 pb-5">{@render children()}</div>
      {#if footer}
        <footer class="flex items-center justify-end gap-2 border-t border-line px-5 py-3">{@render footer()}</footer>
      {/if}
    {/if}
  </div>
</div>
