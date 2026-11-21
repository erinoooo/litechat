<script lang="ts">
  import { onMount } from 'svelte'
  import { app } from './lib/state.svelte'
  import Login from './lib/components/Login.svelte'
  import Sidebar from './lib/components/Sidebar.svelte'
  import ChatView from './lib/components/ChatView.svelte'
  import Settings from './lib/components/Settings.svelte'
  import Search from './lib/components/Search.svelte'

  onMount(() => app.boot())

  const isMobile = () => window.innerWidth < 900

  function onkeydown(e: KeyboardEvent) {
    if (app.auth !== 'ready') return
    const mod = e.metaKey || e.ctrlKey
    if (mod && e.key.toLowerCase() === 'k') {
      e.preventDefault()
      app.searchOpen = !app.searchOpen
    } else if (mod && e.shiftKey && e.key.toLowerCase() === 'o') {
      e.preventDefault()
      app.newChat()
    } else if (mod && e.key === '\\') {
      e.preventDefault()
      app.sidebarOpen = !app.sidebarOpen
    } else if (e.key === 'Escape' && app.current?.streamingId && !app.searchOpen && !app.settingsOpen) {
      app.stop()
    }
  }
</script>

<svelte:window {onkeydown} />

{#if app.auth === 'loading'}
  <div class="h-full"></div>
{:else if app.auth !== 'ready'}
  <Login />
{:else}
  <div class="flex h-full overflow-hidden">
    {#if app.sidebarOpen}
      {#if isMobile()}
        <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
        <div class="fixed inset-0 z-30 bg-black/40" onclick={() => (app.sidebarOpen = false)}></div>
        <div class="fixed inset-y-0 left-0 z-40 animate-[fade-up_0.18s_ease-out]"><Sidebar /></div>
      {:else}
        <Sidebar />
      {/if}
    {/if}
    <main class="flex min-w-0 flex-1"><ChatView /></main>
  </div>

  {#if app.settingsOpen}<Settings />{/if}
  {#if app.searchOpen}<Search />{/if}
{/if}

{#if app.toast}
  <div class="pointer-events-none fixed inset-x-0 bottom-24 z-[60] flex justify-center px-4">
    <div class="animate-fade-up pointer-events-auto max-w-md rounded-xl px-4 py-2.5 text-sm shadow-pop {app.toast.kind === 'error' ? 'bg-danger text-white' : 'bg-fg text-bg'}">
      {app.toast.text}
    </div>
  </div>
{/if}
