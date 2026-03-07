import type { App } from './types';
import * as m from '$lib/paraglide/messages.js';

export interface PopularAppTemplate {
  name: string;
  defaultUrl: string;
  icon: string;  // dashboard-icons name or custom icon filename
  iconType?: 'dashboard' | 'custom';  // defaults to 'dashboard'
  color: string;
  iconBackground: string;  // Dark contrasting background for icon square
  group: string;
  get description(): string;
}

// Pre-defined homelab app templates with icons from dashboard-icons
// Icons are sourced from: https://github.com/homarr-labs/dashboard-icons
export const popularApps: Record<string, PopularAppTemplate[]> = {
  [m.popularApps_groupMedia()]: [
    {
      name: 'Plex',
      defaultUrl: 'http://localhost:32400/web',
      icon: 'plex',
      color: '#E5A00D',
      iconBackground: '#2D2200',
      get group() { return m.popularApps_groupMedia(); },
      get description() { return m.popularApps_plexDesc(); }
    },
    {
      name: 'Jellyfin',
      defaultUrl: 'http://localhost:8096',
      icon: 'jellyfin',
      color: '#00A4DC',
      iconBackground: '#0D1F3D',
      get group() { return m.popularApps_groupMedia(); },
      get description() { return m.popularApps_jellyfinDesc(); }
    },
    {
      name: 'Emby',
      defaultUrl: 'http://localhost:8096',
      icon: 'emby',
      color: '#52B54B',
      iconBackground: '#0D2E1A',
      get group() { return m.popularApps_groupMedia(); },
      get description() { return m.popularApps_embyDesc(); }
    },
    {
      name: 'Tautulli',
      defaultUrl: 'http://localhost:8181',
      icon: 'tautulli',
      color: '#E5A00D',
      iconBackground: '#2D2200',
      get group() { return m.popularApps_groupMedia(); },
      get description() { return m.popularApps_tautulliDesc(); }
    },
    {
      name: 'Stash',
      defaultUrl: 'http://localhost:9999',
      icon: 'stash',
      color: '#1A1A2E',
      iconBackground: '#0D0D1A',
      get group() { return m.popularApps_groupMedia(); },
      get description() { return m.popularApps_stashDesc(); }
    },
    {
      name: 'Overseerr',
      defaultUrl: 'http://localhost:5055',
      icon: 'overseerr',
      color: '#7B2BF9',
      iconBackground: '#150D2E',
      get group() { return m.popularApps_groupMedia(); },
      get description() { return m.popularApps_overseerrDesc(); }
    },
    {
      name: 'Navidrome',
      defaultUrl: 'http://localhost:4533',
      icon: 'navidrome',
      color: '#0091EA',
      iconBackground: '#0D1F3D',
      get group() { return m.popularApps_groupMedia(); },
      get description() { return m.popularApps_navidromeDesc(); }
    },
    {
      name: 'Jellyseerr',
      defaultUrl: 'http://localhost:5055',
      icon: 'jellyseerr',
      color: '#763DCD',
      iconBackground: '#150D2E',
      get group() { return m.popularApps_groupMedia(); },
      get description() { return m.popularApps_jellyseerrDesc(); }
    },
    {
      name: 'Audiobookshelf',
      defaultUrl: 'http://localhost:13378',
      icon: 'audiobookshelf',
      color: '#875D27',
      iconBackground: '#2D1800',
      get group() { return m.popularApps_groupMedia(); },
      get description() { return m.popularApps_audiobookshelfDesc(); }
    },
    {
      name: 'Kavita',
      defaultUrl: 'http://localhost:5000',
      icon: 'kavita',
      color: '#4AC694',
      iconBackground: '#0D2E1A',
      get group() { return m.popularApps_groupMedia(); },
      get description() { return m.popularApps_kavitaDesc(); }
    },
    {
      name: 'Komga',
      defaultUrl: 'http://localhost:25600',
      icon: 'komga',
      color: '#005ED3',
      iconBackground: '#0D1F3D',
      get group() { return m.popularApps_groupMedia(); },
      get description() { return m.popularApps_komgaDesc(); }
    },
    {
      name: 'Calibre-Web',
      defaultUrl: 'http://localhost:8083',
      icon: 'calibre-web',
      color: '#45B29D',
      iconBackground: '#0D2E1A',
      get group() { return m.popularApps_groupMedia(); },
      get description() { return m.popularApps_calibreWebDesc(); }
    }
  ],

  [m.popularApps_groupDownloads()]: [
    {
      name: 'Sonarr',
      defaultUrl: 'http://localhost:8989',
      icon: 'sonarr',
      color: '#00CCFF',
      iconBackground: '#0D2633',
      get group() { return m.popularApps_groupDownloads(); },
      get description() { return m.popularApps_sonarrDesc(); }
    },
    {
      name: 'Radarr',
      defaultUrl: 'http://localhost:7878',
      icon: 'radarr',
      color: '#FFC230',
      iconBackground: '#2D2200',
      get group() { return m.popularApps_groupDownloads(); },
      get description() { return m.popularApps_radarrDesc(); }
    },
    {
      name: 'Lidarr',
      defaultUrl: 'http://localhost:8686',
      icon: 'lidarr',
      color: '#00E087',
      iconBackground: '#0D2E1A',
      get group() { return m.popularApps_groupDownloads(); },
      get description() { return m.popularApps_lidarrDesc(); }
    },
    {
      name: 'Whisparr',
      defaultUrl: 'http://localhost:6969',
      icon: 'whisparr',
      color: '#E0528B',
      iconBackground: '#2D0C1A',
      get group() { return m.popularApps_groupDownloads(); },
      get description() { return m.popularApps_whisparrDesc(); }
    },
    {
      name: 'Bazarr',
      defaultUrl: 'http://localhost:6767',
      icon: 'bazarr',
      color: '#4FC3F7',
      iconBackground: '#0D2633',
      get group() { return m.popularApps_groupDownloads(); },
      get description() { return m.popularApps_bazarrDesc(); }
    },
    {
      name: 'Prowlarr',
      defaultUrl: 'http://localhost:9696',
      icon: 'prowlarr',
      color: '#FFC230',
      iconBackground: '#2D2200',
      get group() { return m.popularApps_groupDownloads(); },
      get description() { return m.popularApps_prowlarrDesc(); }
    },
    {
      name: 'qBittorrent',
      defaultUrl: 'http://localhost:8080',
      icon: 'qbittorrent',
      color: '#2F67BA',
      iconBackground: '#0D1F3D',
      get group() { return m.popularApps_groupDownloads(); },
      get description() { return m.popularApps_qbittorrentDesc(); }
    },
    {
      name: 'SABnzbd',
      defaultUrl: 'http://localhost:8080',
      icon: 'sabnzbd',
      color: '#FDC624',
      iconBackground: '#2D2200',
      get group() { return m.popularApps_groupDownloads(); },
      get description() { return m.popularApps_sabnzbdDesc(); }
    },
    {
      name: 'NZBGet',
      defaultUrl: 'http://localhost:6789',
      icon: 'nzbget',
      color: '#333333',
      iconBackground: '#1A1A1A',
      get group() { return m.popularApps_groupDownloads(); },
      get description() { return m.popularApps_nzbgetDesc(); }
    },
    {
      name: 'Transmission',
      defaultUrl: 'http://localhost:9091',
      icon: 'transmission',
      color: '#B50D0D',
      iconBackground: '#2D0C07',
      get group() { return m.popularApps_groupDownloads(); },
      get description() { return m.popularApps_transmissionDesc(); }
    },
    {
      name: 'Deluge',
      defaultUrl: 'http://localhost:8112',
      icon: 'deluge',
      color: '#2B5B9E',
      iconBackground: '#0D1F3D',
      get group() { return m.popularApps_groupDownloads(); },
      get description() { return m.popularApps_delugeDesc(); }
    },
    {
      name: 'RDTClient',
      defaultUrl: 'http://localhost:6500',
      icon: 'rdt-client',
      color: '#5C6BC0',
      iconBackground: '#150D2E',
      get group() { return m.popularApps_groupDownloads(); },
      get description() { return m.popularApps_rdtclientDesc(); }
    },
    {
      name: 'Huntarr',
      defaultUrl: 'http://localhost:9705',
      icon: 'huntarr',
      color: '#FF6B35',
      iconBackground: '#2D1800',
      get group() { return m.popularApps_groupDownloads(); },
      get description() { return m.popularApps_huntarrDesc(); }
    },
    {
      name: 'Readarr',
      defaultUrl: 'http://localhost:8787',
      icon: 'readarr',
      color: '#8E2222',
      iconBackground: '#2D0C07',
      get group() { return m.popularApps_groupDownloads(); },
      get description() { return m.popularApps_readarrDesc(); }
    }
  ],

  [m.popularApps_groupSystem()]: [
    {
      name: 'Portainer',
      defaultUrl: 'http://localhost:9000',
      icon: 'portainer',
      color: '#13BEF9',
      iconBackground: '#0D2633',
      get group() { return m.popularApps_groupSystem(); },
      get description() { return m.popularApps_portainerDesc(); }
    },
    {
      name: 'Proxmox',
      defaultUrl: 'https://localhost:8006',
      icon: 'proxmox',
      color: '#E57000',
      iconBackground: '#2D1800',
      get group() { return m.popularApps_groupSystem(); },
      get description() { return m.popularApps_proxmoxDesc(); }
    },
    {
      name: 'Unraid',
      defaultUrl: 'http://localhost',
      icon: 'unraid',
      color: '#F15A2B',
      iconBackground: '#2D0C07',
      get group() { return m.popularApps_groupSystem(); },
      get description() { return m.popularApps_unraidDesc(); }
    },
    {
      name: 'TrueNAS',
      defaultUrl: 'http://localhost',
      icon: 'truenas',
      color: '#0095D5',
      iconBackground: '#0D1F3D',
      get group() { return m.popularApps_groupSystem(); },
      get description() { return m.popularApps_truenasDesc(); }
    },
    {
      name: 'Home Assistant',
      defaultUrl: 'http://localhost:8123',
      icon: 'home-assistant',
      color: '#41BDF5',
      iconBackground: '#0D1F3D',
      get group() { return m.popularApps_groupSystem(); },
      get description() { return m.popularApps_homeAssistantDesc(); }
    },
    {
      name: 'Pi-hole',
      defaultUrl: 'http://localhost/admin',
      icon: 'pi-hole',
      color: '#96060C',
      iconBackground: '#2D0C07',
      get group() { return m.popularApps_groupSystem(); },
      get description() { return m.popularApps_piholeDesc(); }
    },
    {
      name: 'AdGuard Home',
      defaultUrl: 'http://localhost:3000',
      icon: 'adguard-home',
      color: '#67B279',
      iconBackground: '#0D2E1A',
      get group() { return m.popularApps_groupSystem(); },
      get description() { return m.popularApps_adguardHomeDesc(); }
    },
    {
      name: 'Nginx Proxy Manager',
      defaultUrl: 'http://localhost:81',
      icon: 'nginx-proxy-manager',
      color: '#F15833',
      iconBackground: '#2D1800',
      get group() { return m.popularApps_groupSystem(); },
      get description() { return m.popularApps_nginxProxyManagerDesc(); }
    },
    {
      name: 'Traefik',
      defaultUrl: 'http://localhost:8080',
      icon: 'traefik',
      color: '#24A1C1',
      iconBackground: '#0D2633',
      get group() { return m.popularApps_groupSystem(); },
      get description() { return m.popularApps_traefikDesc(); }
    },
    {
      name: 'Grafana',
      defaultUrl: 'http://localhost:3000',
      icon: 'grafana',
      color: '#F46800',
      iconBackground: '#2D1800',
      get group() { return m.popularApps_groupSystem(); },
      get description() { return m.popularApps_grafanaDesc(); }
    },
    {
      name: 'Prometheus',
      defaultUrl: 'http://localhost:9090',
      icon: 'prometheus',
      color: '#E6522C',
      iconBackground: '#2D0C07',
      get group() { return m.popularApps_groupSystem(); },
      get description() { return m.popularApps_prometheusDesc(); }
    },
    {
      name: 'Uptime Kuma',
      defaultUrl: 'http://localhost:3001',
      icon: 'uptime-kuma',
      color: '#5CDD8B',
      iconBackground: '#0D2E1A',
      get group() { return m.popularApps_groupSystem(); },
      get description() { return m.popularApps_uptimeKumaDesc(); }
    },
    {
      name: 'Frigate',
      defaultUrl: 'http://localhost:8971',
      icon: 'frigate',
      color: '#2196F3',
      iconBackground: '#0D1F3D',
      get group() { return m.popularApps_groupSystem(); },
      get description() { return m.popularApps_frigateDesc(); }
    },
    {
      name: 'n8n',
      defaultUrl: 'http://localhost:5678',
      icon: 'n8n',
      color: '#EA4B71',
      iconBackground: '#2D0C1A',
      get group() { return m.popularApps_groupSystem(); },
      get description() { return m.popularApps_n8nDesc(); }
    },
    {
      name: 'Tdarr',
      defaultUrl: 'http://localhost:8265',
      icon: 'tdarr',
      color: '#6EC6FF',
      iconBackground: '#0D2633',
      get group() { return m.popularApps_groupSystem(); },
      get description() { return m.popularApps_tdarrDesc(); }
    },
    {
      name: 'Guacamole',
      defaultUrl: 'http://localhost:8080/guacamole',
      icon: 'guacamole',
      color: '#3F8E4F',
      iconBackground: '#0D2E1A',
      get group() { return m.popularApps_groupSystem(); },
      get description() { return m.popularApps_guacamoleDesc(); }
    },
    {
      name: 'Headplane',
      defaultUrl: 'http://localhost:3000',
      icon: 'headscale',
      color: '#4A90D9',
      iconBackground: '#0D1F3D',
      get group() { return m.popularApps_groupSystem(); },
      get description() { return m.popularApps_headplaneDesc(); }
    },
    {
      name: 'Arcane',
      defaultUrl: 'http://localhost:3552',
      icon: 'arcane',
      color: '#7C4DFF',
      iconBackground: '#150D2E',
      get group() { return m.popularApps_groupSystem(); },
      get description() { return m.popularApps_arcaneDesc(); }
    },
    {
      name: 'Healarr',
      defaultUrl: 'http://localhost:3090',
      icon: 'healarr',
      iconType: 'custom',
      color: '#4CAF50',
      iconBackground: '#0D2E1A',
      get group() { return m.popularApps_groupSystem(); },
      get description() { return m.popularApps_healarrDesc(); }
    },
    {
      name: 'Profilarr',
      defaultUrl: 'http://localhost:6868',
      icon: 'profilarr',
      color: '#FF7043',
      iconBackground: '#2D1800',
      get group() { return m.popularApps_groupSystem(); },
      get description() { return m.popularApps_profilarrDesc(); }
    },
    {
      name: 'Agregarr',
      defaultUrl: 'http://localhost:7171',
      icon: 'agregarr',
      color: '#AB47BC',
      iconBackground: '#150D2E',
      get group() { return m.popularApps_groupSystem(); },
      get description() { return m.popularApps_agregarrDesc(); }
    },
    {
      name: 'Authentik',
      defaultUrl: 'http://localhost:9000',
      icon: 'authentik',
      color: '#FD4B2D',
      iconBackground: '#2D0C07',
      get group() { return m.popularApps_groupSystem(); },
      get description() { return m.popularApps_authentikDesc(); }
    },
    {
      name: 'Authelia',
      defaultUrl: 'http://localhost:9091',
      icon: 'authelia',
      color: '#3F51B4',
      iconBackground: '#0D1F3D',
      get group() { return m.popularApps_groupSystem(); },
      get description() { return m.popularApps_autheliaDesc(); }
    },
    {
      name: 'Tailscale',
      defaultUrl: 'https://login.tailscale.com/admin',
      icon: 'tailscale',
      color: '#242424',
      iconBackground: '#1A1A1A',
      get group() { return m.popularApps_groupSystem(); },
      get description() { return m.popularApps_tailscaleDesc(); }
    },
    {
      name: 'WireGuard',
      defaultUrl: 'http://localhost:51821',
      icon: 'wireguard',
      color: '#88171A',
      iconBackground: '#2D0C07',
      get group() { return m.popularApps_groupSystem(); },
      get description() { return m.popularApps_wireguardDesc(); }
    },
    {
      name: 'Watchtower',
      defaultUrl: 'http://localhost:8080',
      icon: 'watchtower',
      color: '#003343',
      iconBackground: '#0D2633',
      get group() { return m.popularApps_groupSystem(); },
      get description() { return m.popularApps_watchtowerDesc(); }
    },
    {
      name: 'CrowdSec',
      defaultUrl: 'http://localhost:8080',
      icon: 'crowdsec',
      color: '#4E4A99',
      iconBackground: '#150D2E',
      get group() { return m.popularApps_groupSystem(); },
      get description() { return m.popularApps_crowdsecDesc(); }
    },
    {
      name: 'Dozzle',
      defaultUrl: 'http://localhost:8080',
      icon: 'dozzle',
      color: '#F5A623',
      iconBackground: '#2D2200',
      get group() { return m.popularApps_groupSystem(); },
      get description() { return m.popularApps_dozzleDesc(); }
    },
    {
      name: 'Glances',
      defaultUrl: 'http://localhost:61208',
      icon: 'glances',
      color: '#57CB6A',
      iconBackground: '#0D2E1A',
      get group() { return m.popularApps_groupSystem(); },
      get description() { return m.popularApps_glancesDesc(); }
    },
    {
      name: 'Netdata',
      defaultUrl: 'http://localhost:19999',
      icon: 'netdata',
      color: '#00AB44',
      iconBackground: '#0D2E1A',
      get group() { return m.popularApps_groupSystem(); },
      get description() { return m.popularApps_netdataDesc(); }
    }
  ],

  [m.popularApps_groupUtilities()]: [
    {
      name: 'Vaultwarden',
      defaultUrl: 'http://localhost:8080',
      icon: 'vaultwarden',
      color: '#175DDC',
      iconBackground: '#0D1F3D',
      get group() { return m.popularApps_groupUtilities(); },
      get description() { return m.popularApps_vaultwardenDesc(); }
    },
    {
      name: 'Nextcloud',
      defaultUrl: 'http://localhost',
      icon: 'nextcloud',
      color: '#0082C9',
      iconBackground: '#0D1F3D',
      get group() { return m.popularApps_groupUtilities(); },
      get description() { return m.popularApps_nextcloudDesc(); }
    },
    {
      name: 'Photoprism',
      defaultUrl: 'http://localhost:2342',
      icon: 'photoprism',
      color: '#9C27B0',
      iconBackground: '#150D2E',
      get group() { return m.popularApps_groupUtilities(); },
      get description() { return m.popularApps_photoprismDesc(); }
    },
    {
      name: 'Immich',
      defaultUrl: 'http://localhost:2283',
      icon: 'immich',
      color: '#4250AF',
      iconBackground: '#150D2E',
      get group() { return m.popularApps_groupUtilities(); },
      get description() { return m.popularApps_immichDesc(); }
    },
    {
      name: 'Paperless-ngx',
      defaultUrl: 'http://localhost:8000',
      icon: 'paperless-ngx',
      color: '#17541F',
      iconBackground: '#8BC34A',
      get group() { return m.popularApps_groupUtilities(); },
      get description() { return m.popularApps_paperlessNgxDesc(); }
    },
    {
      name: 'Gitea',
      defaultUrl: 'http://localhost:3000',
      icon: 'gitea',
      color: '#609926',
      iconBackground: '#0D2E1A',
      get group() { return m.popularApps_groupUtilities(); },
      get description() { return m.popularApps_giteaDesc(); }
    },
    {
      name: 'Code Server',
      defaultUrl: 'http://localhost:8443',
      icon: 'vscode',
      color: '#007ACC',
      iconBackground: '#0D1F3D',
      get group() { return m.popularApps_groupUtilities(); },
      get description() { return m.popularApps_codeServerDesc(); }
    },
    {
      name: 'Syncthing',
      defaultUrl: 'http://localhost:8384',
      icon: 'syncthing',
      color: '#0891D1',
      iconBackground: '#0D1F3D',
      get group() { return m.popularApps_groupUtilities(); },
      get description() { return m.popularApps_syncthingDesc(); }
    },
    {
      name: 'Mealie',
      defaultUrl: 'http://localhost:9000',
      icon: 'mealie',
      color: '#E58325',
      iconBackground: '#2D1800',
      get group() { return m.popularApps_groupUtilities(); },
      get description() { return m.popularApps_mealieDesc(); }
    },
    {
      name: 'Bookstack',
      defaultUrl: 'http://localhost:6875',
      icon: 'bookstack',
      color: '#0288D1',
      iconBackground: '#0D1F3D',
      get group() { return m.popularApps_groupUtilities(); },
      get description() { return m.popularApps_bookstackDesc(); }
    },
    {
      name: 'Wiki.js',
      defaultUrl: 'http://localhost:3000',
      icon: 'wikijs',
      color: '#02BEF3',
      iconBackground: '#0D2633',
      get group() { return m.popularApps_groupUtilities(); },
      get description() { return m.popularApps_wikijsDesc(); }
    },
    {
      name: 'Stirling-PDF',
      defaultUrl: 'http://localhost:8080',
      icon: 'stirling-pdf',
      color: '#8E3131',
      iconBackground: '#2D0C07',
      get group() { return m.popularApps_groupUtilities(); },
      get description() { return m.popularApps_stirlingPdfDesc(); }
    },
    {
      name: 'IT-Tools',
      defaultUrl: 'http://localhost:8080',
      icon: 'it-tools',
      color: '#18A058',
      iconBackground: '#0D2E1A',
      get group() { return m.popularApps_groupUtilities(); },
      get description() { return m.popularApps_itToolsDesc(); }
    },
    {
      name: 'Excalidraw',
      defaultUrl: 'http://localhost:3000',
      icon: 'excalidraw',
      color: '#6965DB',
      iconBackground: '#150D2E',
      get group() { return m.popularApps_groupUtilities(); },
      get description() { return m.popularApps_excalidrawDesc(); }
    },
    {
      name: 'Changedetection.io',
      defaultUrl: 'http://localhost:5000',
      icon: 'changedetection',
      color: '#3056D3',
      iconBackground: '#0D1F3D',
      get group() { return m.popularApps_groupUtilities(); },
      get description() { return m.popularApps_changedetectionDesc(); }
    },
    {
      name: 'FreshRSS',
      defaultUrl: 'http://localhost:8080',
      icon: 'freshrss',
      color: '#0062BE',
      iconBackground: '#0D1F3D',
      get group() { return m.popularApps_groupUtilities(); },
      get description() { return m.popularApps_freshrssDesc(); }
    },
    {
      name: 'Linkding',
      defaultUrl: 'http://localhost:9090',
      icon: 'linkding',
      color: '#5856E0',
      iconBackground: '#150D2E',
      get group() { return m.popularApps_groupUtilities(); },
      get description() { return m.popularApps_linkdingDesc(); }
    }
  ],

  [m.popularApps_groupAI()]: [
    {
      name: 'Ollama',
      defaultUrl: 'http://localhost:11434',
      icon: 'ollama',
      color: '#000000',
      iconBackground: '#1A1A1A',
      get group() { return m.popularApps_groupAI(); },
      get description() { return m.popularApps_ollamaDesc(); }
    },
    {
      name: 'Open WebUI',
      defaultUrl: 'http://localhost:3000',
      icon: 'open-webui',
      color: '#000000',
      iconBackground: '#1A1A1A',
      get group() { return m.popularApps_groupAI(); },
      get description() { return m.popularApps_openWebuiDesc(); }
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
    scale: 1
  };
}
