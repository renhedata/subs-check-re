// frontend/apps/web/src/lib/api.ts
import { getToken } from "./auth";

export class ApiError extends Error {
	constructor(
		public status: number,
		public code: string,
		message: string,
	) {
		super(message);
	}
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
	const token = getToken();
	const headers: Record<string, string> = {
		"Content-Type": "application/json",
		...(init.headers as Record<string, string>),
	};
	if (token) {
		headers.Authorization = `Bearer ${token}`;
	}

	const resp = await fetch(`/api${path}`, { ...init, headers });

	if (!resp.ok) {
		let errorCode = "UNKNOWN";
		let errorMsg = resp.statusText;
		try {
			const body = await resp.json();
			errorCode = body.code ?? errorCode;
			errorMsg = body.message ?? body.error ?? errorMsg;
		} catch {}
		throw new ApiError(resp.status, errorCode, errorMsg);
	}

	if (resp.status === 204) return undefined as T;
	return resp.json();
}

export const api = {
	get: <T>(path: string) => request<T>(path),
	post: <T>(path: string, body?: unknown) =>
		request<T>(path, { method: "POST", body: JSON.stringify(body) }),
	put: <T>(path: string, body?: unknown) =>
		request<T>(path, { method: "PUT", body: JSON.stringify(body) }),
	delete: <T>(path: string) => request<T>(path, { method: "DELETE" }),
};

// --- API types (mirrors backend) ---

export interface User {
	user_id: string;
	username: string;
}

export interface Subscription {
	id: string;
	user_id: string;
	name: string;
	url: string;
	enabled: boolean;
	cron_expr: string | null;
	created_at: string;
	last_run_at: string | null;
}

export interface CheckOptions {
	speed_test: boolean;
	media_apps: string[];
}

export interface CheckJob {
	id: string;
	subscription_id: string;
	status: "queued" | "running" | "completed" | "failed";
	total: number;
	progress: number;
	available: number;
	speed_test: boolean;
	media_apps: string[];
	created_at: string;
	finished_at?: string;
}

export interface NodeResult {
	node_id: string;
	node_name: string;
	node_type: string;
	alive: boolean;
	latency_ms: number;
	speed_kbps: number;
	country: string;
	ip: string;
	netflix: boolean;
	youtube: string;
	openai: boolean;
	claude: boolean;
	gemini: boolean;
	disney: boolean;
	tiktok: string;
}

export interface ScheduledJob {
	id: string;
	subscription_id: string;
	cron_expr: string;
	enabled: boolean;
	created_at: string;
}

export interface NotifyChannel {
	id: string;
	name: string;
	type: "webhook" | "telegram";
	config: Record<string, unknown>;
	enabled: boolean;
	created_at: string;
}

export interface UserSettings {
	speed_test_url: string;
	api_key?: string;
}
