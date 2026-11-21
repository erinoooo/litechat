<script lang="ts">
  import type { Snippet } from 'svelte'
  import Icon from './Icon.svelte'

  type Props = {
    /** 'title' is a large text trigger, e.g. the model name in the header. */
    variant?: 'primary' | 'secondary' | 'ghost' | 'danger' | 'title'
    size?: 'sm' | 'md'
    icon?: string
    type?: 'button' | 'submit'
    href?: string
    download?: boolean
    disabled?: boolean
    full?: boolean
    /** Toggle buttons: shows the on state. */
    pressed?: boolean
    /** Adds a chevron and menu semantics. Pass the menu's open state. */
    menu?: boolean
    expanded?: boolean
    title?: string
    onclick?: (e: MouseEvent) => void
    children: Snippet
  }
  let { variant = 'secondary', size = 'md', icon, type = 'button', href, download, disabled = false, full = false, pressed, menu = false, expanded, title, onclick, children }: Props = $props()

  const base =
    'inline-flex shrink-0 items-center justify-center gap-2 rounded-md text-sm font-medium whitespace-nowrap select-none transition-colors duration-100 disabled:opacity-45 disabled:pointer-events-none'
  const sizes = { sm: 'h-7 px-2.5', md: 'h-8 px-3' }
  const variants = {
    primary: 'bg-pine text-pine-ink hover:bg-pine/88',
    secondary: 'border border-line-strong bg-canvas text-ink hover:bg-hover',
    ghost: 'text-ink-2 hover:bg-hover hover:text-ink',
    danger: 'border border-line-strong bg-canvas text-red hover:bg-red-wash hover:border-red/40',
    title: 'min-w-0 !shrink max-w-full !text-base !font-semibold tracking-tight text-ink hover:bg-hover !gap-1.5',
  }
  const cls = $derived(`${base} ${sizes[size]} ${pressed ? 'bg-pine-wash text-pine hover:bg-pine-wash' : variants[variant]} ${full ? 'w-full' : ''}`)
</script>

{#if href}
  <a class={cls} {href} {download} {title}>
    {#if icon}<Icon name={icon} size={16} />{/if}{@render children()}
  </a>
{:else}
  <button class={cls} {type} {disabled} {title} {onclick} aria-pressed={pressed} aria-haspopup={menu ? 'menu' : undefined} aria-expanded={menu ? !!expanded : undefined}>
    {#if icon}<Icon name={icon} size={16} />{/if}{@render children()}
    {#if menu}<Icon name="chevronDown" size={14} class="shrink-0 text-ink-3" />{/if}
  </button>
{/if}
