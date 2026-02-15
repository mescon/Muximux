package icons

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mescon/muximux/v3/internal/logging"
)

// DashboardIconsClient handles fetching icons from dashboardicons.com
type DashboardIconsClient struct {
	cacheDir   string
	cacheTTL   time.Duration
	httpClient *http.Client
	mu         sync.RWMutex
	iconList   []IconInfo
	listLoaded time.Time
}

// IconInfo represents metadata about an available icon
type IconInfo struct {
	Name     string   `json:"name"`
	Variants []string `json:"variants"` // e.g., ["svg", "png", "webp"]
}

// BaseURL is the GitHub raw content URL for dashboard icons
const (
	GitHubOwner = "homarr-labs"
	GitHubRepo  = "dashboard-icons"
	RawBaseURL  = "https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons"
	APIBaseURL  = "https://api.github.com/repos/homarr-labs/dashboard-icons/contents"
)

// NewDashboardIconsClient creates a new client for fetching dashboard icons
func NewDashboardIconsClient(cacheDir string, cacheTTL time.Duration) *DashboardIconsClient {
	return &DashboardIconsClient{
		cacheDir: cacheDir,
		cacheTTL: cacheTTL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// variantPreference is the fallback order when a requested variant is unavailable.
var variantPreference = []string{"svg", "webp", "png"}

// GetIcon returns the icon data for the given name and variant.
// If the requested variant is not available, it falls back through
// svg → webp → png until one succeeds.
func (c *DashboardIconsClient) GetIcon(name, variant string) ([]byte, string, error) {
	if variant == "" {
		variant = "svg"
	}

	// Check cache first for the requested variant
	if cached, contentType, err := c.getFromCache(name, variant); err == nil {
		return cached, contentType, nil
	}

	// Try downloading the requested variant
	if data, contentType, err := c.downloadIcon(name, variant); err == nil {
		return data, contentType, nil
	}

	// Fallback: try other variants in preference order
	for _, fallback := range variantPreference {
		if fallback == variant {
			continue // already tried
		}
		if cached, contentType, err := c.getFromCache(name, fallback); err == nil {
			return cached, contentType, nil
		}
		if data, contentType, err := c.downloadIcon(name, fallback); err == nil {
			return data, contentType, nil
		}
	}

	return nil, "", fmt.Errorf("icon not found: %s (tried all variants)", name)
}

// GetIconPath returns the local file path for a cached icon
func (c *DashboardIconsClient) GetIconPath(name, variant string) (string, error) {
	if variant == "" {
		variant = "svg"
	}

	cachePath := c.getCachePath(name, variant)
	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}

	// Download if not cached
	_, _, err := c.downloadIcon(name, variant)
	if err != nil {
		return "", err
	}

	return cachePath, nil
}

// ListIcons returns a list of available icons
func (c *DashboardIconsClient) ListIcons() ([]IconInfo, error) {
	c.mu.RLock()
	if len(c.iconList) > 0 && time.Since(c.listLoaded) < c.cacheTTL {
		list := c.iconList
		c.mu.RUnlock()
		return list, nil
	}
	c.mu.RUnlock()

	return c.fetchIconList()
}

// SearchIcons searches for icons matching the query
func (c *DashboardIconsClient) SearchIcons(query string) ([]IconInfo, error) {
	icons, err := c.ListIcons()
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	var results []IconInfo
	for _, icon := range icons {
		if strings.Contains(strings.ToLower(icon.Name), query) {
			results = append(results, icon)
		}
	}
	return results, nil
}

// getCachePath returns the local cache path for an icon
func (c *DashboardIconsClient) getCachePath(name, variant string) string {
	return filepath.Join(c.cacheDir, fmt.Sprintf("%s.%s", name, variant))
}

// getFromCache attempts to read an icon from the local cache
func (c *DashboardIconsClient) getFromCache(name, variant string) ([]byte, string, error) {
	cachePath := c.getCachePath(name, variant)

	info, err := os.Stat(cachePath)
	if err != nil {
		return nil, "", err
	}

	// Check if cache is expired
	if c.cacheTTL > 0 && time.Since(info.ModTime()) > c.cacheTTL {
		return nil, "", fmt.Errorf("cache expired")
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, "", err
	}

	return data, getContentType(variant), nil
}

// downloadIcon downloads an icon from GitHub and caches it
func (c *DashboardIconsClient) downloadIcon(name, variant string) ([]byte, string, error) {
	// Build URL based on variant
	var folder string
	switch variant {
	case "svg":
		folder = "svg"
	case "png":
		folder = "png"
	case "webp":
		folder = "webp"
	default:
		folder = "svg"
		variant = "svg"
	}

	url := fmt.Sprintf("%s/%s/%s.%s", RawBaseURL, folder, name, variant)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch icon: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("icon not found: %s (status %d)", name, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read icon data: %w", err)
	}

	// Cache the icon
	if err := c.saveToCache(name, variant, data); err != nil {
		// Log but don't fail - caching is optional
		logging.Warn("Failed to cache icon", "source", "icons", "name", name, "error", err)
	}

	return data, getContentType(variant), nil
}

// saveToCache saves icon data to the local cache
func (c *DashboardIconsClient) saveToCache(name, variant string, data []byte) error {
	// Ensure cache directory exists
	if err := os.MkdirAll(c.cacheDir, 0755); err != nil {
		return err
	}

	cachePath := c.getCachePath(name, variant)
	return os.WriteFile(cachePath, data, 0644)
}

// fetchIconList fetches the list of available icons from GitHub using the Trees API
// (the Contents API has a 1000-file limit per directory)
func (c *DashboardIconsClient) fetchIconList() ([]IconInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees/main?recursive=1", GitHubOwner, GitHubRepo)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch icon list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch icon list: status %d", resp.StatusCode)
	}

	var tree struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"tree"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tree); err != nil {
		return nil, fmt.Errorf("failed to parse icon tree: %w", err)
	}

	// Collect icons from all variant directories (svg/, png/, webp/)
	type variantDir struct {
		prefix string
		suffix string
		label  string
	}
	dirs := []variantDir{
		{"svg/", ".svg", "svg"},
		{"png/", ".png", "png"},
		{"webp/", ".webp", "webp"},
	}

	iconVariants := make(map[string]map[string]bool)
	for _, dir := range dirs {
		for _, entry := range tree.Tree {
			if entry.Type == "blob" && strings.HasPrefix(entry.Path, dir.prefix) && strings.HasSuffix(entry.Path, dir.suffix) {
				name := strings.TrimSuffix(strings.TrimPrefix(entry.Path, dir.prefix), dir.suffix)
				if iconVariants[name] == nil {
					iconVariants[name] = make(map[string]bool)
				}
				iconVariants[name][dir.label] = true
			}
		}
	}

	var icons []IconInfo
	for name, variants := range iconVariants {
		var variantList []string
		for _, v := range variantPreference {
			if variants[v] {
				variantList = append(variantList, v)
			}
		}
		icons = append(icons, IconInfo{
			Name:     name,
			Variants: variantList,
		})
	}

	// Sort for consistent ordering
	sortIconInfoByName(icons)

	// Cache the list
	c.mu.Lock()
	c.iconList = icons
	c.listLoaded = time.Now()
	c.mu.Unlock()

	return icons, nil
}

func sortIconInfoByName(icons []IconInfo) {
	sort.Slice(icons, func(i, j int) bool {
		return icons[i].Name < icons[j].Name
	})
}

// getContentType returns the MIME type for an icon variant
func getContentType(variant string) string {
	switch variant {
	case "svg":
		return "image/svg+xml"
	case "png":
		return "image/png"
	case "webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

// ClearCache removes all cached icons
func (c *DashboardIconsClient) ClearCache() error {
	return os.RemoveAll(c.cacheDir)
}
