<script lang="ts">
  import { onMount } from 'svelte'
  import { app } from './lib/state.svelte'
  import Login from './lib/components/Login.svelte'
  import Sidebar from './lib/components/Sidebar.svelte'
  import ChatView from './lib/components/ChatView.svelte'
  import Settings from './lib/components/Settings.svelte'
  import Search from './lib/components/Search.svelte'
  import { Toaster } from './lib/ui'

  // /_ui shows the component reference (loaded on demand, not in the main bundle)
  const reference = location.pathname === '/_ui' ? import('./lib/components/Reference.svelte') : null

  onMount(() => {
    if (!reference) app.boot()
  })

  let wide = $state(window.innerWidth >= 900)

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

<svelte:window {onkeydown} onresize={() => (wide = window.innerWidth >= 900)} />

{#if reference}
  {#await reference then mod}<mod.default />{/await}
{:else if app.auth === 'loading'}
  <div class="h-full"></div>
{:else if app.auth !== 'ready'}
  <Login />
{:else}
  <div class="flex h-full overflow-hidden">
    {#if app.sidebarOpen}
      {#if wide}
        <Sidebar />
      {:else}
        <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
        <div class="motion-scrim fixed inset-0 z-30 bg-scrim" onclick={() => (app.sidebarOpen = false)}></div>
        <div class="fixed inset-y-0 left-0 z-40 shadow-overlay"><Sidebar /></div>
      {/if}
    {/if}
    <main class="flex min-w-0 flex-1"><ChatView /></main>
  </div>

  {#if app.settingsOpen}<Settings />{/if}
  {#if app.searchOpen}<Search />{/if}
{/if}

<Toaster />
