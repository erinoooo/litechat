<script lang="ts">
  import { app } from '../state.svelte'
  import Composer from './Composer.svelte'
  import MessageView from './MessageView.svelte'
  import ModelPicker from './ModelPicker.svelte'
  import { Button, Dialog, IconButton, StatusDot, TextArea } from '../ui'

  const cs = $derived(app.current)
  const path = $derived(cs ? cs.path : [])

  let scroller = $state<HTMLDivElement>()
  let content = $state<HTMLDivElement>()
  let stick = $state(true)
  let showSystem = $state(false)
  let systemDraft = $state('')

  // Follow the bottom while content grows, unless the user scrolled up.
  $effect(() => {
    if (!content || !scroller) return
    const sc = scroller
    const ro = new ResizeObserver(() => {
      if (stick) sc.scrollTop = sc.scrollHeight
    })
    ro.observe(content)
    return () => ro.disconnect()
  })

  // Jump to the bottom when switching chats or sending.
  let lastChat = ''
  let lastLen = 0
  $effect(() => {
    const id = cs?.chat.id ?? ''
    const len = path.length
    if (id !== lastChat || len > lastLen) {
      stick = true
      queueMicrotask(() => scroller && (scroller.scrollTop = scroller.scrollHeight))
    }
    lastChat = id
    lastLen = len
  })

  function onscroll() {
    if (!scroller) return
    stick = scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight < 60
  }

  function toBottom() {
    stick = true
    scroller?.scrollTo({ top: scroller.scrollHeight, behavior: 'smooth' })
  }

  const systemPrompt = $derived(cs ? cs.chat.system_prompt : app.draftSystem)

  function openSystem() {
    systemDraft = systemPrompt
    showSystem = true
  }

  function saveSystem() {
    app.setSystemPrompt(systemDraft)
    showSystem = false
  }
</script>

<div class="flex h-full min-w-0 flex-1 flex-col">
  <header class="flex h-14 shrink-0 items-center gap-1 px-2 sm:px-3">
    {#if !app.sidebarOpen}
      <IconButton icon="sidebar" label="Show sidebar (Ctrl+\)" onclick={() => (app.sidebarOpen = true)} />
      <IconButton icon="plus" label="New chat" onclick={() => app.newChat()} />
    {/if}
    <ModelPicker />
    <span class="flex-1"></span>
    <Button variant="ghost" icon="sliders" onclick={openSystem} title="System prompt">
      <span class="max-sm:hidden">System prompt</span>
      {#if systemPrompt}<StatusDot status="ok" label="System prompt is set" />{/if}
    </Button>
  </header>

  {#if !cs || path.length === 0}
    <div class="min-h-0 flex-1 overflow-y-auto">
      <div class="mx-auto w-full max-w-column px-4 pt-empty-state pb-8 sm:px-6">
        <h1 class="mb-5 text-xl font-semibold tracking-tight">New chat</h1>
        <Composer centered />
        {#if app.presets.length}
          <div class="mt-4 flex flex-wrap items-center gap-2">
            <span class="mr-1 text-sm text-ink-3">Start from</span>
            {#each app.presets as p (p.id)}
              {@const on = systemPrompt === p.prompt}
              <Button size="sm" variant={on ? 'primary' : 'secondary'} icon={on ? 'check' : undefined} onclick={() => app.setSystemPrompt(on ? '' : p.prompt)}>{p.name}</Button>
            {/each}
          </div>
        {/if}
      </div>
    </div>
  {:else}
    <div class="relative min-h-0 flex-1">
      <div bind:this={scroller} {onscroll} class="h-full overflow-y-auto scroll-anchor-none">
        <div bind:this={content} class="mx-auto flex w-full max-w-column flex-col px-4 pt-6 pb-12 sm:px-6">
          {#each path as msg, i (msg.id)}
            <MessageView {msg} cs={cs!} last={i === path.length - 1} first={i === 0} />
          {/each}
        </div>
      </div>
      {#if !stick}
        <div class="motion-overlay absolute bottom-3 left-1/2 -translate-x-1/2 rounded-md border border-line bg-raised shadow-overlay">
          <IconButton icon="arrowDown" label="Scroll to bottom" onclick={toBottom} />
        </div>
      {/if}
    </div>
    <div class="mx-auto w-full max-w-column px-4 pb-4 sm:px-6" style="padding-bottom: max(env(safe-area-inset-bottom), 16px)">
      <Composer />
    </div>
  {/if}
</div>

{#if showSystem}
  <Dialog title="System prompt" description="Instructions the model reads before {cs ? 'this chat' : 'your next chat'}." onclose={() => (showSystem = false)}>
    {#if app.presets.length}
      <div class="mb-3 flex flex-wrap gap-2">
        {#each app.presets as p (p.id)}
          <Button size="sm" onclick={() => (systemDraft = p.prompt)}>{p.name}</Button>
        {/each}
      </div>
    {/if}
    <TextArea bind:value={systemDraft} rows={8} autofocus placeholder="For example: Answer briefly. Use metric units."
      onkeydown={(e) => { if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) saveSystem() }} />
    {#snippet footer()}
      {#if systemDraft}<Button variant="ghost" onclick={() => (systemDraft = '')}>Clear</Button>{/if}
      <span class="flex-1"></span>
      <Button onclick={() => (showSystem = false)}>Cancel</Button>
      <Button variant="primary" onclick={saveSystem}>Save</Button>
    {/snippet}
  </Dialog>
{/if}
