export interface ProxyConfig {
  server: string;
  username?: string;
  password?: string;
}

export interface ExecuteRequest {
  script: string;
  proxy?: ProxyConfig;
  url?: string;
  timeout?: number;
  screenshot?: boolean;
}

export interface ExecuteResponse {
  ok: boolean;
  result: boolean;
  final_url?: string;
  title?: string;
  logs: string[];
  screenshot?: string;
  error?: string;
  duration_ms: number;
}

export interface PageContext {
  proxy?: ProxyConfig;
  url?: string;
}
