<script lang="ts">
  import { getAvailableLocales } from '$lib/localeStore';

  let {
    value = $bindable(),
    id = '',
    class: className = '',
    onchange,
  }: {
    value?: string;
    id?: string;
    class?: string;
    onchange?: (tag: string) => void;
  } = $props();

  const effectiveValue = $derived(value || 'en');
  const locales = getAvailableLocales();
  const listId = $derived(id ? `${id}-listbox` : 'locale-listbox');

  let open = $state(false);
  let highlightedIndex = $state(-1);
  let searchBuffer = $state('');
  let searchTimeout = $state<ReturnType<typeof setTimeout> | undefined>(undefined);
  let listRef = $state<HTMLUListElement | undefined>(undefined);
  let buttonRef = $state<HTMLButtonElement | undefined>(undefined);

  const selected = $derived(locales.find(l => l.tag === effectiveValue));

  function toggle() {
    open = !open;
    if (open) {
      highlightedIndex = locales.findIndex(l => l.tag === effectiveValue);
    }
  }

  function select(tag: string) {
    value = tag;
    open = false;
    buttonRef?.focus();
    onchange?.(tag);
  }

  function scrollToIndex(index: number) {
    if (!listRef) return;
    const item = listRef.children[index] as HTMLElement | undefined;
    item?.scrollIntoView({ block: 'nearest' });
  }

  function searchByChar(char: string) {
    clearTimeout(searchTimeout);
    searchBuffer += char.toLowerCase();
    searchTimeout = setTimeout(() => { searchBuffer = ''; }, 500);

    const startIndex = highlightedIndex >= 0 ? highlightedIndex : 0;
    for (let i = 0; i < locales.length; i++) {
      const idx = (startIndex + i) % locales.length;
      if (locales[idx].name.toLowerCase().startsWith(searchBuffer)) {
        highlightedIndex = idx;
        scrollToIndex(idx);
        return;
      }
    }
    if (searchBuffer.length > 1) {
      searchBuffer = char.toLowerCase();
      const nextStart = (highlightedIndex + 1) % locales.length;
      for (let i = 0; i < locales.length; i++) {
        const idx = (nextStart + i) % locales.length;
        if (locales[idx].name.toLowerCase().startsWith(searchBuffer)) {
          highlightedIndex = idx;
          scrollToIndex(idx);
          return;
        }
      }
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (!open) {
      if (e.key === 'Enter' || e.key === ' ' || e.key === 'ArrowDown') {
        e.preventDefault();
        open = true;
        highlightedIndex = locales.findIndex(l => l.tag === effectiveValue);
      }
      return;
    }

    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        highlightedIndex = Math.min(highlightedIndex + 1, locales.length - 1);
        scrollToIndex(highlightedIndex);
        break;
      case 'ArrowUp':
        e.preventDefault();
        highlightedIndex = Math.max(highlightedIndex - 1, 0);
        scrollToIndex(highlightedIndex);
        break;
      case 'Enter':
        e.preventDefault();
        if (highlightedIndex >= 0) select(locales[highlightedIndex].tag);
        break;
      case 'Escape':
        e.preventDefault();
        open = false;
        buttonRef?.focus();
        break;
      case 'Home':
        e.preventDefault();
        highlightedIndex = 0;
        scrollToIndex(0);
        break;
      case 'End':
        e.preventDefault();
        highlightedIndex = locales.length - 1;
        scrollToIndex(highlightedIndex);
        break;
      default:
        if (e.key.length === 1 && !e.ctrlKey && !e.metaKey) {
          e.preventDefault();
          searchByChar(e.key);
        }
    }
  }

  function handleClickOutside(e: MouseEvent) {
    const target = e.target as Node;
    if (!buttonRef?.contains(target) && !listRef?.contains(target)) {
      open = false;
    }
  }
</script>

<svelte:window onclick={handleClickOutside} />

<div class="relative {className}">
  <button
    bind:this={buttonRef}
    type="button"
    {id}
    role="combobox"
    aria-expanded={open}
    aria-controls={listId}
    aria-haspopup="listbox"
    class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary
           text-start flex items-center gap-2
           focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
    onclick={toggle}
    onkeydown={handleKeydown}
  >
    {#if selected}
      <span class="shrink-0">{selected.flag}</span>
      <span class="truncate">{selected.name}</span>
    {:else}
      <span class="text-text-muted">—</span>
    {/if}
    <svg class="w-4 h-4 ms-auto shrink-0 text-text-muted transition-transform {open ? 'rotate-180' : ''}" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
    </svg>
  </button>

  {#if open}
    <ul
      bind:this={listRef}
      id={listId}
      role="listbox"
      tabindex="-1"
      aria-activedescendant={highlightedIndex >= 0 ? `locale-opt-${locales[highlightedIndex].tag}` : undefined}
      class="absolute z-50 mt-1 w-full max-h-60 overflow-auto
             bg-bg-elevated border border-border-subtle rounded-md shadow-lg
             py-1 text-sm"
    >
      {#each locales as locale, i (locale.tag)}
        <li
          id="locale-opt-{locale.tag}"
          role="option"
          aria-selected={locale.tag === effectiveValue}
          class="flex items-center gap-2 px-3 py-1.5 cursor-pointer select-none
                 {i === highlightedIndex ? 'bg-brand-500/20 text-text-primary' : 'text-text-primary hover:bg-bg-surface'}
                 {locale.tag === effectiveValue ? 'font-medium' : ''}"
          onclick={() => select(locale.tag)}
          onkeydown={handleKeydown}
          onpointerenter={() => { highlightedIndex = i; }}
        >
          <span class="shrink-0">{locale.flag}</span>
          <span class="truncate">{locale.name}</span>
          {#if locale.tag === effectiveValue}
            <svg class="w-4 h-4 ms-auto shrink-0 text-brand-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
            </svg>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
</div>
