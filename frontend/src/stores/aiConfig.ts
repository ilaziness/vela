import { create } from 'zustand';
import { AIConfig, AIService } from '../../bindings/github.com/ilaziness/vela/services';
import { Provider } from "../../bindings/github.com/ilaziness/vela/internal/ai/models";
import { SearchProvider } from "../../bindings/github.com/ilaziness/vela/internal/ai/tool/models";
import { translateServiceError } from "../func/service";

export interface SearchProviderRow {
  name: string;
  label: string;
}

/** Mirrors internal/ai/tool.GetAllSearchProviders — always available even if RPC fails. */
export const DEFAULT_WEB_SEARCH_PROVIDERS: SearchProviderRow[] = [
  { name: SearchProvider.SearchTavily, label: "Tavily" },
  { name: SearchProvider.SearchBrave, label: "Brave Search" },
  { name: SearchProvider.SearchSerper, label: "Serper (Google)" },
  { name: SearchProvider.SearchDeepseek, label: "DeepSeek" },
  { name: SearchProvider.SearchYuanbao, label: "Yuanbao" },
  { name: SearchProvider.SearchBocha, label: "Bocha" },
  { name: SearchProvider.SearchMetaso, label: "Metaso" },
];

function knownSearchProvider(p: string | undefined): SearchProvider {
  if (p && DEFAULT_WEB_SEARCH_PROVIDERS.some((row) => row.name === p)) {
    return p as SearchProvider;
  }
  return SearchProvider.$zero;
}

function normalizeLoadedConfig(config: AIConfig | null | undefined): AIConfig {
  return {
    ...defaultConfig,
    ...(config || {}),
    api_key: '',
    filesystem: { allowed_dirs: config?.filesystem?.allowed_dirs ?? [] },
    web_search: {
      provider: knownSearchProvider(config?.web_search?.provider),
      api_key: '',
      api_key_encrypted: config?.web_search?.api_key_encrypted,
    },
  };
}

interface AIConfigState {
  config: AIConfig | null;
  isLoading: boolean;
  error: string | null;
  providers: { name: string; label: string; needs_api_key: boolean; needs_endpoint: boolean }[];
  isLoadingProviders: boolean;
  webSearchProviders: SearchProviderRow[];

  loadConfig: () => Promise<void>;
  saveConfig: (config: AIConfig) => Promise<void>;
  resetConfig: () => Promise<void>;
  resetToolsConfig: () => Promise<void>;
  loadProviders: () => Promise<void>;
  loadWebSearchProviders: () => Promise<void>;
  updatePartialConfig: (partial: Partial<AIConfig>) => void;
  clearError: () => void;
}

const defaultConfig: AIConfig = {
  enabled: false,
  provider: Provider.ProviderOllama,
  model: 'llama3.2',
  endpoint: 'http://localhost:11434',
  temperature: 0.7,
  max_tokens: 2048,
  echo_ssh_commands: false,
  max_agent_turns: 30,
  filesystem: { allowed_dirs: [] },
  web_search: { provider: SearchProvider.$zero },
};

export const useAIConfigStore = create<AIConfigState>((set) => ({
  config: null,
  isLoading: false,
  error: null,
  providers: [],
  isLoadingProviders: false,
  webSearchProviders: DEFAULT_WEB_SEARCH_PROVIDERS,

  loadConfig: async () => {
    set({ isLoading: true, error: null });
    try {
      const config = await AIService.GetConfig();
      set({ config: normalizeLoadedConfig(config), isLoading: false });
    } catch (err) {
      set({ error: translateServiceError(err), isLoading: false });
    }
  },

  loadProviders: async () => {
    set({ isLoadingProviders: true });
    try {
      const providers = await AIService.GetProviders();
      set({ providers: providers || [], isLoadingProviders: false });
    } catch (_err) {
      set({ providers: [], isLoadingProviders: false });
    }
  },

  loadWebSearchProviders: async () => {
    try {
      const providers = await AIService.GetWebSearchProviders();
      const mapped = (providers || [])
        .map((p) => ({
          name: String(p?.name ?? ""),
          label: p?.label ?? "",
        }))
        .filter((p) => p.name !== "");
      set({
        webSearchProviders: mapped.length > 0 ? mapped : DEFAULT_WEB_SEARCH_PROVIDERS,
      });
    } catch (_err) {
      set({ webSearchProviders: DEFAULT_WEB_SEARCH_PROVIDERS });
    }
  },

  saveConfig: async (config: AIConfig) => {
    await AIService.SaveConfig(config);
    const saved = await AIService.GetConfig();
    set({ config: normalizeLoadedConfig(saved), error: null });
  },

  resetConfig: async () => {
    await AIService.ResetConfig();
    const config = await AIService.GetConfig();
    set({ config: normalizeLoadedConfig(config), error: null });
  },

  resetToolsConfig: async () => {
    await AIService.ResetToolsConfig();
    const config = await AIService.GetConfig();
    set({ config: normalizeLoadedConfig(config), error: null });
  },

  updatePartialConfig: (partial: Partial<AIConfig>) => {
    set((state) => ({
      config: state.config ? { ...state.config, ...partial } : { ...defaultConfig, ...partial },
    }));
  },

  clearError: () => set({ error: null }),
}));
