<script lang="ts">
  import type { AppIcon as AppIconType } from '$lib/types';

  export let icon: AppIconType;
  export let name: string;
  export let color: string = '#374151';
  export let size: 'sm' | 'md' | 'lg' | 'xl' = 'md';

  // Size classes
  const sizeClasses = {
    sm: 'w-6 h-6 text-xs',
    md: 'w-8 h-8 text-sm',
    lg: 'w-12 h-12 text-lg',
    xl: 'w-16 h-16 text-2xl'
  };

  // Generate icon URL based on type
  function getIconUrl(): string | null {
    if (!icon) return null;

    switch (icon.type) {
      case 'dashboard':
        if (!icon.name) return null;
        const variant = icon.variant || 'svg';
        return `/icons/dashboard/${icon.name}.${variant}`;
      case 'custom':
        if (!icon.file) return null;
        return `/icons/custom/${icon.file}`;
      case 'url':
        return icon.url || null;
      case 'builtin':
        if (!icon.name) return null;
        return `/icons/builtin/${icon.name}.svg`;
      default:
        return null;
    }
  }

  $: iconUrl = getIconUrl();
  $: fallbackLetter = name.charAt(0).toUpperCase();

  let imageError = false;

  function handleImageError() {
    imageError = true;
  }
</script>

<div
  class="rounded flex items-center justify-center font-bold {sizeClasses[size]}"
  style="background-color: {color}"
>
  {#if iconUrl && !imageError}
    <img
      src={iconUrl}
      alt={name}
      class="w-full h-full object-contain p-1"
      on:error={handleImageError}
    />
  {:else}
    <span class="text-white">{fallbackLetter}</span>
  {/if}
</div>

<style>
  img {
    /* Ensure SVGs display properly */
    max-width: 100%;
    max-height: 100%;
  }
</style>
