<script lang="ts">
  import { fade } from 'svelte/transition';
  import type { App } from '$lib/types';
  import { slugify } from '$lib/api';

  export let app: App;

  // Compute the effective URL - use proxy if enabled
  $: effectiveUrl = app.proxy ? `/proxy/${slugify(app.name)}/` : app.url;

  $: scale = app.scale || 1;
  $: transform = scale !== 1 ? `scale(${scale})` : '';
  $: transformOrigin = scale !== 1 ? 'top left' : '';
  $: width = scale !== 1 ? `${100 / scale}%` : '100%';
  $: height = scale !== 1 ? `${100 / scale}%` : '100%';
</script>

<div
  class="w-full h-full overflow-hidden bg-white"
  in:fade={{ duration: 150, delay: 50 }}
>
  <iframe
    src={effectiveUrl}
    title={app.name}
    class="app-frame"
    style:transform
    style:transform-origin={transformOrigin}
    style:width
    style:height
    sandbox="allow-forms allow-same-origin allow-pointer-lock allow-scripts allow-downloads allow-popups allow-modals allow-top-navigation"
    allowfullscreen
  ></iframe>
</div>
