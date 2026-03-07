import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import GeneralTab from './GeneralTab.svelte';
import type { Config, App } from '$lib/types';

function makeConfig(overrides: Partial<Config> = {}): Config {
  return {
    title: 'Test Dashboard',
    navigation: {
      position: 'top',
      width: '220px',
      auto_hide: false,
      auto_hide_delay: '0.5s',
      show_on_hover: true,
      show_labels: true,
      show_logo: true,
      show_app_colors: true,
      show_icon_background: false,
      icon_scale: 1,
      show_splash_on_startup: false,
      show_shadow: true,
      bar_style: 'grouped',
      floating_position: 'bottom-right',
      hide_sidebar_footer: false,
      max_open_tabs: 0,
    },
    groups: [],
    apps: [],
    ...overrides,
  };
}

function makeApp(_overrides: Partial<App> = {}): App {
  return {
    name: 'TestApp',
    url: 'http://localhost:8080',
    icon: { type: 'lucide', name: 'home', file: '', url: '', variant: '' },
    group: 'Default',
    proxy: false,
    open_mode: 'iframe',
    enabled: true,
    default: false,
    order: 0,
    health_check: false,
    color: '',
    scale: 1,
  } as App;
}

describe('GeneralTab', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders Dashboard Title input with correct value', () => {
    const config = makeConfig({ title: 'My Dashboard' });
    render(GeneralTab, {
      props: {
        localConfig: config,
        localApps: [],
        onexport: vi.fn(),
        onimportselect: vi.fn(),
      },
    });

    const input = screen.getByLabelText('Dashboard Title') as HTMLInputElement;
    expect(input).toBeInTheDocument();
    expect(input.value).toBe('My Dashboard');
  });

  it('renders Navigation Position selector with all positions', () => {
    const config = makeConfig();
    render(GeneralTab, {
      props: {
        localConfig: config,
        localApps: [],
        onexport: vi.fn(),
        onimportselect: vi.fn(),
      },
    });

    expect(screen.getByText('Navigation Position')).toBeInTheDocument();
    expect(screen.getByText('Top Bar')).toBeInTheDocument();
    expect(screen.getByText('Left Sidebar')).toBeInTheDocument();
    expect(screen.getByText('Right Sidebar')).toBeInTheDocument();
    expect(screen.getByText('Bottom Bar')).toBeInTheDocument();
    expect(screen.getByText('Floating')).toBeInTheDocument();
  });

  it('renders export and import buttons', () => {
    const config = makeConfig();
    render(GeneralTab, {
      props: {
        localConfig: config,
        localApps: [],
        onexport: vi.fn(),
        onimportselect: vi.fn(),
      },
    });

    expect(screen.getByText('Export Config')).toBeInTheDocument();
    expect(screen.getByText('Import Config')).toBeInTheDocument();
  });

  it('calls onexport when export button is clicked', async () => {
    const onexport = vi.fn();
    const config = makeConfig();
    render(GeneralTab, {
      props: {
        localConfig: config,
        localApps: [],
        onexport,
        onimportselect: vi.fn(),
      },
    });

    const exportBtn = screen.getByText('Export Config').closest('button')!;
    await fireEvent.click(exportBtn);
    expect(onexport).toHaveBeenCalledOnce();
  });

  it('changing title updates the input value', async () => {
    const config = makeConfig({ title: 'Old Title' });
    render(GeneralTab, {
      props: {
        localConfig: config,
        localApps: [],
        onexport: vi.fn(),
        onimportselect: vi.fn(),
      },
    });

    const input = screen.getByLabelText('Dashboard Title') as HTMLInputElement;
    await fireEvent.input(input, { target: { value: 'New Title' } });
    expect(input.value).toBe('New Title');
  });

  it('renders navigation option checkboxes', () => {
    const config = makeConfig();
    render(GeneralTab, {
      props: {
        localConfig: config,
        localApps: [],
        onexport: vi.fn(),
        onimportselect: vi.fn(),
      },
    });

    expect(screen.getByText('Show Labels')).toBeInTheDocument();
    expect(screen.getByText('Show Logo')).toBeInTheDocument();
    expect(screen.getByText('App Color Accents')).toBeInTheDocument();
    expect(screen.getByText('Icon Background')).toBeInTheDocument();
  });

  it('renders log level selector', () => {
    const config = makeConfig({ log_level: 'info' });
    render(GeneralTab, {
      props: {
        localConfig: config,
        localApps: [],
        onexport: vi.fn(),
        onimportselect: vi.fn(),
      },
    });

    expect(screen.getByLabelText('Log Level')).toBeInTheDocument();
  });

  it('renders health check bulk action buttons', () => {
    const config = makeConfig();
    render(GeneralTab, {
      props: {
        localConfig: config,
        localApps: [makeApp()],
        onexport: vi.fn(),
        onimportselect: vi.fn(),
      },
    });

    expect(screen.getByText('Health Checks')).toBeInTheDocument();
    expect(screen.getByText('Enable all')).toBeInTheDocument();
    expect(screen.getByText('Disable all')).toBeInTheDocument();
  });

  it('shows bar style options when position is top', () => {
    const config = makeConfig();
    config.navigation.position = 'top';
    render(GeneralTab, {
      props: {
        localConfig: config,
        localApps: [],
        onexport: vi.fn(),
        onimportselect: vi.fn(),
      },
    });

    expect(screen.getByText('Bar Style')).toBeInTheDocument();
    expect(screen.getByText('Group Dropdowns')).toBeInTheDocument();
    expect(screen.getByText('Flat List')).toBeInTheDocument();
  });

  it('shows floating position options when position is floating', async () => {
    const config = makeConfig();
    config.navigation.position = 'floating';
    render(GeneralTab, {
      props: {
        localConfig: config,
        localApps: [],
        onexport: vi.fn(),
        onimportselect: vi.fn(),
      },
    });

    expect(screen.getByText('Floating Button Position')).toBeInTheDocument();
    expect(screen.getByText('Bottom Right')).toBeInTheDocument();
    expect(screen.getByText('Bottom Left')).toBeInTheDocument();
    expect(screen.getByText('Top Right')).toBeInTheDocument();
    expect(screen.getByText('Top Left')).toBeInTheDocument();
  });

  describe('navigation position selection', () => {
    it('updates config when a different navigation position is clicked', async () => {
      const config = makeConfig();
      config.navigation.position = 'top';
      render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: [],
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      const leftSidebarBtn = screen.getByText('Left Sidebar').closest('button')!;
      await fireEvent.click(leftSidebarBtn);

      expect(config.navigation.position).toBe('left');
    });

    it('hides bar style when position is not top or bottom', () => {
      const config = makeConfig();
      config.navigation.position = 'left';
      render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: [],
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      expect(screen.queryByText('Bar Style')).not.toBeInTheDocument();
    });

    it('shows bar style when position is bottom', () => {
      const config = makeConfig();
      config.navigation.position = 'bottom';
      render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: [],
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      expect(screen.getByText('Bar Style')).toBeInTheDocument();
    });

    it('hides floating position when position is not floating', () => {
      const config = makeConfig();
      config.navigation.position = 'top';
      render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: [],
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      expect(screen.queryByText('Floating Button Position')).not.toBeInTheDocument();
    });
  });

  describe('bar style selection', () => {
    it('updates bar_style to flat when Flat List is clicked', async () => {
      const config = makeConfig();
      config.navigation.position = 'top';
      config.navigation.bar_style = 'grouped';
      render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: [],
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      const flatBtn = screen.getByText('Flat List').closest('button')!;
      await fireEvent.click(flatBtn);

      expect(config.navigation.bar_style).toBe('flat');
    });

    it('updates bar_style to grouped when Group Dropdowns is clicked', async () => {
      const config = makeConfig();
      config.navigation.position = 'top';
      config.navigation.bar_style = 'flat';
      render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: [],
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      const groupedBtn = screen.getByText('Group Dropdowns').closest('button')!;
      await fireEvent.click(groupedBtn);

      expect(config.navigation.bar_style).toBe('grouped');
    });
  });

  describe('floating position selection', () => {
    it('updates floating_position when a position button is clicked', async () => {
      const config = makeConfig();
      config.navigation.position = 'floating';
      config.navigation.floating_position = 'bottom-right';
      render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: [],
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      const topLeftBtn = screen.getByText('Top Left').closest('button')!;
      await fireEvent.click(topLeftBtn);

      expect(config.navigation.floating_position).toBe('top-left');
    });
  });

  describe('checkbox toggles', () => {
    it('toggles show_labels checkbox', async () => {
      const config = makeConfig();
      config.navigation.show_labels = true;
      render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: [],
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      const showLabelsCheckbox = screen.getByText('Show Labels').closest('label')!.querySelector('input[type="checkbox"]') as HTMLInputElement;
      await fireEvent.click(showLabelsCheckbox);

      expect(showLabelsCheckbox.checked).toBe(false);
    });

    it('toggles show_logo checkbox', async () => {
      const config = makeConfig();
      config.navigation.show_logo = true;
      render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: [],
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      const showLogoCheckbox = screen.getByText('Show Logo').closest('label')!.querySelector('input[type="checkbox"]') as HTMLInputElement;
      await fireEvent.click(showLogoCheckbox);

      expect(showLogoCheckbox.checked).toBe(false);
    });

    it('toggles show_app_colors checkbox', async () => {
      const config = makeConfig();
      config.navigation.show_app_colors = false;
      render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: [],
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      const appColorsCheckbox = screen.getByText('App Color Accents').closest('label')!.querySelector('input[type="checkbox"]') as HTMLInputElement;
      await fireEvent.click(appColorsCheckbox);

      expect(appColorsCheckbox.checked).toBe(true);
    });

    it('toggles show_icon_background checkbox', async () => {
      const config = makeConfig();
      config.navigation.show_icon_background = false;
      render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: [],
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      const iconBgCheckbox = screen.getByText('Icon Background').closest('label')!.querySelector('input[type="checkbox"]') as HTMLInputElement;
      await fireEvent.click(iconBgCheckbox);

      expect(iconBgCheckbox.checked).toBe(true);
    });
  });

  describe('auto-hide menu', () => {
    it('shows auto-hide delay selector when auto_hide is enabled', () => {
      const config = makeConfig();
      config.navigation.auto_hide = true;
      render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: [],
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      expect(screen.getByText('Hide after')).toBeInTheDocument();
      // The delay select should have the current value
      const delaySelect = screen.getByDisplayValue('0.5s');
      expect(delaySelect).toBeInTheDocument();
    });

    it('hides auto-hide delay selector when auto_hide is disabled', () => {
      const config = makeConfig();
      config.navigation.auto_hide = false;
      render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: [],
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      expect(screen.queryByText('Hide after')).not.toBeInTheDocument();
    });

    it('shows shadow checkbox when auto_hide is enabled', () => {
      const config = makeConfig();
      config.navigation.auto_hide = true;
      render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: [],
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      expect(screen.getByText(/Shadow/)).toBeInTheDocument();
    });

    it('changes auto_hide_delay when a different delay is selected', async () => {
      const config = makeConfig();
      config.navigation.auto_hide = true;
      config.navigation.auto_hide_delay = '0.5s';
      render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: [],
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      const delaySelect = screen.getByDisplayValue('0.5s') as HTMLSelectElement;
      await fireEvent.change(delaySelect, { target: { value: '2s' } });

      expect(delaySelect.value).toBe('2s');
    });
  });

  describe('icon scale slider', () => {
    it('renders icon scale slider with current value', () => {
      const config = makeConfig();
      config.navigation.icon_scale = 1;
      render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: [],
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      expect(screen.getByText('Icon Size')).toBeInTheDocument();
      // The current scale value should be displayed
      expect(screen.getByText(/1×/)).toBeInTheDocument();
    });

    it('shows the icon scale value', () => {
      const config = makeConfig();
      config.navigation.icon_scale = 1.5;
      render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: [],
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      expect(screen.getByText('1.5×')).toBeInTheDocument();
    });
  });

  describe('splash on startup toggle', () => {
    it('renders start on overview checkbox', () => {
      const config = makeConfig();
      render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: [],
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      expect(screen.getByText('Start on Overview')).toBeInTheDocument();
    });

    it('clears default app when splash on startup is enabled', async () => {
      const config = makeConfig();
      config.navigation.show_splash_on_startup = false;
      const apps = [
        makeApp({ name: 'DefaultApp', default: true }),
        makeApp({ name: 'OtherApp', default: false }),
      ];

      render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: apps,
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      const splashCheckbox = screen.getByText('Start on Overview').closest('label')!.querySelector('input[type="checkbox"]') as HTMLInputElement;
      await fireEvent.click(splashCheckbox);

      // After enabling splash, all apps should have default = false
      expect(apps[0].default).toBe(false);
      expect(apps[1].default).toBe(false);
    });

    it('does not show default app hint when no app is set as default', () => {
      const config = makeConfig();
      config.navigation.show_splash_on_startup = false;
      const apps = [
        makeApp({ name: 'NoDefault', default: false }),
      ];

      const { container } = render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: apps,
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      expect(container.textContent).not.toContain('default startup app');
    });

    it('does not show default app message when splash is enabled', () => {
      const config = makeConfig();
      config.navigation.show_splash_on_startup = true;
      const apps = [
        makeApp({ name: 'MyDefault', default: true }),
      ];

      render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: apps,
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      expect(screen.queryByText(/MyDefault is set as the default startup app/)).not.toBeInTheDocument();
    });
  });

  describe('collapsible footer (sidebar only)', () => {
    it('shows collapsible footer option when position is left', () => {
      const config = makeConfig();
      config.navigation.position = 'left';
      render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: [],
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      expect(screen.getByText('Collapsible Footer')).toBeInTheDocument();
    });

    it('shows collapsible footer option when position is right', () => {
      const config = makeConfig();
      config.navigation.position = 'right';
      render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: [],
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      expect(screen.getByText('Collapsible Footer')).toBeInTheDocument();
    });

    it('hides collapsible footer option when position is top', () => {
      const config = makeConfig();
      config.navigation.position = 'top';
      render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: [],
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      expect(screen.queryByText('Collapsible Footer')).not.toBeInTheDocument();
    });

    it('hides collapsible footer option when position is floating', () => {
      const config = makeConfig();
      config.navigation.position = 'floating';
      render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: [],
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      expect(screen.queryByText('Collapsible Footer')).not.toBeInTheDocument();
    });
  });

  describe('health check bulk actions', () => {
    it('enables health_check on all apps when "Enable all" is clicked', async () => {
      const config = makeConfig();
      const apps = [
        makeApp({ name: 'App1', health_check: false }),
        makeApp({ name: 'App2', health_check: undefined }),
      ];

      render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: apps,
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      const enableAllBtn = screen.getByText('Enable all');
      await fireEvent.click(enableAllBtn);

      expect(apps[0].health_check).toBe(true);
      expect(apps[1].health_check).toBe(true);
    });

    it('disables health_check on all apps when "Disable all" is clicked', async () => {
      const config = makeConfig();
      const apps = [
        makeApp({ name: 'App1', health_check: true }),
        makeApp({ name: 'App2', health_check: true }),
      ];

      render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: apps,
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      const disableAllBtn = screen.getByText('Disable all');
      await fireEvent.click(disableAllBtn);

      expect(apps[0].health_check).toBeUndefined();
      expect(apps[1].health_check).toBeUndefined();
    });
  });

  describe('log level selector', () => {
    it('changes log level when a different option is selected', async () => {
      const config = makeConfig({ log_level: 'info' });
      render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: [],
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      const logSelect = screen.getByLabelText('Log Level') as HTMLSelectElement;
      await fireEvent.change(logSelect, { target: { value: 'debug' } });

      expect(logSelect.value).toBe('debug');
    });
  });

  describe('proxy timeout', () => {
    it('renders proxy timeout input', () => {
      const config = makeConfig({ proxy_timeout: '30s' });
      render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: [],
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      const input = screen.getByLabelText('Proxy Timeout') as HTMLInputElement;
      expect(input).toBeInTheDocument();
      expect(input.value).toBe('30s');
    });

    it('updates proxy timeout on input change', async () => {
      const config = makeConfig({ proxy_timeout: '30s' });
      render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: [],
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      const input = screen.getByLabelText('Proxy Timeout') as HTMLInputElement;
      await fireEvent.input(input, { target: { value: '1m' } });

      expect(input.value).toBe('1m');
    });
  });

  describe('import config', () => {
    it('triggers file input click when Import Config button is clicked', async () => {
      const config = makeConfig();
      const { container } = render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: [],
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;
      const clickSpy = vi.spyOn(fileInput, 'click');

      const importBtn = screen.getByText('Import Config').closest('button')!;
      await fireEvent.click(importBtn);

      expect(clickSpy).toHaveBeenCalled();
    });

    it('calls onimportselect when a file is selected', async () => {
      const onimportselect = vi.fn();
      const config = makeConfig();
      const { container } = render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: [],
          onexport: vi.fn(),
          onimportselect,
        },
      });

      const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;
      await fireEvent.change(fileInput);

      expect(onimportselect).toHaveBeenCalledTimes(1);
    });

    it('accepts .yaml and .yml file types', () => {
      const config = makeConfig();
      const { container } = render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: [],
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;
      expect(fileInput.accept).toBe('.yaml,.yml');
    });
  });

  describe('template variables hint', () => {
    it('shows template variable hints for title', () => {
      const config = makeConfig();
      render(GeneralTab, {
        props: {
          localConfig: config,
          localApps: [],
          onexport: vi.fn(),
          onimportselect: vi.fn(),
        },
      });

      const hints = screen.getAllByText((_content, element) => {
        if (element?.tagName !== 'P') return false;
        const text = element?.textContent || '';
        return text.includes('%title%') && text.includes('%group%') && text.includes('%version%') && text.includes('%url%');
      });
      expect(hints.length).toBeGreaterThan(0);
    });
  });
});
