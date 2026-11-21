<script lang="ts">
  /** Renders the single app-wide toast. Use app.notify(text, kind) to show one. */
  import { app } from '../state.svelte'
</script>

<div class="pointer-events-none fixed inset-x-0 bottom-24 z-60 flex justify-center px-4" aria-live="polite">
  {#if app.toast}
    <div
      class="motion-overlay pointer-events-auto max-w-md rounded-md border px-3.5 py-2 text-sm shadow-overlay {app.toast.kind === 'error' ? 'border-red/30 bg-red-wash text-red' : 'border-line bg-raised text-ink'}"
      role={app.toast.kind === 'error' ? 'alert' : 'status'}
    >
      {app.toast.text}
    </div>
  {/if}
</div>
