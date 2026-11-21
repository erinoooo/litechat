<script lang="ts">
  import type { Message } from '../api'
  import { app, type ChatState } from '../state.svelte'
  import Markdown from './Markdown.svelte'
  import Icon from './Icon.svelte'

  let { msg, cs, last }: { msg: Message; cs: ChatState; last: boolean } = $props()

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
  let editEl = $state<HTMLTextAreaElement>()

  function startEdit() {
    draft = msg.content
    editing = true
    queueMicrotask(() => {
      if (!editEl) return
      editEl.focus()
      editEl.setSelectionRange(draft.length, draft.length)
      autosize()
    })
  }

  function autosize() {
    if (!editEl) return
    editEl.style.height = 'auto'
    editEl.style.height = Math.min(editEl.scrollHeight, 400) + 'px'
  }

  function submitEdit() {
    if (!draft.trim() || draft === msg.content) {
      editing = false
      return
    }
    editing = false
    app.edit(msg, draft)
  }

  function copy() {
    const text = msg.content.replace(/<think>[\s\S]*?(<\/think>|$)/g, '').trim()
    navigator.clipboard.writeText(text)
    copied = true
    setTimeout(() => (copied = false), 1400)
  }
</script>

{#snippet branchNav()}
  {#if siblings.length > 1}
    <div class="flex items-center text-xs text-faint tabular-nums select-none">
      <button class="btn-icon" disabled={sibIndex === 0 || busy} onclick={() => app.switchBranch(msg, -1)} aria-label="Previous version">
        <Icon name="chevronLeft" size={14} />
      </button>
      <span class="px-0.5">{sibIndex + 1}/{siblings.length}</span>
      <button class="btn-icon" disabled={sibIndex === siblings.length - 1 || busy} onclick={() => app.switchBranch(msg, 1)} aria-label="Next version">
        <Icon name="chevronRight" size={14} />
      </button>
    </div>
  {/if}
{/snippet}

{#if msg.role === 'user'}
  <div class="group flex flex-col items-end gap-1 animate-fade-up">
    {#if editing}
      <div class="w-full rounded-2xl border border-line-strong bg-elev p-3 shadow-soft">
        <textarea
          bind:this={editEl}
          bind:value={draft}
          oninput={autosize}
          onkeydown={(e) => {
            if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) { e.preventDefault(); submitEdit() }
            if (e.key === 'Escape') editing = false
          }}
          class="w-full resize-none bg-transparent outline-none"
          rows="1"
        ></textarea>
        <div class="mt-2 flex justify-end gap-2">
          <button class="btn-ghost" onclick={() => (editing = false)}>Cancel</button>
          <button class="btn-primary" onclick={submitEdit}>Send</button>
        </div>
      </div>
    {:else}
      {#if msg.images.length}
        <div class="flex flex-wrap justify-end gap-2">
          {#each msg.images as src}
            <img {src} alt="" class="max-h-56 max-w-[240px] rounded-xl border border-line object-cover" />
          {/each}
        </div>
      {/if}
      {#if parsed.files.length}
        <div class="flex flex-wrap justify-end gap-2">
          {#each parsed.files as f}
            <div class="flex items-center gap-2 rounded-xl border border-line bg-elev px-3 py-2 text-sm">
              <Icon name="paperclip" size={15} class="text-muted" />
              <span class="max-w-[200px] truncate font-medium">{f.name}</span>
              <span class="text-xs text-faint">{(f.size / 1024).toFixed(1)} KB</span>
            </div>
          {/each}
        </div>
      {/if}
      {#if parsed.text}
        <div class="max-w-[85%] whitespace-pre-wrap break-words rounded-2xl rounded-br-md bg-bubble px-4 py-2.5">{parsed.text}</div>
      {/if}
      <div class="flex items-center gap-0.5 opacity-0 transition-opacity group-hover:opacity-100 focus-within:opacity-100 max-md:opacity-100">
        {@render branchNav()}
        <button class="btn-icon" onclick={copy} aria-label="Copy" title="Copy"><Icon name={copied ? 'check' : 'copy'} size={15} /></button>
        <button class="btn-icon" onclick={startEdit} disabled={busy} aria-label="Edit" title="Edit"><Icon name="edit" size={15} /></button>
      </div>
    {/if}
  </div>
{:else}
  <div class="group animate-fade-up">
    {#if !msg.content && streaming}
      <div class="typing" aria-label="Generating"><span></span><span></span><span></span></div>
    {:else}
      <Markdown content={msg.content} {streaming} />
    {/if}

    {#if msg.error}
      <div class="mt-3 flex items-start gap-2 rounded-xl border border-danger/30 bg-danger/5 px-3 py-2 text-sm text-danger">
        <span class="min-w-0 flex-1 break-words">{msg.error}</span>
        <button class="shrink-0 font-medium underline underline-offset-2" onclick={() => app.regenerate(msg)}>Retry</button>
      </div>
    {/if}

    {#if !streaming}
      <div class="mt-1.5 -ml-1.5 flex items-center gap-0.5 transition-opacity {last ? '' : 'opacity-0 group-hover:opacity-100 focus-within:opacity-100 max-md:opacity-100'}">
        <button class="btn-icon" onclick={copy} aria-label="Copy" title="Copy"><Icon name={copied ? 'check' : 'copy'} size={15} /></button>
        <button class="btn-icon" onclick={() => app.regenerate(msg)} disabled={busy} aria-label="Regenerate" title="Regenerate"><Icon name="refresh" size={15} /></button>
        {@render branchNav()}
        {#if msg.model}
          <span class="ml-1.5 truncate text-xs text-faint">{app.modelName(msg.model)}</span>
        {/if}
      </div>
    {/if}
  </div>
{/if}
