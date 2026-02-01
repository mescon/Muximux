<script lang="ts">
  import type { App } from '$lib/types';

  export let app: App;

  $: scale = app.scale || 1;
  $: transform = scale !== 1 ? `scale(${scale})` : '';
  $: transformOrigin = scale !== 1 ? 'top left' : '';
  $: width = scale !== 1 ? `${100 / scale}%` : '100%';
  $: height = scale !== 1 ? `${100 / scale}%` : '100%';
</script>

<div class="w-full h-full overflow-hidden bg-white">
  <iframe
    src={app.url}
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
