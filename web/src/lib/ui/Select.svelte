<script lang="ts">
  import Field from './Field.svelte'
  import Icon from './Icon.svelte'

  type Option = { value: string; label: string }
  type Props = {
    value?: string
    options: Option[]
    label?: string
    hint?: string
    ariaLabel?: string
    onchange?: (value: string) => void
  }
  let { value = $bindable(''), options, label, hint, ariaLabel, onchange }: Props = $props()
  const id = 'f' + Math.random().toString(36).slice(2, 8)
</script>

<Field {id} {label} {hint}>
  <div class="relative">
    <select
      {id}
      aria-label={ariaLabel}
      bind:value
      onchange={() => onchange?.(value)}
      class="h-9 w-full appearance-none truncate rounded-md border border-line-strong bg-canvas pr-8 pl-3 text-sm text-ink outline-none focus:border-pine focus:shadow-[0_0_0_3px_var(--c-pine-wash)]"
    >
      {#each options as o (o.value)}<option value={o.value}>{o.label}</option>{/each}
    </select>
    <Icon name="chevronDown" size={14} class="pointer-events-none absolute top-1/2 right-3 -translate-y-1/2 text-ink-3" />
  </div>
</Field>
