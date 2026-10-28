package tool

import "github.com/ilaziness/vela/internal/i18n"

// SearchProvider identifies a web search API vendor.
type SearchProvider string

const (
	SearchTavily   SearchProvider = "tavily"
	SearchBrave    SearchProvider = "brave"
	SearchSerper   SearchProvider = "serper"
	SearchDeepseek SearchProvider = "deepseek"
	SearchYuanbao  SearchProvider = "yuanbao"
	SearchBocha    SearchProvider = "bocha"
	SearchMetaso   SearchProvider = "metaso"
)

// SearchProviderInfo drives settings UI fields.
type SearchProviderInfo struct {
	Name  SearchProvider `json:"name"`
	Label string         `json:"label"`
}

// GetAllSearchProviders returns the catalog for settings.
func GetAllSearchProviders() []SearchProviderInfo {
	return []SearchProviderInfo{
		{Name: SearchTavily, Label: "Tavily"},
		{Name: SearchBrave, Label: "Brave Search"},
		{Name: SearchSerper, Label: "Serper (Google)"},
		{Name: SearchDeepseek, Label: "DeepSeek"},
		{Name: SearchYuanbao, Label: i18n.T("search.yuanbao")},
		{Name: SearchBocha, Label: i18n.T("search.bocha")},
		{Name: SearchMetaso, Label: i18n.T("search.metaso")},
	}
}

// ValidSearchProvider reports whether p is a known provider.
func ValidSearchProvider(p SearchProvider) bool {
	for _, info := range GetAllSearchProviders() {
		if info.Name == p {
			return true
		}
	}
	return false
}
