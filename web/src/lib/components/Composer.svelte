<script lang="ts">
  import { api } from '../api'
  import { app } from '../state.svelte'
  import Icon from './Icon.svelte'

  let { centered = false }: { centered?: boolean } = $props()

  let text = $state('')
  let images = $state<string[]>([])
  let files = $state<{ name: string; content: string; loading?: boolean }[]>([])
  let dragging = $state(false)
  let el = $state<HTMLTextAreaElement>()
  let fileInput = $state<HTMLInputElement>()

  const streaming = $derived(!!app.current?.streamingId)
  const extracting = $derived(files.some((f) => f.loading))
  const canSend = $derived((text.trim() || images.length || files.length) && !streaming && !extracting)

  export function focus() {
    el?.focus()
  }

  $effect(() => {
    // focus the box when switching chats (desktop only, to avoid popping the mobile keyboard)
    app.current?.chat.id
    if (window.matchMedia('(pointer: fine)').matches) queueMicrotask(() => el?.focus())
  })

  function autosize() {
    if (!el) return
    el.style.height = 'auto'
    el.style.height = Math.min(el.scrollHeight, 320) + 'px'
  }

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
    queueMicrotask(autosize)
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
    const type = file.type === 'image/png' ? 'image/png' : 'image/jpeg'
    return canvas.toDataURL(type, 0.88)
  }

  const TEXT_EXT = /\.(txt|md|markdown|json|ya?ml|toml|ini|csv|tsv|xml|html?|css|scss|js|mjs|cjs|ts|tsx|jsx|svelte|vue|py|rb|go|rs|java|kt|swift|c|h|cpp|hpp|cs|php|sh|bash|zsh|sql|lua|r|dart|ex|exs|log|env|conf|dockerfile|gitignore)$/i

  async function addFiles(list: FileList | File[]) {
    for (const f of Array.from(list)) {
      if (f.type.startsWith('image/')) {
        try {
          images.push(await toDataURL(f))
        } catch {
          app.notify(`Couldn't read image ${f.name}`, 'error')
        }
      } else if (f.type.startsWith('text/') || TEXT_EXT.test(f.name) || f.type === 'application/json') {
        if (f.size > 2_000_000) {
          app.notify(`${f.name} is too large (max 2 MB)`, 'error')
          continue
        }
        files.push({ name: f.name, content: await f.text() })
      } else if (/\.(pdf|docx)$/i.test(f.name) || f.type === 'application/pdf') {
        if (f.size > 25_000_000) {
          app.notify(`${f.name} is too large (max 25 MB)`, 'error')
          continue
        }
        // extracted on the server, so the browser never loads a PDF library
        const entry = { name: f.name, content: '', loading: true }
        files.push(entry)
        api
          .extract(f)
          .then((r) => {
            const i = files.findIndex((x) => x.name === f.name && x.loading)
            if (i >= 0) files[i] = { name: f.name, content: r.text }
            if (r.truncated) app.notify(`${f.name} was long; only the first part is included`)
          })
          .catch((e) => {
            files = files.filter((x) => !(x.name === f.name && x.loading))
            app.notify((e as Error).message, 'error')
          })
      } else {
        app.notify(`${f.name}: supported files are images, PDF, DOCX and text`, 'error')
      }
    }
  }

  function onpaste(e: ClipboardEvent) {
    const fs = e.clipboardData?.files
    if (fs && fs.length) {
      e.preventDefault()
      addFiles(fs)
    }
  }

  function ondrop(e: DragEvent) {
    e.preventDefault()
    dragging = false
    if (e.dataTransfer?.files.length) addFiles(e.dataTransfer.files)
  }
</script>

<div class="mx-auto w-full max-w-3xl px-3 sm:px-4 {centered ? '' : 'pb-3 sm:pb-4'}" style="padding-bottom: max(env(safe-area-inset-bottom), {centered ? '0px' : '12px'})">
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="relative rounded-[22px] border bg-elev shadow-soft transition-colors {dragging ? 'border-accent' : 'border-line focus-within:border-line-strong'}"
    ondragover={(e) => { e.preventDefault(); dragging = true }}
    ondragleave={() => (dragging = false)}
    {ondrop}
  >
    {#if images.length || files.length}
      <div class="flex flex-wrap gap-2 px-3 pt-3">
        {#each images as src, i}
          <div class="group relative">
            <img {src} alt="" class="h-16 w-16 rounded-lg border border-line object-cover" />
            <button
              class="absolute -right-1.5 -top-1.5 grid h-5 w-5 place-items-center rounded-full bg-fg text-bg opacity-0 shadow transition-opacity group-hover:opacity-100 max-md:opacity-100"
              onclick={() => images.splice(i, 1)}
              aria-label="Remove image"><Icon name="x" size={12} stroke={2.5} /></button>
          </div>
        {/each}
        {#each files as f, i}
          <div class="group relative flex h-16 max-w-[200px] items-center gap-2 rounded-lg border border-line bg-sunk px-3">
            {#if f.loading}
              <span class="h-4 w-4 shrink-0 animate-spin rounded-full border-2 border-muted border-t-transparent"></span>
            {:else}
              <Icon name="file" size={16} class="text-muted" />
            {/if}
            <div class="min-w-0">
              <div class="truncate text-sm font-medium">{f.name}</div>
              <div class="text-xs text-faint">{f.loading ? 'Reading…' : `${(f.content.length / 1024).toFixed(1)} KB of text`}</div>
            </div>
            <button
              class="absolute -right-1.5 -top-1.5 grid h-5 w-5 place-items-center rounded-full bg-fg text-bg opacity-0 shadow transition-opacity group-hover:opacity-100 max-md:opacity-100"
              onclick={() => files.splice(i, 1)}
              aria-label="Remove file"><Icon name="x" size={12} stroke={2.5} /></button>
          </div>
        {/each}
      </div>
    {/if}

    <textarea
      bind:this={el}
      bind:value={text}
      oninput={autosize}
      {onkeydown}
      {onpaste}
      rows="1"
      placeholder={centered ? 'Ask anything…' : 'Reply…'}
      class="block max-h-80 w-full resize-none bg-transparent px-4 pt-3.5 pb-1 text-[15px] leading-relaxed outline-none placeholder:text-faint"
    ></textarea>

    <div class="flex items-center justify-between px-2.5 pb-2.5 pt-1">
      <div class="flex items-center">
        <button class="btn-icon" onclick={() => fileInput?.click()} aria-label="Attach files" title="Attach images, PDFs, DOCX or text files">
          <Icon name="paperclip" />
        </button>
        <input bind:this={fileInput} type="file" multiple class="hidden" onchange={(e) => { const t = e.currentTarget; if (t.files) addFiles(t.files); t.value = '' }} />
        {#if app.toolsAvailable}
          <button
            class="ml-0.5 flex h-[30px] items-center gap-1.5 rounded-full px-2.5 text-[13px] font-medium transition-colors {app.toolsOn ? 'bg-accent-soft text-fg' : 'text-muted hover:bg-hover'}"
            onclick={() => app.savePrefs({ tools_enabled: !app.toolsOn })}
            title={app.toolsOn ? 'Tools are on: the model can call your MCP servers' : 'Tools are off'}
            aria-pressed={app.toolsOn}
          >
            <Icon name="tool" size={15} class={app.toolsOn ? 'text-accent' : ''} /> Tools
          </button>
        {/if}
      </div>
      {#if streaming}
        <button class="grid h-8 w-8 place-items-center rounded-full bg-fg text-bg transition-transform active:scale-95" onclick={() => app.stop()} aria-label="Stop generating" title="Stop">
          <Icon name="stop" size={14} stroke={0} class="fill-current" />
        </button>
      {:else}
        <button
          class="grid h-8 w-8 place-items-center rounded-full transition-all active:scale-95 {canSend ? 'bg-accent text-accent-fg' : 'bg-hover text-faint'}"
          disabled={!canSend}
          onclick={submit}
          aria-label="Send"
          title="Send (Enter)"
        >
          <Icon name="arrowUp" size={17} stroke={2.25} />
        </button>
      {/if}
    </div>
  </div>
</div>
