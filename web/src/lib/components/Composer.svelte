<script lang="ts">
  import { api } from '../api'
  import { app } from '../state.svelte'
  import { Attachment, AutoTextarea, Button, FilePicker, IconButton, Well } from '../ui'

  let { centered = false }: { centered?: boolean } = $props()

  let text = $state('')
  let images = $state<string[]>([])
  let files = $state<{ name: string; content: string; loading?: boolean }[]>([])
  let dragging = $state(false)
  let input = $state<AutoTextarea>()

  const streaming = $derived(!!app.current?.streamingId)
  const extracting = $derived(files.some((f) => f.loading))
  const canSend = $derived(!!(text.trim() || images.length || files.length) && !streaming && !extracting)

  $effect(() => {
    // focus the box when switching chats (desktop only, to avoid popping the mobile keyboard)
    app.current?.chat.id
    if (window.matchMedia('(pointer: fine)').matches) queueMicrotask(() => input?.focus())
  })

  function submit() {
    if (!canSend) return
    let content = text
    if (files.length) {
      const attached = files.map((f) => `<file name="${f.name}">\n${f.content}\n</file>`).join('\n\n')
      content = attached + (content.trim() ? '\n\n' + content : '')
    }
    app.send(content, images)
    text = ''
    images = []
    files = []
  }

  function onkeydown(e: KeyboardEvent) {
    const enterSends = app.prefs.send_on_enter ?? true
    if (e.key === 'Enter' && !e.isComposing && (enterSends ? !e.shiftKey : e.metaKey || e.ctrlKey)) {
      e.preventDefault()
      submit()
    }
  }

  /** Downscale images so they fit model limits and keep the database small. */
  async function toDataURL(file: File): Promise<string> {
    const bmp = await createImageBitmap(file)
    const max = 1568
    const scale = Math.min(1, max / Math.max(bmp.width, bmp.height))
    const w = Math.round(bmp.width * scale)
    const h = Math.round(bmp.height * scale)
    const canvas = document.createElement('canvas')
    canvas.width = w
    canvas.height = h
    canvas.getContext('2d')!.drawImage(bmp, 0, 0, w, h)
    bmp.close()
    return canvas.toDataURL(file.type === 'image/png' ? 'image/png' : 'image/jpeg', 0.88)
  }

  const TEXT_EXT = /\.(txt|md|markdown|json|ya?ml|toml|ini|csv|tsv|xml|html?|css|scss|js|mjs|cjs|ts|tsx|jsx|svelte|vue|py|rb|go|rs|java|kt|swift|c|h|cpp|hpp|cs|php|sh|bash|zsh|sql|lua|r|dart|ex|exs|log|env|conf|dockerfile|gitignore)$/i

  async function addFiles(list: File[]) {
    for (const f of list) {
      if (f.type.startsWith('image/')) {
        try {
          images.push(await toDataURL(f))
        } catch {
          app.notify(`Couldn't read the image ${f.name}.`, 'error')
        }
      } else if (f.type.startsWith('text/') || TEXT_EXT.test(f.name) || f.type === 'application/json') {
        if (f.size > 2_000_000) {
          app.notify(`${f.name} is over 2 MB. Attach a smaller text file.`, 'error')
          continue
        }
        files.push({ name: f.name, content: await f.text() })
      } else if (/\.(pdf|docx)$/i.test(f.name) || f.type === 'application/pdf') {
        if (f.size > 25_000_000) {
          app.notify(`${f.name} is over 25 MB. Attach a smaller document.`, 'error')
          continue
        }
        // extracted on the server, so the browser never loads a PDF library
        files.push({ name: f.name, content: '', loading: true })
        api
          .extract(f)
          .then((r) => {
            const i = files.findIndex((x) => x.name === f.name && x.loading)
            if (i >= 0) files[i] = { name: f.name, content: r.text }
            if (r.truncated) app.notify(`${f.name} is long, so only the first part is attached.`)
          })
          .catch((e) => {
            files = files.filter((x) => !(x.name === f.name && x.loading))
            app.notify((e as Error).message, 'error')
          })
      } else {
        app.notify(`Can't attach ${f.name}. Use images, PDF, DOCX or text files.`, 'error')
      }
    }
  }

  function onpaste(e: ClipboardEvent) {
    const fs = e.clipboardData?.files
    if (fs && fs.length) {
      e.preventDefault()
      addFiles(Array.from(fs))
    }
  }

  const kb = (n: number) => `${(n / 1024).toFixed(1)} KB of text`
</script>

<Well
  highlight={dragging}
  ondragover={(e) => { e.preventDefault(); dragging = true }}
  ondragleave={() => (dragging = false)}
  ondrop={(e) => {
    e.preventDefault()
    dragging = false
    if (e.dataTransfer?.files.length) addFiles(Array.from(e.dataTransfer.files))
  }}
>
  {#if images.length || files.length}
    <div class="flex flex-wrap gap-2 px-2 pt-2">
      {#each images as src, i}
        <Attachment name="Image {i + 1}" detail="Image" thumb={src} onremove={() => images.splice(i, 1)} />
      {/each}
      {#each files as f, i}
        <Attachment name={f.name} detail={f.loading ? 'Reading…' : kb(f.content.length)} loading={f.loading} onremove={() => files.splice(i, 1)} />
      {/each}
    </div>
  {/if}

  <div class="px-3 pt-3 pb-1">
    <AutoTextarea bind:this={input} bind:value={text} {onkeydown} {onpaste} ariaLabel="Message"
      placeholder={centered ? `Message ${app.activeModel ? app.modelName(app.activeModel) : 'the model'}` : 'Reply'} />
  </div>

  <div class="flex items-center gap-1 p-2">
    <FilePicker label="Attach images, PDF, DOCX or text" multiple onfiles={addFiles} />
    {#if app.toolsAvailable}
      <Button size="sm" variant="ghost" icon="tool" pressed={app.toolsOn}
        title={app.toolsOn ? 'Tools on: the model can use your MCP servers' : 'Tools off'}
        onclick={() => app.savePrefs({ tools_enabled: !app.toolsOn })}>Tools</Button>
    {/if}
    <span class="flex-1"></span>
    {#if streaming}
      <IconButton icon="stop" tone="primary" label="Stop (Esc)" onclick={() => app.stop()} />
    {:else}
      <IconButton icon="arrowUp" tone="primary" label="Send (Enter)" disabled={!canSend} onclick={submit} />
    {/if}
  </div>
</Well>
