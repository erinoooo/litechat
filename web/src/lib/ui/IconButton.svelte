<script lang="ts">
  import Icon from './Icon.svelte'

  type Props = {
    icon: string
    /** Required: used for the tooltip and screen readers. */
    label: string
    size?: 'sm' | 'md'
    /** 'primary' is the filled pine button (send). */
    tone?: 'default' | 'danger' | 'pine' | 'primary'
    active?: boolean
    disabled?: boolean
    spin?: boolean
    onclick?: (e: MouseEvent) => void
    /** For use as a menu trigger. */
    expanded?: boolean
  }
  let { icon, label, size = 'md', tone = 'default', active = false, disabled = false, spin = false, onclick, expanded }: Props = $props()

  const tones = {
    default: 'text-ink-2 hover:text-ink',
    danger: 'text-ink-2 hover:text-red',
    pine: 'text-pine',
    primary: 'bg-pine text-pine-ink hover:!bg-pine/88',
  }
</script>

<button
  type="button"
  class="grid shrink-0 place-items-center rounded-md transition-colors duration-100 hover:bg-hover disabled:pointer-events-none disabled:opacity-35 {size === 'sm' ? 'size-7' : 'size-8'} {tones[tone]} {active ? 'bg-hover text-ink' : ''}"
  aria-label={label}
  title={label}
  aria-expanded={expanded}
  {disabled}
  {onclick}
>
  <Icon name={icon} size={size === 'sm' ? 14 : 16} class={spin ? 'motion-spin' : ''} />
</button>
