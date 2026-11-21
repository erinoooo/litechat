<script lang="ts">
  import type { Snippet } from 'svelte'
  import Icon from './Icon.svelte'

  type Props = {
    label: string
    icon?: string
    /** Secondary line under the label. */
    detail?: string
    /** Snippet for rich secondary content (e.g. highlighted search snippets). */
    detailHtml?: Snippet
    tone?: 'default' | 'danger'
    selected?: boolean
    /** Keyboard highlight. */
    active?: boolean
    onclick?: () => void
    onmousemove?: () => void
  }
  let { label, icon, detail, detailHtml, tone = 'default', selected = false, active = false, onclick, onmousemove }: Props = $props()
</script>

<button
  type="button"
  role="menuitem"
  data-active={active || undefined}
  class="flex w-full items-start gap-2.5 rounded-md px-2.5 py-1.5 text-left text-sm transition-colors duration-75 hover:bg-hover data-active:bg-hover {tone === 'danger' ? 'text-red' : 'text-ink'}"
  {onclick}
  {onmousemove}
>
  {#if icon}<Icon name={icon} size={16} class="mt-px {tone === 'danger' ? '' : 'text-ink-2'}" />{/if}
  <span class="min-w-0 flex-1">
    <span class="block truncate">{label}</span>
    {#if detailHtml}
      <span class="line-clamp-2 block text-xs text-ink-2">{@render detailHtml()}</span>
    {:else if detail}
      <span class="block truncate text-xs text-ink-3">{detail}</span>
    {/if}
  </span>
  {#if selected}<Icon name="check" size={16} class="mt-px text-pine" />{/if}
</button>
