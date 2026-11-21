<script lang="ts">
  import Icon from './Icon.svelte'
  import IconButton from './IconButton.svelte'
  import Spinner from './Spinner.svelte'

  /** A file or image attached to a message. */
  type Props = { name: string; detail?: string; thumb?: string; loading?: boolean; onremove?: () => void }
  let { name, detail, thumb, loading = false, onremove }: Props = $props()
</script>

<div class="flex h-12 max-w-64 items-center gap-2.5 rounded-md border border-line bg-canvas pr-1 {thumb ? 'pl-1' : 'pl-3'}">
  {#if thumb}
    <img src={thumb} alt="" class="size-10 rounded-sm object-cover" />
  {:else if loading}
    <Spinner size={14} label="Reading file" />
  {:else}
    <Icon name="file" size={16} class="text-ink-3" />
  {/if}
  <div class="min-w-0 flex-1">
    <div class="truncate text-sm text-ink">{name}</div>
    {#if detail}<div class="truncate text-xs text-ink-3">{detail}</div>{/if}
  </div>
  {#if onremove}<IconButton icon="x" label="Remove {name}" size="sm" onclick={onremove} />{/if}
</div>
