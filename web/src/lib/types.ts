export interface App {
  name: string;
  url: string;
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
}

export interface AppIcon {
  type: 'dashboard' | 'builtin' | 'custom' | 'url';
  name: string;
  file: string;
  url: string;
  variant: string;
}

export interface Group {
  name: string;
  icon: string;
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

export interface ProxyConfig {
  enabled: boolean;
  listen?: string;
  auto_https?: boolean;
  acme_email?: string;
  tls_cert?: string;
  tls_key?: string;
}

export interface Config {
  title: string;
  navigation: NavigationConfig;
  health?: HealthConfig;
  auth?: AuthConfig;
  proxy?: ProxyConfig;
  groups: Group[];
  apps: App[];
}
