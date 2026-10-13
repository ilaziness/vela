// AI 相关 TypeScript 类型定义

// 注意：此文件中的类型定义已过时，不再使用
// 当前使用的是从 Go 绑定自动生成的类型定义
// 位置：frontend/bindings/github.com/ilaziness/vela/services/index.ts

// 模型提供商
export type AIProvider =
  | "ollama"
  | "openai"
  | "anthropic"
  | "googleai"
  | "azure_openai"
  | "mistral"
  | "cohere";

// AI 配置（已过时，请使用 Go 绑定生成的 AIConfig）
export interface AIConfig {
  enabled: boolean;
  provider: AIProvider;
  model: string;
  api_key?: string;
  api_key_encrypted?: string;
  ollama_endpoint: string;
  temperature: number;
  max_tokens: number;
  safety_check_level: "low" | "medium" | "high";
}

// 上下文信息
export interface AIContext {
  current_directory: string;
  os_info: string;
  recent_commands: string[];
  user_level: "beginner" | "intermediate" | "advanced";
}

// 命令生成请求
export interface AICommandRequest {
  natural_language: string;
  session_id?: string;
  context?: AIContext;
}

// 安全警告
export interface AISafetyWarning {
  level: "high" | "medium" | "low";
  message: string;
  risk_detail: string;
  suggestion: string;
}

// 命令生成响应
export interface AICommandResponse {
  command: string;
  explanation: string;
  safety_warning?: AISafetyWarning;
  alternatives: string[];
  confidence: number;
}

// 命令解释请求
export interface ExplainCommandRequest {
  command: string;
  session_id?: string;
  context?: AIContext;
}

// 命令各部分解释
export interface CommandPart {
  part: string;
  meaning: string;
}

// 命令解释响应
export interface ExplainCommandResponse {
  explanation: string;
  parts: CommandPart[];
  safety_warning?: AISafetyWarning;
}

// 活跃会话信息
export interface ActiveSession {
  id: string;
  host: string;
  port: number;
  user: string;
  is_active: boolean;
}

// AI 面板模式
export type AIPanelMode = "generate" | "explain";

// AI 消息类型
export interface AIMessage {
  id: string;
  role: "user" | "assistant";
  content: string;
  command?: AICommandResponse;
  timestamp: number;
}
