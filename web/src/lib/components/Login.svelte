<script lang="ts">
  import { api } from '../api'
  import { app } from '../state.svelte'
  import { Button, TextField } from '../ui'

  const setup = $derived(app.auth === 'setup')
  let password = $state('')
  let confirmPw = $state('')
  let error = $state('')
  let busy = $state(false)

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    error = ''
    if (setup && password !== confirmPw) {
      error = "The passwords don't match."
      return
    }
    if (setup && password.length < 8) {
      error = 'Use at least 8 characters.'
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
      if (!setup) password = ''
    } finally {
      busy = false
    }
  }
</script>

<div class="flex min-h-full items-center justify-center p-4">
  <form onsubmit={submit} class="w-full max-w-80">
    <img src="/icon.svg" alt="" class="mb-6 size-8" />
    <h1 class="text-xl font-semibold tracking-tight">{setup ? 'Set a password' : 'Log in to liteChat'}</h1>
    <p class="mt-1 mb-6 text-sm text-ink-2">
      {setup ? 'This is the only account. You can change the password later in Settings.' : 'Enter the password for this server.'}
    </p>
    <div class="flex flex-col gap-3">
      <TextField type="password" label="Password" bind:value={password} autofocus autocomplete={setup ? 'new-password' : 'current-password'} required error={!setup ? error : undefined} />
      {#if setup}
        <TextField type="password" label="Confirm password" bind:value={confirmPw} autocomplete="new-password" required error={error || undefined} />
      {/if}
      <div class="mt-2">
        <Button type="submit" variant="primary" full disabled={busy || !password}>{setup ? 'Set password' : 'Log in'}</Button>
      </div>
    </div>
  </form>
</div>
