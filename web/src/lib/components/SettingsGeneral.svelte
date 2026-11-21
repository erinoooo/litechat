<script lang="ts">
  import { api } from '../api'
  import { app } from '../state.svelte'
  import { Button, PageHeader, Section, Segmented, Select, Switch, TextArea, TextField } from '../ui'

  let pw = $state({ current: '', next: '' })
  let defaultSystem = $state(app.prefs.default_system_prompt ?? '')

  async function changePassword() {
    try {
      await api.changePassword(pw.current, pw.next)
      pw = { current: '', next: '' }
      app.notify('Password changed. Other devices are logged out.')
    } catch (e) {
      app.notify((e as Error).message, 'error')
    }
  }

  const modelOptions = $derived([
    { value: '', label: 'First available model' },
    ...app.models.map((m) => ({ value: m.key, label: `${m.name} (${m.provider_name})` })),
  ])
</script>

<PageHeader title="General" />

<Section title="New chats">
  <div class="flex max-w-md flex-col gap-4">
    <Select label="Default model" value={app.prefs.default_model ?? ''} options={modelOptions}
      onchange={(v) => { app.savePrefs({ default_model: v }); if (v) app.draftModel = v }} />
    <TextArea label="Default system prompt" bind:value={defaultSystem} rows={3} hint="Used for every new chat unless you pick another."
      onchange={() => app.savePrefs({ default_system_prompt: defaultSystem })} />
  </div>
</Section>

<Section title="Behavior">
  <div class="flex max-w-md flex-col gap-3">
    <Switch label="Name chats automatically" description="The model writes a short title after the first reply."
      checked={app.prefs.auto_title ?? true} onchange={(v) => app.savePrefs({ auto_title: v })} />
    <Switch label="Send with Enter" description="When off, Enter adds a new line and Ctrl+Enter sends."
      checked={app.prefs.send_on_enter ?? true} onchange={(v) => app.savePrefs({ send_on_enter: v })} />
  </div>
</Section>

<Section title="Appearance">
  <Segmented ariaLabel="Theme" value={app.theme} onchange={(v) => app.setTheme(v as 'system')} options={[
    { value: 'system', label: 'System', icon: 'monitor' },
    { value: 'light', label: 'Light', icon: 'sun' },
    { value: 'dark', label: 'Dark', icon: 'moon' },
  ]} />
</Section>

<Section title="Password" description="Changing it logs out every other device.">
  <div class="flex max-w-md flex-col gap-3">
    <TextField label="Current password" type="password" autocomplete="current-password" bind:value={pw.current} />
    <TextField label="New password" type="password" autocomplete="new-password" bind:value={pw.next} hint="At least 8 characters." />
    <div><Button disabled={!pw.current || pw.next.length < 8} onclick={changePassword}>Change password</Button></div>
  </div>
</Section>
