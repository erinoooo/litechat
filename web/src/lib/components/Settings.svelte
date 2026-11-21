<script lang="ts">
  import { app } from '../state.svelte'
  import { Dialog, Icon, IconButton, NavItem } from '../ui'
  import SettingsProviders from './SettingsProviders.svelte'
  import SettingsTools from './SettingsTools.svelte'
  import SettingsPrompts from './SettingsPrompts.svelte'
  import SettingsData from './SettingsData.svelte'
  import SettingsGeneral from './SettingsGeneral.svelte'

  const pages = [
    { id: 'providers', label: 'Providers', icon: 'sparkle' },
    { id: 'tools', label: 'Tools', icon: 'tool' },
    { id: 'presets', label: 'Prompts', icon: 'message' },
    { id: 'data', label: 'Search and data', icon: 'database' },
    { id: 'general', label: 'General', icon: 'settings' },
  ] as const
</script>

<Dialog title="Settings" width="wide" tall bare onclose={() => (app.settingsOpen = false)}>
  <div class="flex min-h-0 flex-1 max-sm:flex-col">
    <nav class="flex shrink-0 gap-0.5 border-line bg-panel p-2 sm:w-48 sm:flex-col sm:border-r max-sm:overflow-x-auto max-sm:border-b" aria-label="Settings">
      <h2 class="flex h-10 items-center px-2.5 text-base font-semibold tracking-tight max-sm:hidden">Settings</h2>
      {#each pages as p (p.id)}
        <div class="shrink-0">
          <NavItem label={p.label} active={app.settingsTab === p.id} onselect={() => (app.settingsTab = p.id)}>
            {#snippet icon()}<Icon name={p.icon} size={16} />{/snippet}
          </NavItem>
        </div>
      {/each}
    </nav>
    <div class="relative min-h-0 min-w-0 flex-1 overflow-y-auto px-5 py-5 sm:px-8 sm:py-6">
      <div class="absolute top-3 right-3"><IconButton icon="x" label="Close settings" onclick={() => (app.settingsOpen = false)} /></div>
      {#if app.settingsTab === 'providers'}<SettingsProviders />
      {:else if app.settingsTab === 'tools'}<SettingsTools />
      {:else if app.settingsTab === 'presets'}<SettingsPrompts />
      {:else if app.settingsTab === 'data'}<SettingsData />
      {:else}<SettingsGeneral />{/if}
    </div>
  </div>
</Dialog>
