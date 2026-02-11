import type { App } from './types';

export interface PopularAppTemplate {
  name: string;
  defaultUrl: string;
  icon: string;  // dashboard-icons name
  color: string;
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
      group: 'Media',
      description: 'Stream your media library'
    },
    {
      name: 'Jellyfin',
      defaultUrl: 'http://localhost:8096',
      icon: 'jellyfin',
      color: '#00A4DC',
      group: 'Media',
      description: 'Free media server'
    },
    {
      name: 'Emby',
      defaultUrl: 'http://localhost:8096',
      icon: 'emby',
      color: '#52B54B',
      group: 'Media',
      description: 'Media streaming server'
    },
    {
      name: 'Tautulli',
      defaultUrl: 'http://localhost:8181',
      icon: 'tautulli',
      color: '#E5A00D',
      group: 'Media',
      description: 'Plex monitoring & statistics'
    },
    {
      name: 'Overseerr',
      defaultUrl: 'http://localhost:5055',
      icon: 'overseerr',
      color: '#7B2BF9',
      group: 'Media',
      description: 'Media request management'
    },
    {
      name: 'Navidrome',
      defaultUrl: 'http://localhost:4533',
      icon: 'navidrome',
      color: '#0091EA',
      group: 'Media',
      description: 'Personal music streaming'
    }
  ],

  'Downloads': [
    {
      name: 'Sonarr',
      defaultUrl: 'http://localhost:8989',
      icon: 'sonarr',
      color: '#00CCFF',
      group: 'Downloads',
      description: 'TV show management'
    },
    {
      name: 'Radarr',
      defaultUrl: 'http://localhost:7878',
      icon: 'radarr',
      color: '#FFC230',
      group: 'Downloads',
      description: 'Movie management'
    },
    {
      name: 'Lidarr',
      defaultUrl: 'http://localhost:8686',
      icon: 'lidarr',
      color: '#00E087',
      group: 'Downloads',
      description: 'Music management'
    },
    {
      name: 'Prowlarr',
      defaultUrl: 'http://localhost:9696',
      icon: 'prowlarr',
      color: '#FFC230',
      group: 'Downloads',
      description: 'Indexer management'
    },
    {
      name: 'qBittorrent',
      defaultUrl: 'http://localhost:8080',
      icon: 'qbittorrent',
      color: '#2F67BA',
      group: 'Downloads',
      description: 'Torrent client'
    },
    {
      name: 'SABnzbd',
      defaultUrl: 'http://localhost:8080',
      icon: 'sabnzbd',
      color: '#FDC624',
      group: 'Downloads',
      description: 'Usenet downloader'
    },
    {
      name: 'NZBGet',
      defaultUrl: 'http://localhost:6789',
      icon: 'nzbget',
      color: '#333333',
      group: 'Downloads',
      description: 'Usenet downloader'
    },
    {
      name: 'Transmission',
      defaultUrl: 'http://localhost:9091',
      icon: 'transmission',
      color: '#B50D0D',
      group: 'Downloads',
      description: 'Torrent client'
    },
    {
      name: 'Deluge',
      defaultUrl: 'http://localhost:8112',
      icon: 'deluge',
      color: '#2B5B9E',
      group: 'Downloads',
      description: 'Torrent client'
    }
  ],

  'System': [
    {
      name: 'Portainer',
      defaultUrl: 'http://localhost:9000',
      icon: 'portainer',
      color: '#13BEF9',
      group: 'System',
      description: 'Docker management'
    },
    {
      name: 'Proxmox',
      defaultUrl: 'https://localhost:8006',
      icon: 'proxmox',
      color: '#E57000',
      group: 'System',
      description: 'Virtualization platform'
    },
    {
      name: 'Unraid',
      defaultUrl: 'http://localhost',
      icon: 'unraid',
      color: '#F15A2B',
      group: 'System',
      description: 'NAS & server OS'
    },
    {
      name: 'TrueNAS',
      defaultUrl: 'http://localhost',
      icon: 'truenas',
      color: '#0095D5',
      group: 'System',
      description: 'Storage management'
    },
    {
      name: 'Home Assistant',
      defaultUrl: 'http://localhost:8123',
      icon: 'home-assistant',
      color: '#41BDF5',
      group: 'System',
      description: 'Home automation'
    },
    {
      name: 'Pi-hole',
      defaultUrl: 'http://localhost/admin',
      icon: 'pi-hole',
      color: '#96060C',
      group: 'System',
      description: 'Network-wide ad blocker'
    },
    {
      name: 'AdGuard Home',
      defaultUrl: 'http://localhost:3000',
      icon: 'adguard-home',
      color: '#67B279',
      group: 'System',
      description: 'DNS-based ad blocker'
    },
    {
      name: 'Nginx Proxy Manager',
      defaultUrl: 'http://localhost:81',
      icon: 'nginx-proxy-manager',
      color: '#F15833',
      group: 'System',
      description: 'Reverse proxy management'
    },
    {
      name: 'Traefik',
      defaultUrl: 'http://localhost:8080',
      icon: 'traefik',
      color: '#24A1C1',
      group: 'System',
      description: 'Edge router & proxy'
    },
    {
      name: 'Grafana',
      defaultUrl: 'http://localhost:3000',
      icon: 'grafana',
      color: '#F46800',
      group: 'System',
      description: 'Metrics visualization'
    },
    {
      name: 'Prometheus',
      defaultUrl: 'http://localhost:9090',
      icon: 'prometheus',
      color: '#E6522C',
      group: 'System',
      description: 'Metrics collection'
    },
    {
      name: 'Uptime Kuma',
      defaultUrl: 'http://localhost:3001',
      icon: 'uptime-kuma',
      color: '#5CDD8B',
      group: 'System',
      description: 'Status monitoring'
    }
  ],

  'Utilities': [
    {
      name: 'Vaultwarden',
      defaultUrl: 'http://localhost:8080',
      icon: 'vaultwarden',
      color: '#175DDC',
      group: 'Utilities',
      description: 'Password manager'
    },
    {
      name: 'Nextcloud',
      defaultUrl: 'http://localhost',
      icon: 'nextcloud',
      color: '#0082C9',
      group: 'Utilities',
      description: 'Cloud storage & productivity'
    },
    {
      name: 'Photoprism',
      defaultUrl: 'http://localhost:2342',
      icon: 'photoprism',
      color: '#9C27B0',
      group: 'Utilities',
      description: 'Photo management'
    },
    {
      name: 'Immich',
      defaultUrl: 'http://localhost:2283',
      icon: 'immich',
      color: '#4250AF',
      group: 'Utilities',
      description: 'Photo & video backup'
    },
    {
      name: 'Paperless-ngx',
      defaultUrl: 'http://localhost:8000',
      icon: 'paperless-ngx',
      color: '#17541F',
      group: 'Utilities',
      description: 'Document management'
    },
    {
      name: 'Gitea',
      defaultUrl: 'http://localhost:3000',
      icon: 'gitea',
      color: '#609926',
      group: 'Utilities',
      description: 'Git server'
    },
    {
      name: 'Code Server',
      defaultUrl: 'http://localhost:8443',
      icon: 'code-server',
      color: '#007ACC',
      group: 'Utilities',
      description: 'VS Code in the browser'
    },
    {
      name: 'Syncthing',
      defaultUrl: 'http://localhost:8384',
      icon: 'syncthing',
      color: '#0891D1',
      group: 'Utilities',
      description: 'File synchronization'
    },
    {
      name: 'Mealie',
      defaultUrl: 'http://localhost:9000',
      icon: 'mealie',
      color: '#E58325',
      group: 'Utilities',
      description: 'Recipe manager'
    },
    {
      name: 'Bookstack',
      defaultUrl: 'http://localhost:6875',
      icon: 'bookstack',
      color: '#0288D1',
      group: 'Utilities',
      description: 'Documentation wiki'
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
      type: 'dashboard',
      name: template.icon,
      file: '',
      url: '',
      variant: 'svg'
    },
    color: template.color,
    group: template.group,
    order,
    enabled: true,
    default: order === 0,  // First app is default
    open_mode: 'iframe',
    proxy: false,
    scale: 1
  };
}
