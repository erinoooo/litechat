<script lang="ts">
  type Props = {
    checked?: boolean
    /** Visible label. Without it, pass ariaLabel. */
    label?: string
    description?: string
    ariaLabel?: string
    onchange?: (checked: boolean) => void
  }
  let { checked = $bindable(false), label, description, ariaLabel, onchange }: Props = $props()

  function toggle() {
    checked = !checked
    onchange?.(checked)
  }
</script>

{#snippet track()}
  <span
    class="relative inline-flex h-5 w-8 shrink-0 items-center rounded-full transition-colors duration-150 {checked ? 'bg-pine' : 'bg-line-strong'}"
    aria-hidden="true"
  >
    <span class="size-4 rounded-full bg-canvas transition-transform duration-150 {checked ? 'translate-x-3.5' : 'translate-x-0.5'}"></span>
  </span>
{/snippet}

{#if label}
  <button type="button" role="switch" aria-checked={checked} onclick={toggle} class="flex w-full items-center justify-between gap-6 py-1 text-left">
    <span class="min-w-0">
      <span class="block text-sm font-medium text-ink">{label}</span>
      {#if description}<span class="block text-sm text-ink-2">{description}</span>{/if}
    </span>
    {@render track()}
  </button>
{:else}
  <button type="button" role="switch" aria-checked={checked} aria-label={ariaLabel} title={ariaLabel} onclick={toggle} class="inline-flex rounded-full">
    {@render track()}
  </button>
{/if}
