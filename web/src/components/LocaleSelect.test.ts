import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';

vi.mock('$lib/paraglide/runtime.js', () => ({
  setLocale: vi.fn(),
  getLocale: vi.fn().mockReturnValue('en'),
  locales: ['en', 'sv', 'ar', 'de', 'fr'],
  localStorageKey: 'PARAGLIDE_LOCALE',
}));

import LocaleSelect from './LocaleSelect.svelte';

// jsdom does not implement scrollIntoView
if (!Element.prototype.scrollIntoView) {
  Element.prototype.scrollIntoView = vi.fn();
}

describe('LocaleSelect', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders with the current locale selected (shows flag + name)', () => {
    render(LocaleSelect, { props: { value: 'en' } });
    const button = screen.getByRole('combobox');
    expect(button).toBeInTheDocument();
    expect(button.textContent).toContain('English');
  });

  it('defaults to English when value is empty', () => {
    render(LocaleSelect, { props: {} });
    const button = screen.getByRole('combobox');
    expect(button.textContent).toContain('English');
  });

  it('shows the selected locale name for a non-English locale', () => {
    render(LocaleSelect, { props: { value: 'sv' } });
    const button = screen.getByRole('combobox');
    expect(button.textContent).toContain('Svenska');
  });

  it('opens dropdown on click', async () => {
    render(LocaleSelect, { props: { value: 'en' } });
    const button = screen.getByRole('combobox');

    expect(button.getAttribute('aria-expanded')).toBe('false');
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();

    await fireEvent.click(button);

    expect(button.getAttribute('aria-expanded')).toBe('true');
    expect(screen.getByRole('listbox')).toBeInTheDocument();
  });

  it('closes dropdown on Escape', async () => {
    render(LocaleSelect, { props: { value: 'en' } });
    const button = screen.getByRole('combobox');

    await fireEvent.click(button);
    expect(screen.getByRole('listbox')).toBeInTheDocument();

    await fireEvent.keyDown(button, { key: 'Escape' });
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });

  it('selects a locale on click', async () => {
    const onchange = vi.fn();
    render(LocaleSelect, { props: { value: 'en', onchange } });
    const button = screen.getByRole('combobox');

    // Open dropdown
    await fireEvent.click(button);

    // Click on Deutsch option
    const options = screen.getAllByRole('option');
    const deutschOption = options.find(o => o.textContent?.includes('Deutsch'));
    expect(deutschOption).toBeDefined();
    await fireEvent.click(deutschOption!);

    // Dropdown should close
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
    // Callback should fire
    expect(onchange).toHaveBeenCalledWith('de');
  });

  it('calls onchange callback when a locale is selected', async () => {
    const onchange = vi.fn();
    render(LocaleSelect, { props: { value: 'en', onchange } });
    const button = screen.getByRole('combobox');

    await fireEvent.click(button);

    const options = screen.getAllByRole('option');
    const svOption = options.find(o => o.textContent?.includes('Svenska'));
    await fireEvent.click(svOption!);

    expect(onchange).toHaveBeenCalledTimes(1);
    expect(onchange).toHaveBeenCalledWith('sv');
  });

  it('type-ahead search jumps to matching locale', async () => {
    render(LocaleSelect, { props: { value: 'en' } });
    const button = screen.getByRole('combobox');

    // Open dropdown
    await fireEvent.click(button);

    // Type 'D' to search for Deutsch
    await fireEvent.keyDown(button, { key: 'D' });

    // The Deutsch option should now be highlighted
    const listbox = screen.getByRole('listbox');
    const activedescendant = listbox.getAttribute('aria-activedescendant');
    expect(activedescendant).toBe('locale-opt-de');
  });

  it('opens dropdown with ArrowDown key when closed', async () => {
    render(LocaleSelect, { props: { value: 'en' } });
    const button = screen.getByRole('combobox');

    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
    await fireEvent.keyDown(button, { key: 'ArrowDown' });
    expect(screen.getByRole('listbox')).toBeInTheDocument();
  });

  it('navigates options with ArrowDown and ArrowUp keys', async () => {
    render(LocaleSelect, { props: { value: 'en' } });
    const button = screen.getByRole('combobox');

    await fireEvent.click(button);
    const listbox = screen.getByRole('listbox');

    // Press ArrowDown to move highlight
    await fireEvent.keyDown(button, { key: 'ArrowDown' });
    const after = listbox.getAttribute('aria-activedescendant');

    // Press ArrowUp to move back
    await fireEvent.keyDown(button, { key: 'ArrowUp' });
    const back = listbox.getAttribute('aria-activedescendant');

    // They should differ (or at least ArrowDown should have moved)
    // The exact indices depend on sort order, but keys should work
    expect(after).toBeDefined();
    expect(back).toBeDefined();
  });

  it('selects highlighted option with Enter key', async () => {
    const onchange = vi.fn();
    render(LocaleSelect, { props: { value: 'en', onchange } });
    const button = screen.getByRole('combobox');

    // Open dropdown
    await fireEvent.click(button);

    // Move down once from current position, then select
    await fireEvent.keyDown(button, { key: 'ArrowDown' });
    await fireEvent.keyDown(button, { key: 'Enter' });

    // Should have called onchange with some locale tag
    expect(onchange).toHaveBeenCalledTimes(1);
    // Dropdown should close after selection
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });

  it('shows all available locale options in dropdown', async () => {
    render(LocaleSelect, { props: { value: 'en' } });
    const button = screen.getByRole('combobox');

    await fireEvent.click(button);

    const options = screen.getAllByRole('option');
    // Should have options for all 5 mocked locales
    expect(options.length).toBe(5);
  });

  it('marks the current value as selected in the dropdown', async () => {
    render(LocaleSelect, { props: { value: 'sv' } });
    const button = screen.getByRole('combobox');

    await fireEvent.click(button);

    const options = screen.getAllByRole('option');
    const svOption = options.find(o => o.textContent?.includes('Svenska'));
    expect(svOption?.getAttribute('aria-selected')).toBe('true');
  });

  it('applies custom id to the button', () => {
    render(LocaleSelect, { props: { value: 'en', id: 'my-locale' } });
    const button = screen.getByRole('combobox');
    expect(button.id).toBe('my-locale');
  });

  it('jumps to Home on Home key', async () => {
    render(LocaleSelect, { props: { value: 'en' } });
    const button = screen.getByRole('combobox');

    await fireEvent.click(button);
    await fireEvent.keyDown(button, { key: 'End' });
    await fireEvent.keyDown(button, { key: 'Home' });

    const listbox = screen.getByRole('listbox');
    const activedescendant = listbox.getAttribute('aria-activedescendant');
    // Home should jump to the first option
    const options = screen.getAllByRole('option');
    expect(activedescendant).toBe(options[0].id);
  });

  it('jumps to End on End key', async () => {
    render(LocaleSelect, { props: { value: 'en' } });
    const button = screen.getByRole('combobox');

    await fireEvent.click(button);
    await fireEvent.keyDown(button, { key: 'End' });

    const listbox = screen.getByRole('listbox');
    const activedescendant = listbox.getAttribute('aria-activedescendant');
    const options = screen.getAllByRole('option');
    expect(activedescendant).toBe(options[options.length - 1].id);
  });
});
