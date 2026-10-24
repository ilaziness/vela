package tool

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
		{Name: SearchYuanbao, Label: "元宝搜索"},
		{Name: SearchBocha, Label: "博查"},
		{Name: SearchMetaso, Label: "秘塔"},
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
