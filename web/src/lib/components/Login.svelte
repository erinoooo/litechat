<script lang="ts">
  import { api } from '../api'
  import { app } from '../state.svelte'

  const setup = $derived(app.auth === 'setup')
  let password = $state('')
  let confirmPw = $state('')
  let error = $state('')
  let busy = $state(false)

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    error = ''
    if (setup && password !== confirmPw) {
      error = "Passwords don't match"
      return
    }
    busy = true
    try {
      if (setup) await api.setup(password)
      else await api.login(password)
      app.auth = 'ready'
      await app.loadAll()
    } catch (err) {
      error = (err as Error).message
      password = setup ? password : ''
    } finally {
      busy = false
    }
  }
</script>

<div class="grid min-h-full place-items-center p-4">
  <form onsubmit={submit} class="animate-fade-up w-full max-w-sm">
    <div class="mb-8 text-center">
      <img src="/icon.svg" alt="" class="mx-auto mb-4 h-12 w-12" />
      <h1 class="text-2xl font-semibold tracking-tight">{setup ? 'Welcome to liteChat' : 'Welcome back'}</h1>
      <p class="mt-1.5 text-sm text-muted">{setup ? 'Choose a password to protect your chats.' : 'Enter your password to continue.'}</p>
    </div>
    <div class="space-y-3 rounded-2xl border border-line bg-elev p-5 shadow-soft">
      <!-- svelte-ignore a11y_autofocus -->
      <input class="input" type="password" placeholder="Password" bind:value={password} autofocus
        autocomplete={setup ? 'new-password' : 'current-password'} minlength={setup ? 8 : undefined} required />
      {#if setup}
        <input class="input" type="password" placeholder="Confirm password" bind:value={confirmPw} autocomplete="new-password" required />
      {/if}
      {#if error}<p class="text-sm text-danger">{error}</p>{/if}
      <button class="btn-primary h-10 w-full" disabled={busy || !password}>
        {busy ? '…' : setup ? 'Create password' : 'Log in'}
      </button>
    </div>
  </form>
</div>
