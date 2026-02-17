<script lang="ts">
  import type { AppIcon as AppIconType } from '$lib/types';
  import { getBase } from '$lib/api';

  let { icon, name, color = '#374151', size = 'md', showBackground = true, forceBackground = false, scale }: {
    icon: AppIconType;
    name: string;
    color?: string;
    size?: 'sm' | 'md' | 'lg' | 'xl';
    showBackground?: boolean;
    forceBackground?: boolean;
    scale?: number;
  } = $props();

  // Size classes
  const sizeClasses = {
    sm: 'w-6 h-6 text-xs',
    md: 'w-8 h-8 text-sm',
    lg: 'w-12 h-12 text-lg',
    xl: 'w-16 h-16 text-2xl'
  };

  // Base pixel sizes for inline scale override
  const sizePx: Record<string, number> = { sm: 24, md: 32, lg: 48, xl: 64 };
  let scaleStyle = $derived(scale && scale !== 1 ? `width: ${sizePx[size] * scale}px; height: ${sizePx[size] * scale}px;` : '');

  // Generate icon URL based on type
  function getIconUrl(): string | null {
    if (!icon) return null;

    const base = getBase();
    switch (icon.type) {
      case 'dashboard': {
        if (!icon.name) return null;
        const variant = icon.variant || 'svg';
        return `${base}/icons/dashboard/${icon.name}.${variant}`;
      }
      case 'custom':
        if (!icon.file) return null;
        return `${base}/icons/custom/${icon.file}`;
      case 'url':
        return icon.url || null;
      case 'lucide':
        if (!icon.name) return null;
        return `${base}/icons/lucide/${icon.name}.svg`;
      default:
        return null;
    }
  }

  let iconUrl = $derived(getIconUrl());
  let fallbackLetter = $derived(name.charAt(0).toUpperCase());
  // When showBackground is enabled, use the icon's explicit background if set,
  // otherwise darken the app color to create contrast.
  let bgColor = $derived(
    (showBackground || forceBackground)
      ? (icon?.background && icon.background !== 'transparent'
          ? icon.background
          : `color-mix(in srgb, ${color} 50%, black)`)
      : 'transparent'
  );
  // Icon tint color for Lucide (CSS mask); falls back to theme text color
  let tintColor = $derived(icon?.color || '');

  let imageError = $state(false);

  function handleImageError() {
    imageError = true;
  }
</script>

<div
  class="rounded flex items-center justify-center font-bold {sizeClasses[size]}"
  style="background-color: {bgColor};{scaleStyle}"
>
  {#if iconUrl && !imageError}
    {#if icon?.type === 'lucide'}
      <div
        class="w-full h-full {showBackground ? 'p-1.5' : 'p-1'} lucide-icon"
        style="-webkit-mask-image: url({iconUrl}); mask-image: url({iconUrl});{tintColor ? ` background-color: ${tintColor};` : ''}"
        role="img"
        aria-label={name}
      ></div>
    {:else}
      <img
        src={iconUrl}
        alt={name}
        class="w-full h-full object-contain {showBackground ? 'p-1.5' : 'p-1'}"
        style={icon?.invert ? 'filter: invert(1);' : ''}
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
  .lucide-icon {
    background-color: var(--text-primary, #fff);
    -webkit-mask-size: contain;
    mask-size: contain;
    -webkit-mask-repeat: no-repeat;
    mask-repeat: no-repeat;
    -webkit-mask-position: center;
    mask-position: center;
    -webkit-mask-origin: content-box;
    mask-origin: content-box;
  }
</style>
