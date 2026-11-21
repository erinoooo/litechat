<script lang="ts">
  import Button from './Button.svelte'
  import IconButton from './IconButton.svelte'

  type Props = {
    label: string
    icon?: string
    accept?: string
    multiple?: boolean
    /** 'icon' renders an IconButton, 'button' a secondary Button. */
    as?: 'icon' | 'button'
    disabled?: boolean
    onfiles: (files: File[]) => void
  }
  let { label, icon = 'paperclip', accept, multiple = false, as = 'icon', disabled = false, onfiles }: Props = $props()
  let input = $state<HTMLInputElement>()
</script>

{#if as === 'icon'}
  <IconButton {icon} {label} {disabled} onclick={() => input?.click()} />
{:else}
  <Button {icon} {disabled} onclick={() => input?.click()}>{label}</Button>
{/if}
<input
  bind:this={input}
  type="file"
  class="hidden"
  {accept}
  {multiple}
  onchange={(e) => {
    const t = e.currentTarget
    if (t.files?.length) onfiles(Array.from(t.files))
    t.value = ''
  }}
/>
