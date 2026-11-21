<script lang="ts">
  /** Borderless, auto-growing text input for composer-style surfaces. */
  type Props = {
    value?: string
    placeholder?: string
    maxHeight?: number
    ariaLabel: string
    onkeydown?: (e: KeyboardEvent) => void
    onpaste?: (e: ClipboardEvent) => void
  }
  let { value = $bindable(''), placeholder, maxHeight = 320, ariaLabel, onkeydown, onpaste }: Props = $props()
  let el = $state<HTMLTextAreaElement>()

  export function focus() {
    el?.focus()
  }

  export function setCursorEnd() {
    if (!el) return
    el.focus()
    el.setSelectionRange(value.length, value.length)
  }

  $effect(() => {
    value // resize whenever the value changes, including programmatic resets
    if (!el) return
    el.style.height = 'auto'
    el.style.height = Math.min(el.scrollHeight, maxHeight) + 'px'
  })
</script>

<textarea
  bind:this={el}
  bind:value
  {placeholder}
  {onkeydown}
  {onpaste}
  aria-label={ariaLabel}
  rows="1"
  class="block w-full resize-none bg-transparent text-base text-ink outline-none placeholder:text-ink-3"
></textarea>
