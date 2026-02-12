export interface App {
  name: string;
  url: string;
  proxyUrl?: string;  // Proxy path for iframe loading (when proxy enabled)
  health_url?: string;
  icon: AppIcon;
  color: string;
  group: string;
  order: number;
  enabled: boolean;
  default: boolean;
  open_mode: 'iframe' | 'new_tab' | 'new_window' | 'redirect';
  proxy: boolean;
  scale: number;
  disable_keyboard_shortcuts: boolean;
}

export function getEffectiveUrl(app: App): string {
  return app.proxyUrl || app.url;
}

export interface AppIcon {
  type: 'dashboard' | 'lucide' | 'custom' | 'url';
  name: string;
  file: string;
  url: string;
  variant: string;
  color?: string;
  background?: string;
}

export interface Group {
  name: string;
  icon: AppIcon;
  color: string;
  order: number;
  expanded: boolean;
}

export interface NavigationConfig {
  position: 'top' | 'left' | 'right' | 'bottom' | 'floating';
  width: string;
  auto_hide: boolean;
  auto_hide_delay: string;
  show_on_hover: boolean;
  show_labels: boolean;
  show_logo: boolean;
  show_app_colors: boolean;
  show_icon_background: boolean;
  show_splash_on_startup: boolean;
  show_shadow: boolean;
}

export interface HealthConfig {
  enabled: boolean;
  interval: string;
  timeout: string;
}

export interface AuthConfig {
  method: 'none' | 'builtin' | 'forward_auth' | 'oidc';
  session_max_age?: string;
  secure_cookies?: boolean;
}

export interface TLSConfig {
  domain?: string;
  email?: string;
  cert?: string;
  key?: string;
}

export interface KeyCombo {
  key: string;
  ctrl?: boolean;
  alt?: boolean;
  shift?: boolean;
  meta?: boolean;
}

export interface KeybindingsConfig {
  bindings?: Record<string, KeyCombo[]>;
}

export interface ThemeConfig {
  family: string;
  variant: 'dark' | 'light' | 'system';
}

export interface Config {
  title: string;
  navigation: NavigationConfig;
  theme?: ThemeConfig;
  health?: HealthConfig;
  auth?: AuthConfig;
  tls?: TLSConfig;
  gateway?: string;
  keybindings?: KeybindingsConfig;
  groups: Group[];
  apps: App[];
}
