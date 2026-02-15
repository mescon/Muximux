import type { App } from './types';

export interface PopularAppTemplate {
  name: string;
  defaultUrl: string;
  icon: string;  // dashboard-icons name or custom icon filename
  iconType?: 'dashboard' | 'custom';  // defaults to 'dashboard'
  color: string;
  iconBackground: string;  // Dark contrasting background for icon square
  group: string;
  description: string;
}

// Pre-defined homelab app templates with icons from dashboard-icons
// Icons are sourced from: https://github.com/homarr-labs/dashboard-icons
export const popularApps: Record<string, PopularAppTemplate[]> = {
  'Media': [
    {
      name: 'Plex',
      defaultUrl: 'http://localhost:32400/web',
      icon: 'plex',
      color: '#E5A00D',
      iconBackground: '#2D2200',
      group: 'Media',
      description: 'Stream your media library'
    },
    {
      name: 'Jellyfin',
      defaultUrl: 'http://localhost:8096',
      icon: 'jellyfin',
      color: '#00A4DC',
      iconBackground: '#0D1F3D',
      group: 'Media',
      description: 'Free media server'
    },
    {
      name: 'Emby',
      defaultUrl: 'http://localhost:8096',
      icon: 'emby',
      color: '#52B54B',
      iconBackground: '#0D2E1A',
      group: 'Media',
      description: 'Media streaming server'
    },
    {
      name: 'Tautulli',
      defaultUrl: 'http://localhost:8181',
      icon: 'tautulli',
      color: '#E5A00D',
      iconBackground: '#2D2200',
      group: 'Media',
      description: 'Plex monitoring & statistics'
    },
    {
      name: 'Stash',
      defaultUrl: 'http://localhost:9999',
      icon: 'stash',
      color: '#1A1A2E',
      iconBackground: '#0D0D1A',
      group: 'Media',
      description: 'Media organizer & manager'
    },
    {
      name: 'Overseerr',
      defaultUrl: 'http://localhost:5055',
      icon: 'overseerr',
      color: '#7B2BF9',
      iconBackground: '#150D2E',
      group: 'Media',
      description: 'Media request management'
    },
    {
      name: 'Navidrome',
      defaultUrl: 'http://localhost:4533',
      icon: 'navidrome',
      color: '#0091EA',
      iconBackground: '#0D1F3D',
      group: 'Media',
      description: 'Personal music streaming'
    },
    {
      name: 'Jellyseerr',
      defaultUrl: 'http://localhost:5055',
      icon: 'jellyseerr',
      color: '#763DCD',
      iconBackground: '#150D2E',
      group: 'Media',
      description: 'Media requests for Jellyfin'
    },
    {
      name: 'Audiobookshelf',
      defaultUrl: 'http://localhost:13378',
      icon: 'audiobookshelf',
      color: '#875D27',
      iconBackground: '#2D1800',
      group: 'Media',
      description: 'Audiobook & podcast server'
    },
    {
      name: 'Kavita',
      defaultUrl: 'http://localhost:5000',
      icon: 'kavita',
      color: '#4AC694',
      iconBackground: '#0D2E1A',
      group: 'Media',
      description: 'Comics, manga & ebook reader'
    },
    {
      name: 'Komga',
      defaultUrl: 'http://localhost:25600',
      icon: 'komga',
      color: '#005ED3',
      iconBackground: '#0D1F3D',
      group: 'Media',
      description: 'Comics & manga media server'
    },
    {
      name: 'Calibre-Web',
      defaultUrl: 'http://localhost:8083',
      icon: 'calibre-web',
      color: '#45B29D',
      iconBackground: '#0D2E1A',
      group: 'Media',
      description: 'Ebook library & reader'
    }
  ],

  'Downloads': [
    {
      name: 'Sonarr',
      defaultUrl: 'http://localhost:8989',
      icon: 'sonarr',
      color: '#00CCFF',
      iconBackground: '#0D2633',
      group: 'Downloads',
      description: 'TV show management'
    },
    {
      name: 'Radarr',
      defaultUrl: 'http://localhost:7878',
      icon: 'radarr',
      color: '#FFC230',
      iconBackground: '#2D2200',
      group: 'Downloads',
      description: 'Movie management'
    },
    {
      name: 'Lidarr',
      defaultUrl: 'http://localhost:8686',
      icon: 'lidarr',
      color: '#00E087',
      iconBackground: '#0D2E1A',
      group: 'Downloads',
      description: 'Music management'
    },
    {
      name: 'Whisparr',
      defaultUrl: 'http://localhost:6969',
      icon: 'whisparr',
      color: '#E0528B',
      iconBackground: '#2D0C1A',
      group: 'Downloads',
      description: 'Adult content management'
    },
    {
      name: 'Bazarr',
      defaultUrl: 'http://localhost:6767',
      icon: 'bazarr',
      color: '#4FC3F7',
      iconBackground: '#0D2633',
      group: 'Downloads',
      description: 'Subtitle management'
    },
    {
      name: 'Prowlarr',
      defaultUrl: 'http://localhost:9696',
      icon: 'prowlarr',
      color: '#FFC230',
      iconBackground: '#2D2200',
      group: 'Downloads',
      description: 'Indexer management'
    },
    {
      name: 'qBittorrent',
      defaultUrl: 'http://localhost:8080',
      icon: 'qbittorrent',
      color: '#2F67BA',
      iconBackground: '#0D1F3D',
      group: 'Downloads',
      description: 'Torrent client'
    },
    {
      name: 'SABnzbd',
      defaultUrl: 'http://localhost:8080',
      icon: 'sabnzbd',
      color: '#FDC624',
      iconBackground: '#2D2200',
      group: 'Downloads',
      description: 'Usenet downloader'
    },
    {
      name: 'NZBGet',
      defaultUrl: 'http://localhost:6789',
      icon: 'nzbget',
      color: '#333333',
      iconBackground: '#1A1A1A',
      group: 'Downloads',
      description: 'Usenet downloader'
    },
    {
      name: 'Transmission',
      defaultUrl: 'http://localhost:9091',
      icon: 'transmission',
      color: '#B50D0D',
      iconBackground: '#2D0C07',
      group: 'Downloads',
      description: 'Torrent client'
    },
    {
      name: 'Deluge',
      defaultUrl: 'http://localhost:8112',
      icon: 'deluge',
      color: '#2B5B9E',
      iconBackground: '#0D1F3D',
      group: 'Downloads',
      description: 'Torrent client'
    },
    {
      name: 'RDTClient',
      defaultUrl: 'http://localhost:6500',
      icon: 'rdt-client',
      color: '#5C6BC0',
      iconBackground: '#150D2E',
      group: 'Downloads',
      description: 'Real-Debrid torrent client'
    },
    {
      name: 'Huntarr',
      defaultUrl: 'http://localhost:9705',
      icon: 'huntarr',
      color: '#FF6B35',
      iconBackground: '#2D1800',
      group: 'Downloads',
      description: 'Missing media hunter'
    },
    {
      name: 'Readarr',
      defaultUrl: 'http://localhost:8787',
      icon: 'readarr',
      color: '#8E2222',
      iconBackground: '#2D0C07',
      group: 'Downloads',
      description: 'Book management'
    }
  ],

  'System': [
    {
      name: 'Portainer',
      defaultUrl: 'http://localhost:9000',
      icon: 'portainer',
      color: '#13BEF9',
      iconBackground: '#0D2633',
      group: 'System',
      description: 'Docker management'
    },
    {
      name: 'Proxmox',
      defaultUrl: 'https://localhost:8006',
      icon: 'proxmox',
      color: '#E57000',
      iconBackground: '#2D1800',
      group: 'System',
      description: 'Virtualization platform'
    },
    {
      name: 'Unraid',
      defaultUrl: 'http://localhost',
      icon: 'unraid',
      color: '#F15A2B',
      iconBackground: '#2D0C07',
      group: 'System',
      description: 'NAS & server OS'
    },
    {
      name: 'TrueNAS',
      defaultUrl: 'http://localhost',
      icon: 'truenas',
      color: '#0095D5',
      iconBackground: '#0D1F3D',
      group: 'System',
      description: 'Storage management'
    },
    {
      name: 'Home Assistant',
      defaultUrl: 'http://localhost:8123',
      icon: 'home-assistant',
      color: '#41BDF5',
      iconBackground: '#0D1F3D',
      group: 'System',
      description: 'Home automation'
    },
    {
      name: 'Pi-hole',
      defaultUrl: 'http://localhost/admin',
      icon: 'pi-hole',
      color: '#96060C',
      iconBackground: '#2D0C07',
      group: 'System',
      description: 'Network-wide ad blocker'
    },
    {
      name: 'AdGuard Home',
      defaultUrl: 'http://localhost:3000',
      icon: 'adguard-home',
      color: '#67B279',
      iconBackground: '#0D2E1A',
      group: 'System',
      description: 'DNS-based ad blocker'
    },
    {
      name: 'Nginx Proxy Manager',
      defaultUrl: 'http://localhost:81',
      icon: 'nginx-proxy-manager',
      color: '#F15833',
      iconBackground: '#2D1800',
      group: 'System',
      description: 'Reverse proxy management'
    },
    {
      name: 'Traefik',
      defaultUrl: 'http://localhost:8080',
      icon: 'traefik',
      color: '#24A1C1',
      iconBackground: '#0D2633',
      group: 'System',
      description: 'Edge router & proxy'
    },
    {
      name: 'Grafana',
      defaultUrl: 'http://localhost:3000',
      icon: 'grafana',
      color: '#F46800',
      iconBackground: '#2D1800',
      group: 'System',
      description: 'Metrics visualization'
    },
    {
      name: 'Prometheus',
      defaultUrl: 'http://localhost:9090',
      icon: 'prometheus',
      color: '#E6522C',
      iconBackground: '#2D0C07',
      group: 'System',
      description: 'Metrics collection'
    },
    {
      name: 'Uptime Kuma',
      defaultUrl: 'http://localhost:3001',
      icon: 'uptime-kuma',
      color: '#5CDD8B',
      iconBackground: '#0D2E1A',
      group: 'System',
      description: 'Status monitoring'
    },
    {
      name: 'Frigate',
      defaultUrl: 'http://localhost:8971',
      icon: 'frigate',
      color: '#2196F3',
      iconBackground: '#0D1F3D',
      group: 'System',
      description: 'NVR & camera system'
    },
    {
      name: 'n8n',
      defaultUrl: 'http://localhost:5678',
      icon: 'n8n',
      color: '#EA4B71',
      iconBackground: '#2D0C1A',
      group: 'System',
      description: 'Workflow automation'
    },
    {
      name: 'Tdarr',
      defaultUrl: 'http://localhost:8265',
      icon: 'tdarr',
      color: '#6EC6FF',
      iconBackground: '#0D2633',
      group: 'System',
      description: 'Media transcoding'
    },
    {
      name: 'Guacamole',
      defaultUrl: 'http://localhost:8080/guacamole',
      icon: 'guacamole',
      color: '#3F8E4F',
      iconBackground: '#0D2E1A',
      group: 'System',
      description: 'Remote desktop gateway'
    },
    {
      name: 'Headplane',
      defaultUrl: 'http://localhost:3000',
      icon: 'headscale',
      color: '#4A90D9',
      iconBackground: '#0D1F3D',
      group: 'System',
      description: 'Headscale web UI'
    },
    {
      name: 'Arcane',
      defaultUrl: 'http://localhost:3552',
      icon: 'arcane',
      color: '#7C4DFF',
      iconBackground: '#150D2E',
      group: 'System',
      description: 'Docker management'
    },
    {
      name: 'Healarr',
      defaultUrl: 'http://localhost:3090',
      icon: 'healarr',
      iconType: 'custom',
      color: '#4CAF50',
      iconBackground: '#0D2E1A',
      group: 'System',
      description: 'Health monitor for *arr'
    },
    {
      name: 'Profilarr',
      defaultUrl: 'http://localhost:6868',
      icon: 'profilarr',
      color: '#FF7043',
      iconBackground: '#2D1800',
      group: 'System',
      description: 'Profile sync for *arr'
    },
    {
      name: 'Agregarr',
      defaultUrl: 'http://localhost:7171',
      icon: 'agregarr',
      color: '#AB47BC',
      iconBackground: '#150D2E',
      group: 'System',
      description: 'Plex collections manager'
    },
    {
      name: 'Authentik',
      defaultUrl: 'http://localhost:9000',
      icon: 'authentik',
      color: '#FD4B2D',
      iconBackground: '#2D0C07',
      group: 'System',
      description: 'Identity provider & SSO'
    },
    {
      name: 'Authelia',
      defaultUrl: 'http://localhost:9091',
      icon: 'authelia',
      color: '#3F51B4',
      iconBackground: '#0D1F3D',
      group: 'System',
      description: 'Authentication & 2FA portal'
    },
    {
      name: 'Tailscale',
      defaultUrl: 'https://login.tailscale.com/admin',
      icon: 'tailscale',
      color: '#242424',
      iconBackground: '#1A1A1A',
      group: 'System',
      description: 'Mesh VPN'
    },
    {
      name: 'WireGuard',
      defaultUrl: 'http://localhost:51821',
      icon: 'wireguard',
      color: '#88171A',
      iconBackground: '#2D0C07',
      group: 'System',
      description: 'VPN server'
    },
    {
      name: 'Watchtower',
      defaultUrl: 'http://localhost:8080',
      icon: 'watchtower',
      color: '#003343',
      iconBackground: '#0D2633',
      group: 'System',
      description: 'Automatic Docker updates'
    },
    {
      name: 'CrowdSec',
      defaultUrl: 'http://localhost:8080',
      icon: 'crowdsec',
      color: '#4E4A99',
      iconBackground: '#150D2E',
      group: 'System',
      description: 'Collaborative intrusion prevention'
    },
    {
      name: 'Dozzle',
      defaultUrl: 'http://localhost:8080',
      icon: 'dozzle',
      color: '#F5A623',
      iconBackground: '#2D2200',
      group: 'System',
      description: 'Real-time Docker log viewer'
    },
    {
      name: 'Glances',
      defaultUrl: 'http://localhost:61208',
      icon: 'glances',
      color: '#57CB6A',
      iconBackground: '#0D2E1A',
      group: 'System',
      description: 'System monitoring dashboard'
    },
    {
      name: 'Netdata',
      defaultUrl: 'http://localhost:19999',
      icon: 'netdata',
      color: '#00AB44',
      iconBackground: '#0D2E1A',
      group: 'System',
      description: 'Real-time performance monitoring'
    }
  ],

  'Utilities': [
    {
      name: 'Vaultwarden',
      defaultUrl: 'http://localhost:8080',
      icon: 'vaultwarden',
      color: '#175DDC',
      iconBackground: '#0D1F3D',
      group: 'Utilities',
      description: 'Password manager'
    },
    {
      name: 'Nextcloud',
      defaultUrl: 'http://localhost',
      icon: 'nextcloud',
      color: '#0082C9',
      iconBackground: '#0D1F3D',
      group: 'Utilities',
      description: 'Cloud storage & productivity'
    },
    {
      name: 'Photoprism',
      defaultUrl: 'http://localhost:2342',
      icon: 'photoprism',
      color: '#9C27B0',
      iconBackground: '#150D2E',
      group: 'Utilities',
      description: 'Photo management'
    },
    {
      name: 'Immich',
      defaultUrl: 'http://localhost:2283',
      icon: 'immich',
      color: '#4250AF',
      iconBackground: '#150D2E',
      group: 'Utilities',
      description: 'Photo & video backup'
    },
    {
      name: 'Paperless-ngx',
      defaultUrl: 'http://localhost:8000',
      icon: 'paperless-ngx',
      color: '#17541F',
      iconBackground: '#8BC34A',
      group: 'Utilities',
      description: 'Document management'
    },
    {
      name: 'Gitea',
      defaultUrl: 'http://localhost:3000',
      icon: 'gitea',
      color: '#609926',
      iconBackground: '#0D2E1A',
      group: 'Utilities',
      description: 'Git server'
    },
    {
      name: 'Code Server',
      defaultUrl: 'http://localhost:8443',
      icon: 'vscode',
      color: '#007ACC',
      iconBackground: '#0D1F3D',
      group: 'Utilities',
      description: 'VS Code in the browser'
    },
    {
      name: 'Syncthing',
      defaultUrl: 'http://localhost:8384',
      icon: 'syncthing',
      color: '#0891D1',
      iconBackground: '#0D1F3D',
      group: 'Utilities',
      description: 'File synchronization'
    },
    {
      name: 'Mealie',
      defaultUrl: 'http://localhost:9000',
      icon: 'mealie',
      color: '#E58325',
      iconBackground: '#2D1800',
      group: 'Utilities',
      description: 'Recipe manager'
    },
    {
      name: 'Bookstack',
      defaultUrl: 'http://localhost:6875',
      icon: 'bookstack',
      color: '#0288D1',
      iconBackground: '#0D1F3D',
      group: 'Utilities',
      description: 'Documentation wiki'
    },
    {
      name: 'Wiki.js',
      defaultUrl: 'http://localhost:3000',
      icon: 'wikijs',
      color: '#02BEF3',
      iconBackground: '#0D2633',
      group: 'Utilities',
      description: 'Modern documentation wiki'
    },
    {
      name: 'Stirling-PDF',
      defaultUrl: 'http://localhost:8080',
      icon: 'stirling-pdf',
      color: '#8E3131',
      iconBackground: '#2D0C07',
      group: 'Utilities',
      description: 'PDF tools & manipulation'
    },
    {
      name: 'IT-Tools',
      defaultUrl: 'http://localhost:8080',
      icon: 'it-tools',
      color: '#18A058',
      iconBackground: '#0D2E1A',
      group: 'Utilities',
      description: 'Developer & sysadmin toolkit'
    },
    {
      name: 'Excalidraw',
      defaultUrl: 'http://localhost:3000',
      icon: 'excalidraw',
      color: '#6965DB',
      iconBackground: '#150D2E',
      group: 'Utilities',
      description: 'Collaborative whiteboard'
    },
    {
      name: 'Changedetection.io',
      defaultUrl: 'http://localhost:5000',
      icon: 'changedetection',
      color: '#3056D3',
      iconBackground: '#0D1F3D',
      group: 'Utilities',
      description: 'Website change monitoring'
    },
    {
      name: 'FreshRSS',
      defaultUrl: 'http://localhost:8080',
      icon: 'freshrss',
      color: '#0062BE',
      iconBackground: '#0D1F3D',
      group: 'Utilities',
      description: 'RSS feed aggregator'
    },
    {
      name: 'Linkding',
      defaultUrl: 'http://localhost:9090',
      icon: 'linkding',
      color: '#5856E0',
      iconBackground: '#150D2E',
      group: 'Utilities',
      description: 'Bookmark manager'
    }
  ],

  'AI': [
    {
      name: 'Ollama',
      defaultUrl: 'http://localhost:11434',
      icon: 'ollama',
      color: '#000000',
      iconBackground: '#1A1A1A',
      group: 'AI',
      description: 'Local LLM inference server'
    },
    {
      name: 'Open WebUI',
      defaultUrl: 'http://localhost:3000',
      icon: 'open-webui',
      color: '#000000',
      iconBackground: '#1A1A1A',
      group: 'AI',
      description: 'Chat interface for local LLMs'
    }
  ]
};

// Get all apps as a flat list
export function getAllPopularApps(): PopularAppTemplate[] {
  return Object.values(popularApps).flat();
}

// Get all group names
export function getAllGroups(): string[] {
  return Object.keys(popularApps);
}

// Convert a template to an App object
export function templateToApp(template: PopularAppTemplate, url: string, order: number): App {
  return {
    name: template.name,
    url: url || template.defaultUrl,
    icon: {
      type: template.iconType || 'dashboard',
      name: template.icon,
      file: '',
      url: '',
      variant: 'svg',
      background: template.iconBackground
    },
    color: template.color,
    group: template.group,
    order,
    enabled: true,
    default: order === 0,  // First app is default
    open_mode: 'iframe',
    proxy: false,
    scale: 1,
    disable_keyboard_shortcuts: false
  };
}
