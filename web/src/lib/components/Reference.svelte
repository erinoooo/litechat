<script lang="ts">
  // Living reference for the UI kit, at /_ui. Loaded on demand; not part of the main bundle.
  // When you add or change a primitive in src/lib/ui, show it here.
  import {
    Attachment, AutoTextarea, Badge, Button, Disclosure, IconButton, Kbd, List, ListRow, Menu, MenuGroup, MenuItem,
    NavItem, Notice, PageHeader, Progress, SearchField, Section, Segmented, Select, Spinner, StatusDot, Switch,
    TextArea, TextField, Well,
  } from '../ui'

  let theme = $state(document.documentElement.dataset.theme ?? 'system')
  let menuOpen = $state(false)
  let sw = $state(true)
  let text = $state('')
  let seg = $state('a')

  const colors = ['canvas', 'panel', 'raised', 'hover', 'line', 'line-strong', 'ink', 'ink-2', 'ink-3', 'pine', 'pine-wash', 'red', 'red-wash']
  const sizes = [
    ['text-xl', 'Heading 22/28 semibold'],
    ['text-lg', 'Title 17/24 semibold'],
    ['text-base', 'Body 15/24 regular'],
    ['text-sm', 'Small 13/18'],
    ['text-xs', 'Caption 12/16'],
  ]

  function setTheme(v: string) {
    theme = v
    if (v === 'system') delete document.documentElement.dataset.theme
    else document.documentElement.dataset.theme = v
  }
</script>

<div class="h-full overflow-y-auto">
  <div class="mx-auto max-w-column px-6 py-10">
    <PageHeader title="liteChat UI kit" description="Tokens and components from src/lib/ui. Rules live in web/DESIGN.md.">
      {#snippet actions()}
        <Segmented ariaLabel="Theme" value={theme} onchange={setTheme} options={[
          { value: 'system', label: 'System' }, { value: 'light', label: 'Light' }, { value: 'dark', label: 'Dark' },
        ]} />
      {/snippet}
    </PageHeader>

    <Section title="Color">
      <div class="grid grid-cols-4 gap-3 sm:grid-cols-7">
        {#each colors as c}
          <div>
            <div class="h-10 rounded-md border border-line" style="background: var(--c-{c})"></div>
            <p class="mt-1 text-xs text-ink-2">{c}</p>
          </div>
        {/each}
      </div>
    </Section>

    <Section title="Type">
      {#each sizes as [cls, name]}
        <p class="{cls} {cls === 'text-xl' || cls === 'text-lg' ? 'font-semibold tracking-tight' : ''}">{name}</p>
      {/each}
      <p class="mt-2 font-mono text-sm">IBM Plex Mono, code only</p>
    </Section>

    <Section title="Buttons">
      <div class="flex flex-wrap items-center gap-2">
        <Button variant="primary">Save changes</Button>
        <Button>Cancel</Button>
        <Button variant="ghost" icon="settings">Settings</Button>
        <Button variant="danger" icon="trash">Delete</Button>
        <Button size="sm" icon="plus">Add</Button>
        <Button size="sm" variant="ghost" icon="tool" pressed>Tools</Button>
        <Button variant="title" menu>Model name</Button>
        <IconButton icon="copy" label="Copy" />
        <IconButton icon="trash" label="Delete" tone="danger" />
        <IconButton icon="arrowUp" label="Send" tone="primary" />
        <IconButton icon="refresh" label="Refresh" size="sm" spin />
      </div>
    </Section>

    <Section title="Inputs">
      <div class="flex max-w-md flex-col gap-4">
        <TextField label="Name" placeholder="My server" hint="A hint explains the field." />
        <TextField label="Base URL" mono value="http://localhost" error="Use an http(s) URL." />
        <Select label="API type" options={[{ value: 'a', label: 'OpenAI-compatible' }, { value: 'b', label: 'Anthropic' }]} />
        <TextArea label="System prompt" rows={3} placeholder="Answer briefly." />
        <Switch label="Send with Enter" description="When off, Enter adds a new line." bind:checked={sw} />
        <Segmented ariaLabel="Example" bind:value={seg} options={[{ value: 'a', label: 'One' }, { value: 'b', label: 'Two' }]} />
      </div>
    </Section>

    <Section title="Writing surface">
      <Well>
        <div class="px-3 pt-3 pb-1"><AutoTextarea bind:value={text} ariaLabel="Example" placeholder="Message" /></div>
        <div class="flex items-center gap-2 px-2 pb-2">
          <Attachment name="report.pdf" detail="4.2 KB of text" onremove={() => {}} />
          <Attachment name="notes.md" detail="Reading…" loading />
          <span class="flex-1"></span>
          <IconButton icon="arrowUp" label="Send" tone="primary" />
        </div>
      </Well>
    </Section>

    <Section title="Lists and status">
      <List>
        <ListRow title="Ollama" subtitle="http://localhost:11434/v1, 12 models">
          {#snippet leading()}<StatusDot status="ok" />{/snippet}
          {#snippet trailing()}<Switch checked ariaLabel="On" /><IconButton icon="edit" label="Edit" />{/snippet}
        </ListRow>
        <ListRow title="OpenRouter" error="Provider returned 401: invalid key">
          {#snippet leading()}<StatusDot status="error" />{/snippet}
        </ListRow>
        <ListRow title="Old server" subtitle="Turned off" dim>
          {#snippet leading()}<StatusDot status="off" />{/snippet}
        </ListRow>
      </List>
      <div class="mt-4 flex items-center gap-3">
        <Badge>Neutral</Badge><Badge tone="pine">On</Badge><Kbd keys="Ctrl K" /><Spinner /><StatusDot status="pending" />
      </div>
      <div class="mt-4 max-w-sm"><Progress value={62} label="Example" /></div>
    </Section>

    <Section title="Feedback">
      <div class="flex flex-col gap-3">
        <Notice>Local servers are off. Restart with --allow-local-mcp to allow them.</Notice>
        <Notice tone="error">The provider returned 429: rate limited.
          {#snippet action()}<Button size="sm" variant="danger">Try again</Button>{/snippet}
        </Notice>
        <Disclosure>
          {#snippet summary()}<code class="min-w-0 flex-1 truncate font-mono text-sm">search("weather in Ankara")</code><span class="text-xs text-ink-3">Brave</span>{/snippet}
          <p class="text-ink-2">Expanded details.</p>
        </Disclosure>
      </div>
    </Section>

    <Section title="Navigation and menus">
      <div class="flex gap-8">
        <div class="w-sidebar rounded-md bg-panel p-2">
          <MenuGroup label="Today">
            <NavItem label="Debounced search input" active />
            <NavItem label="Trip to Cappadocia" />
          </MenuGroup>
        </div>
        <Menu bind:open={menuOpen}>
          {#snippet trigger({ toggle })}<Button menu expanded={menuOpen} onclick={toggle}>Open menu</Button>{/snippet}
          {#snippet header()}<SearchField placeholder="Find" />{/snippet}
          <MenuGroup label="Group">
            <MenuItem icon="pin" label="Pin" />
            <MenuItem icon="edit" label="Rename" selected />
            <MenuItem icon="trash" label="Delete" tone="danger" />
          </MenuGroup>
        </Menu>
      </div>
    </Section>
  </div>
</div>
