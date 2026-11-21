<script lang="ts">
  import { app } from '../state.svelte'
  import Composer from './Composer.svelte'
  import MessageView from './MessageView.svelte'
  import ModelPicker from './ModelPicker.svelte'
  import Icon from './Icon.svelte'

  const cs = $derived(app.current)
  const path = $derived(cs ? cs.path : [])

  let scroller = $state<HTMLDivElement>()
  let content = $state<HTMLDivElement>()
  let stick = $state(true)
  let showSystem = $state(false)
  let systemDraft = $state('')

  const greeting = (() => {
    const h = new Date().getHours()
    return h < 5 ? 'Up late?' : h < 12 ? 'Good morning' : h < 18 ? 'Good afternoon' : 'Good evening'
  })()

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

  function openSystem() {
    systemDraft = cs ? cs.chat.system_prompt : app.draftSystem
    showSystem = true
  }

  function saveSystem() {
    app.setSystemPrompt(systemDraft)
    showSystem = false
  }

  const systemSet = $derived(!!(cs ? cs.chat.system_prompt : app.draftSystem))
</script>

<div class="flex h-full min-w-0 flex-1 flex-col">
  <header class="flex h-14 shrink-0 items-center gap-1 px-2 sm:px-3">
    {#if !app.sidebarOpen}
      <button class="btn-icon" onclick={() => (app.sidebarOpen = true)} aria-label="Open sidebar" title="Open sidebar"><Icon name="sidebar" /></button>
      <button class="btn-icon" onclick={() => app.newChat()} aria-label="New chat" title="New chat"><Icon name="plus" /></button>
    {/if}
    <ModelPicker />
    <div class="ml-auto flex items-center">
      <button
        class="flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-sm transition-colors hover:bg-hover {systemSet ? 'text-accent' : 'text-muted'}"
        onclick={openSystem}
        title="System prompt"
      >
        <Icon name="sliders" size={16} />
        <span class="max-sm:hidden">{systemSet ? 'System prompt on' : 'System prompt'}</span>
      </button>
    </div>
  </header>

  {#if !cs || path.length === 0}
    <!-- empty state -->
    <div class="flex min-h-0 flex-1 flex-col items-center justify-center pb-[12vh]">
      <h1 class="animate-fade-up mb-7 px-4 text-center text-[28px] font-semibold tracking-tight sm:text-[32px]">
        <span class="text-accent">✦</span> {greeting}
      </h1>
      <div class="w-full"><Composer centered /></div>
      {#if app.presets.length}
        <div class="mt-4 flex max-w-3xl flex-wrap justify-center gap-2 px-4">
          {#each app.presets as p (p.id)}
            {@const on = (cs ? cs.chat.system_prompt : app.draftSystem) === p.prompt}
            <button
              class="rounded-full border px-3.5 py-1.5 text-sm transition-colors {on ? 'border-accent bg-accent-soft text-fg' : 'border-line text-muted hover:bg-hover hover:text-fg'}"
              onclick={() => app.setSystemPrompt(on ? '' : p.prompt)}
            >{p.name}</button>
          {/each}
        </div>
      {/if}
    </div>
  {:else}
    <div class="relative min-h-0 flex-1">
      <div bind:this={scroller} {onscroll} class="h-full overflow-y-auto [overflow-anchor:none]">
        <div bind:this={content} class="mx-auto flex max-w-3xl flex-col gap-7 px-4 pb-10 pt-4 sm:px-6">
          {#each path as msg, i (msg.id)}
            <MessageView {msg} cs={cs!} last={i === path.length - 1} />
          {/each}
        </div>
      </div>
      {#if !stick}
        <button
          class="animate-pop absolute bottom-3 left-1/2 grid h-9 w-9 -translate-x-1/2 place-items-center rounded-full border border-line bg-elev text-muted shadow-soft hover:text-fg"
          onclick={toBottom}
          aria-label="Scroll to bottom"
        ><Icon name="arrowDown" size={17} /></button>
      {/if}
    </div>
    <Composer />
  {/if}
</div>

{#if showSystem}
  <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
  <div class="fixed inset-0 z-50 grid place-items-center bg-black/40 p-4 backdrop-blur-[2px]" onclick={(e) => e.target === e.currentTarget && (showSystem = false)}>
    <div class="animate-pop w-full max-w-xl rounded-2xl border border-line bg-elev p-5 shadow-pop">
      <div class="mb-1 flex items-center justify-between">
        <h2 class="text-base font-semibold">System prompt</h2>
        <button class="btn-icon" onclick={() => (showSystem = false)} aria-label="Close"><Icon name="x" /></button>
      </div>
      <p class="mb-3 text-sm text-muted">Instructions the model sees at the start of {cs ? 'this chat' : 'the next chat'}.</p>
      {#if app.presets.length}
        <div class="mb-3 flex flex-wrap gap-1.5">
          {#each app.presets as p (p.id)}
            <button class="rounded-full border border-line px-3 py-1 text-xs text-muted hover:bg-hover hover:text-fg" onclick={() => (systemDraft = p.prompt)}>{p.name}</button>
          {/each}
        </div>
      {/if}
      <!-- svelte-ignore a11y_autofocus -->
      <textarea bind:value={systemDraft} rows="8" class="input font-[inherit]" placeholder="You are a helpful assistant…" autofocus
        onkeydown={(e) => { if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) saveSystem() }}></textarea>
      <div class="mt-4 flex justify-end gap-2">
        {#if systemDraft}<button class="btn-ghost mr-auto" onclick={() => (systemDraft = '')}>Clear</button>{/if}
        <button class="btn-ghost" onclick={() => (showSystem = false)}>Cancel</button>
        <button class="btn-primary" onclick={saveSystem}>Save</button>
      </div>
    </div>
  </div>
{/if}
