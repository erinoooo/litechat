<script lang="ts">
  import type { Message } from '../api'
  import { app, type ChatState } from '../state.svelte'
  import { plainText } from '../markdown'
  import Markdown from './Markdown.svelte'
  import ToolCall from './ToolCall.svelte'
  import { Attachment, AutoTextarea, Button, IconButton, Notice, Well } from '../ui'

  let { msg, cs, last, first }: { msg: Message; cs: ChatState; last: boolean; first: boolean } = $props()

  const streaming = $derived(cs.streamingId === msg.id)
  const siblings = $derived(cs.children(msg.parent_id))
  const sibIndex = $derived(siblings.findIndex((m) => m.id === msg.id))
  const busy = $derived(!!cs.streamingId)

  // split attached files (sent as <file name="…">…</file>) from the typed text
  const parsed = $derived.by(() => {
    const files: { name: string; size: number }[] = []
    const text = msg.content
      .replace(/<file name="([^"]*)">\n?([\s\S]*?)\n?<\/file>\s*/g, (_, name, body) => {
        files.push({ name, size: body.length })
        return ''
      })
      .trim()
    return { files, text }
  })

  let editing = $state(false)
  let draft = $state('')
  let copied = $state(false)
  let editor = $state<AutoTextarea>()

  function startEdit() {
    draft = msg.content
    editing = true
    queueMicrotask(() => editor?.setCursorEnd())
  }

  function submitEdit() {
    editing = false
    if (draft.trim() && draft !== msg.content) app.edit(msg, draft)
  }

  function copy() {
    navigator.clipboard.writeText(msg.role === 'user' ? msg.content : plainText(msg.content))
    copied = true
    setTimeout(() => (copied = false), 1400)
  }

  const kb = (n: number) => `${(n / 1024).toFixed(1)} KB of text`
</script>

{#snippet branchNav()}
  {#if siblings.length > 1}
    <div class="flex items-center text-xs text-ink-3 tabular-nums">
      <IconButton icon="chevronLeft" label="Previous version" size="sm" disabled={sibIndex === 0 || busy} onclick={() => app.switchBranch(msg, -1)} />
      <span aria-label="Version {sibIndex + 1} of {siblings.length}">{sibIndex + 1} of {siblings.length}</span>
      <IconButton icon="chevronRight" label="Next version" size="sm" disabled={sibIndex === siblings.length - 1 || busy} onclick={() => app.switchBranch(msg, 1)} />
    </div>
  {/if}
{/snippet}

{#if msg.role === 'user'}
  <!-- The prompt reads as the heading of its turn: pine rule, medium weight. -->
  <div class="group {first ? '' : 'mt-6'}">
    {#if editing}
      <Well>
        <div class="px-3 pt-2"><AutoTextarea bind:this={editor} bind:value={draft} ariaLabel="Edit message"
          onkeydown={(e) => {
            if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) { e.preventDefault(); submitEdit() }
            if (e.key === 'Escape') editing = false
          }} /></div>
        <div class="flex justify-end gap-2 p-2">
          <Button size="sm" variant="ghost" onclick={() => (editing = false)}>Cancel</Button>
          <Button size="sm" variant="primary" onclick={submitEdit}>Send edit</Button>
        </div>
      </Well>
    {:else}
      <div class="flex items-start gap-2">
        <div class="min-w-0 flex-1 border-l-2 border-pine pl-4">
          {#if parsed.text}
            <p class="text-base font-medium whitespace-pre-wrap break-words text-ink">{parsed.text}</p>
          {/if}
          {#if msg.images.length || parsed.files.length}
            <div class="mt-2 flex flex-wrap gap-2">
              {#each msg.images as src}
                <img {src} alt="Attached" class="max-h-48 max-w-60 rounded-md border border-line object-cover" />
              {/each}
              {#each parsed.files as f}
                <Attachment name={f.name} detail={kb(f.size)} />
              {/each}
            </div>
          {/if}
        </div>
        <div class="-my-1 flex shrink-0 items-center opacity-0 transition-opacity group-focus-within:opacity-100 group-hover:opacity-100 max-md:opacity-100">
          {@render branchNav()}
          <IconButton icon="edit" label="Edit" size="sm" disabled={busy} onclick={startEdit} />
          <IconButton icon={copied ? 'check' : 'copy'} label={copied ? 'Copied' : 'Copy'} size="sm" onclick={copy} />
        </div>
      </div>
    {/if}
  </div>
{:else}
  <div class="group mt-4">
    {#if !msg.content && streaming && !cs.activeTool}
      <span class="motion-blink inline-block h-5 w-0.5 bg-pine align-middle" aria-label="Generating"></span>
    {:else if msg.content}
      <Markdown content={msg.content} streaming={streaming && !cs.activeTool} />
    {/if}
    {#if streaming && cs.activeTool}
      <ToolCall tool={cs.activeTool} running />
    {/if}

    {#if msg.error}
      <div class="mt-3">
        <Notice tone="error">
          {msg.error}
          {#snippet action()}<Button size="sm" variant="danger" onclick={() => app.regenerate(msg)}>Try again</Button>{/snippet}
        </Notice>
      </div>
    {/if}

    {#if !streaming}
      <div class="mt-2 -ml-1.5 flex items-center transition-opacity {last ? '' : 'opacity-0 group-focus-within:opacity-100 group-hover:opacity-100 max-md:opacity-100'}">
        <IconButton icon={copied ? 'check' : 'copy'} label={copied ? 'Copied' : 'Copy'} size="sm" onclick={copy} />
        <IconButton icon="refresh" label="Regenerate" size="sm" disabled={busy} onclick={() => app.regenerate(msg)} />
        {@render branchNav()}
        {#if msg.model}<span class="ml-2 truncate text-xs text-ink-3">{app.modelName(msg.model)}</span>{/if}
      </div>
    {/if}
  </div>
{/if}
