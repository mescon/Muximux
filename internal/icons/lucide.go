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
)

// LucideIconInfo represents metadata about a Lucide icon
type LucideIconInfo struct {
	Name       string   `json:"name"`
	Categories []string `json:"categories,omitempty"`
}

// LucideClient handles fetching icons from the Lucide icon library via CDN
type LucideClient struct {
	cacheDir   string
	cacheTTL   time.Duration
	httpClient *http.Client
	mu         sync.RWMutex
	iconList   []LucideIconInfo
	listLoaded time.Time
}

const (
	LucideCDNURL        = "https://cdn.jsdelivr.net/gh/lucide-icons/lucide@main/icons/%s.svg"
	LucideTreesURL      = "https://api.github.com/repos/lucide-icons/lucide/git/trees/main?recursive=1"
	LucideCategoriesURL = "https://cdn.jsdelivr.net/gh/lucide-icons/lucide@main/categories.json"
)

// NewLucideClient creates a new client for fetching Lucide icons
func NewLucideClient(cacheDir string, cacheTTL time.Duration) *LucideClient {
	return &LucideClient{
		cacheDir: cacheDir,
		cacheTTL: cacheTTL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetIcon returns the icon SVG data for the given name
func (c *LucideClient) GetIcon(name string) ([]byte, string, error) {
	// Strip .svg extension if present
	name = strings.TrimSuffix(name, ".svg")

	// Check cache first
	cached, err := c.getFromCache(name)
	if err == nil {
		return cached, "image/svg+xml", nil
	}

	// Download from CDN
	return c.downloadIcon(name)
}

// ListIcons returns a list of all available Lucide icons with categories
func (c *LucideClient) ListIcons() ([]LucideIconInfo, error) {
	c.mu.RLock()
	if len(c.iconList) > 0 && time.Since(c.listLoaded) < c.cacheTTL {
		list := c.iconList
		c.mu.RUnlock()
		return list, nil
	}
	c.mu.RUnlock()

	return c.fetchIconList()
}

// SearchIcons searches for icons matching the query by name or category
func (c *LucideClient) SearchIcons(query string) ([]LucideIconInfo, error) {
	icons, err := c.ListIcons()
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	var results []LucideIconInfo
	for _, icon := range icons {
		if strings.Contains(strings.ToLower(icon.Name), query) {
			results = append(results, icon)
			continue
		}
		for _, cat := range icon.Categories {
			if strings.Contains(strings.ToLower(cat), query) {
				results = append(results, icon)
				break
			}
		}
	}
	return results, nil
}

// getCachePath returns the local cache path for an icon
func (c *LucideClient) getCachePath(name string) string {
	return filepath.Join(c.cacheDir, name+".svg")
}

// getFromCache attempts to read an icon from the local cache
func (c *LucideClient) getFromCache(name string) ([]byte, error) {
	cachePath := c.getCachePath(name)

	info, err := os.Stat(cachePath)
	if err != nil {
		return nil, err
	}

	// Check if cache is expired
	if c.cacheTTL > 0 && time.Since(info.ModTime()) > c.cacheTTL {
		return nil, fmt.Errorf("cache expired")
	}

	return os.ReadFile(cachePath)
}

// saveToCache saves icon data to the local cache
func (c *LucideClient) saveToCache(name string, data []byte) error {
	if err := os.MkdirAll(c.cacheDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(c.getCachePath(name), data, 0644)
}

// downloadIcon downloads an icon from the Lucide CDN and caches it
func (c *LucideClient) downloadIcon(name string) ([]byte, string, error) {
	url := fmt.Sprintf(LucideCDNURL, name)

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

	if err := c.saveToCache(name, data); err != nil {
		fmt.Printf("Warning: failed to cache lucide icon %s: %v\n", name, err)
	}

	return data, "image/svg+xml", nil
}

// fetchIconList fetches icon names from the GitHub Trees API and categories from CDN
func (c *LucideClient) fetchIconList() ([]LucideIconInfo, error) {
	// Fetch the tree and categories in parallel
	type treeResult struct {
		names []string
		err   error
	}
	type catResult struct {
		categories map[string][]string
		err        error
	}

	treeCh := make(chan treeResult, 1)
	catCh := make(chan catResult, 1)

	go func() {
		names, err := c.fetchTreeNames()
		treeCh <- treeResult{names, err}
	}()

	go func() {
		cats, err := c.fetchCategories()
		catCh <- catResult{cats, err}
	}()

	tr := <-treeCh
	if tr.err != nil {
		return nil, tr.err
	}

	cr := <-catCh
	// Categories are optional — if the fetch fails we still have icon names
	categoryMap := cr.categories

	// Invert categories map: category→[]iconName becomes iconName→[]category
	iconCategories := make(map[string][]string)
	if categoryMap != nil {
		for category, iconNames := range categoryMap {
			for _, name := range iconNames {
				iconCategories[name] = append(iconCategories[name], category)
			}
		}
	}

	// Build final icon list
	icons := make([]LucideIconInfo, 0, len(tr.names))
	for _, name := range tr.names {
		info := LucideIconInfo{Name: name}
		if cats, ok := iconCategories[name]; ok {
			sort.Strings(cats)
			info.Categories = cats
		}
		icons = append(icons, info)
	}

	sort.Slice(icons, func(i, j int) bool {
		return icons[i].Name < icons[j].Name
	})

	// Cache in memory
	c.mu.Lock()
	c.iconList = icons
	c.listLoaded = time.Now()
	c.mu.Unlock()

	return icons, nil
}

// fetchTreeNames fetches icon names from the GitHub Trees API
func (c *LucideClient) fetchTreeNames() ([]string, error) {
	req, err := http.NewRequest("GET", LucideTreesURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch lucide icon tree: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch lucide icon tree: status %d", resp.StatusCode)
	}

	var tree struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"tree"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tree); err != nil {
		return nil, fmt.Errorf("failed to parse lucide icon tree: %w", err)
	}

	var names []string
	for _, entry := range tree.Tree {
		if entry.Type == "blob" && strings.HasPrefix(entry.Path, "icons/") && strings.HasSuffix(entry.Path, ".svg") {
			name := strings.TrimSuffix(strings.TrimPrefix(entry.Path, "icons/"), ".svg")
			names = append(names, name)
		}
	}

	return names, nil
}

// fetchCategories fetches category metadata from the Lucide CDN
func (c *LucideClient) fetchCategories() (map[string][]string, error) {
	resp, err := c.httpClient.Get(LucideCategoriesURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch lucide categories: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch lucide categories: status %d", resp.StatusCode)
	}

	var categories map[string][]string
	if err := json.NewDecoder(resp.Body).Decode(&categories); err != nil {
		return nil, fmt.Errorf("failed to parse lucide categories: %w", err)
	}

	return categories, nil
}
