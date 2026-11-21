<script lang="ts">
  let { status, label }: { status: 'ok' | 'error' | 'off' | 'pending'; label?: string } = $props()
  const cls = { ok: 'bg-pine', error: 'bg-red', off: 'bg-line-strong', pending: 'bg-ink-3 motion-blink' }
</script>

<span class="inline-block size-2 shrink-0 rounded-full {cls[status]}" role="img" aria-label={label ?? status} title={label}></span>
