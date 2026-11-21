<script lang="ts">
  import Icon from './Icon.svelte'
  /** Borderless search input for the top of menus and the command palette. */
  type Props = {
    value?: string
    placeholder: string
    size?: 'sm' | 'lg'
    autofocus?: boolean
    onkeydown?: (e: KeyboardEvent) => void
    oninput?: () => void
  }
  let { value = $bindable(''), placeholder, size = 'sm', autofocus = false, onkeydown, oninput }: Props = $props()
</script>

<div class="flex items-center gap-2.5 {size === 'lg' ? 'h-14 px-4' : 'h-9 px-2.5'}">
  <Icon name="search" size={size === 'lg' ? 18 : 16} class="text-ink-3" />
  <!-- svelte-ignore a11y_autofocus -->
  <input
    bind:value
    {placeholder}
    {autofocus}
    {onkeydown}
    {oninput}
    aria-label={placeholder}
    class="w-full bg-transparent text-ink outline-none placeholder:text-ink-3 {size === 'lg' ? 'text-base' : 'text-sm'}"
  />
</div>
