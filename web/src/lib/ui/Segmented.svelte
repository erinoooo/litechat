<script lang="ts">
  import Icon from './Icon.svelte'
  type Option = { value: string; label: string; icon?: string }
  let { value = $bindable(''), options, ariaLabel, onchange }: { value?: string; options: Option[]; ariaLabel: string; onchange?: (v: string) => void } = $props()
</script>

<div role="radiogroup" aria-label={ariaLabel} class="inline-flex rounded-md border border-line-strong p-0.5">
  {#each options as o (o.value)}
    <button
      type="button"
      role="radio"
      aria-checked={value === o.value}
      class="flex h-7 items-center gap-1.5 rounded-sm px-2.5 text-sm transition-colors {value === o.value ? 'bg-hover font-medium text-ink' : 'text-ink-2 hover:text-ink'}"
      onclick={() => {
        value = o.value
        onchange?.(o.value)
      }}
    >
      {#if o.icon}<Icon name={o.icon} size={14} />{/if}{o.label}
    </button>
  {/each}
</div>
