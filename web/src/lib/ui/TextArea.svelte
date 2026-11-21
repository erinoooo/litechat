<script lang="ts">
  import Field from './Field.svelte'

  type Props = {
    value?: string
    label?: string
    hint?: string
    error?: string
    placeholder?: string
    rows?: number
    mono?: boolean
    autofocus?: boolean
    onkeydown?: (e: KeyboardEvent) => void
    onchange?: (e: Event & { currentTarget: HTMLTextAreaElement }) => void
  }
  let { value = $bindable(''), label, hint, error, placeholder, rows = 4, mono = false, autofocus, onkeydown, onchange }: Props = $props()
  const id = 'f' + Math.random().toString(36).slice(2, 8)
</script>

<Field {id} {label} {hint} {error}>
  <!-- svelte-ignore a11y_autofocus -->
  <textarea
    {id}
    {rows}
    {placeholder}
    {autofocus}
    bind:value
    {onkeydown}
    {onchange}
    class="w-full resize-y rounded-md border border-line-strong bg-canvas px-3 py-2 text-sm text-ink transition-colors outline-none placeholder:text-ink-3 focus:border-pine focus:shadow-[0_0_0_3px_var(--c-pine-wash)] {mono ? 'font-mono' : ''}"
  ></textarea>
</Field>
