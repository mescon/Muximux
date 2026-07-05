<script lang="ts">
  import { announcement } from '$lib/announce';
</script>

<!--
  A single visually-hidden polite live region, mounted once at the app root.
  Screen readers announce whatever `announce()` publishes here (e.g. the
  result of a keyboard reorder) without any visible UI. `aria-atomic` makes
  the whole message read as a unit rather than only the changed word.
-->
<div class="sr-only-announcer" role="status" aria-live="polite" aria-atomic="true">
  {$announcement}
</div>

<style>
  .sr-only-announcer {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }
</style>
