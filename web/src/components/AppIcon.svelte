<script lang="ts">
  import type { AppIcon as AppIconType } from '$lib/types';

  let { icon, name, color = '#374151', size = 'md', showBackground = true }: {
    icon: AppIconType;
    name: string;
    color?: string;
    size?: 'sm' | 'md' | 'lg' | 'xl';
    showBackground?: boolean;
  } = $props();

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

  let iconUrl = $derived(getIconUrl());
  let fallbackLetter = $derived(name.charAt(0).toUpperCase());

  let imageError = $state(false);

  function handleImageError() {
    imageError = true;
  }
</script>

<div
  class="rounded flex items-center justify-center font-bold {sizeClasses[size]}"
  style="background-color: {showBackground ? color : 'transparent'}"
>
  {#if iconUrl && !imageError}
    {#if icon?.type === 'builtin'}
      <div
        class="w-full h-full p-1 builtin-icon"
        style="-webkit-mask-image: url({iconUrl}); mask-image: url({iconUrl});"
        role="img"
        aria-label={name}
      ></div>
    {:else}
      <img
        src={iconUrl}
        alt={name}
        class="w-full h-full object-contain p-1"
        onerror={handleImageError}
      />
    {/if}
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
  .builtin-icon {
    background-color: var(--text-primary, #fff);
    -webkit-mask-size: contain;
    mask-size: contain;
    -webkit-mask-repeat: no-repeat;
    mask-repeat: no-repeat;
    -webkit-mask-position: center;
    mask-position: center;
  }
</style>
