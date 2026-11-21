<script lang="ts">
  import { onDestroy } from 'svelte'
  import { renderBlocks, segments, onRendererChange } from '../markdown'
  import ToolCall from './ToolCall.svelte'

  let { content, streaming = false }: { content: string; streaming?: boolean } = $props()

  // re-render once lazy libraries (highlighter, KaTeX) arrive
  let tick = $state(0)
  const off = onRendererChange(() => tick++)
  onDestroy(off)

  const segs = $derived.by(() => {
    void tick
    return segments(content)
  })
  const rendered = $derived(segs.map((s) => ({ ...s, blocks: s.kind === 'tool' ? [] : renderBlocks(s.text) })))

  function onclick(e: MouseEvent) {
    const btn = (e.target as HTMLElement).closest<HTMLButtonElement>('[data-copy]')
    if (!btn) return
    const code = btn.closest('.code-block')?.querySelector('code')?.textContent ?? ''
    navigator.clipboard.writeText(code).then(() => {
      btn.textContent = 'Copied'
      setTimeout(() => (btn.textContent = 'Copy'), 1400)
    })
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div class="prose" class:caret={streaming && !segs.some((s) => s.open)} {onclick}>
  {#each rendered as seg, si (si)}
    {#if seg.kind === 'think'}
      <details class="think" class:live={seg.open && streaming} open={seg.open && streaming}>
        <summary>{seg.open && streaming ? 'Thinking…' : 'Thought process'}</summary>
        {#each seg.blocks as b (b.key)}
          <div class="contents">{@html b.html}</div>
        {/each}
      </details>
    {:else if seg.kind === 'tool'}
      <ToolCall tool={seg.tool!} />
    {:else}
      {#each seg.blocks as b (b.key)}
        <div class="contents">{@html b.html}</div>
      {/each}
    {/if}
  {/each}
</div>

<style>
  /* streaming caret: after the text of the last paragraph or list item */
  .caret > :global(.contents:last-child > p:last-child::after),
  .caret > :global(.contents:last-child > :is(ul, ol):last-child > li:last-child::after) {
    content: '';
    display: inline-block;
    width: 0.5em;
    height: 1.05em;
    margin-left: 3px;
    vertical-align: -0.15em;
    background: var(--accent);
    border-radius: 2px;
    animation: pulse 1s ease-in-out infinite;
  }
</style>
