<script lang="ts">
  import type { Snippet } from 'svelte'

  type Props = {
    label: string
    href?: string
    icon?: Snippet
    active?: boolean
    onselect?: () => void
    ondblclick?: () => void
    /** Hover/focus actions on the right (e.g. a Menu). */
    actions?: Snippet
    /** Keep actions visible (e.g. while their menu is open). */
    pinActions?: boolean
  }
  let { label, href, icon, active = false, onselect, ondblclick, actions, pinActions = false }: Props = $props()

  function onclick(e: MouseEvent) {
    if (href && (e.metaKey || e.ctrlKey || e.shiftKey)) return // let the browser open a new tab
    e.preventDefault()
    onselect?.()
  }
</script>

<div class="group relative">
  <a
    href={href ?? '#'}
    {onclick}
    {ondblclick}
    aria-current={active ? 'page' : undefined}
    class="flex h-8 items-center gap-2 rounded-md px-2.5 text-sm transition-colors duration-75 {actions ? 'pr-9' : ''} {active ? 'bg-hover font-medium text-ink' : 'text-ink-2 hover:bg-hover hover:text-ink'}"
  >
    {#if icon}{@render icon()}{/if}
    <span class="truncate">{label}</span>
  </a>
  {#if actions}
    <div class="absolute top-0 right-0.5 flex h-8 items-center {pinActions || active ? '' : 'opacity-0 group-focus-within:opacity-100 group-hover:opacity-100 max-md:opacity-100'}">
      {@render actions()}
    </div>
  {/if}
</div>
