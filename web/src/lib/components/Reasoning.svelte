<script lang="ts">
  import { Disclosure, Icon, Spinner } from '../ui'

  /** A model's reasoning, kept apart from the answer. While the model is
   *  thinking it is open and follows the newest text; once the answer
   *  starts it folds away, unless the reader opened or closed it. */
  let { blocks, live = false }: { blocks: { key: string; html: string }[]; live?: boolean } = $props()

  let touched = $state(false)
  let userOpen = $state(false)
  let body = $state<HTMLDivElement>()

  const open = $derived(touched ? userOpen : live)

  $effect(() => {
    void blocks
    if (live && open && body) body.scrollTop = body.scrollHeight
  })
</script>

<div class="mb-4">
  <Disclosure
    open={open}
    onToggle={(v) => {
      touched = true
      userOpen = v
    }}
  >
    {#snippet summary()}
      {#if live}
        <Spinner size={14} label="Thinking" />
      {:else}
        <Icon name="sparkle" size={14} class="shrink-0 text-ink-3" />
      {/if}
      <span class="flex-1 text-ink-2">{live ? 'Thinking' : 'Reasoning'}</span>
    {/snippet}
    <div bind:this={body} class="prose {live ? 'max-h-48' : 'max-h-96'} overflow-y-auto text-sm text-ink-2">
      {#each blocks as b (b.key)}
        <div class="contents">{@html b.html}</div>
      {/each}
    </div>
  </Disclosure>
</div>
