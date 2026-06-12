import { clearToken, getToken } from "./auth";
import Client, { isAPIError } from "./client.gen";

function getBaseUrl(): string {
	if (typeof window !== "undefined") {
		return `${window.location.origin}/api`;
	}
	// Server-side: use ENCORE_URL env var (set by Nitro)
	return `${process.env.ENCORE_URL ?? "http://localhost:4000"}`;
}

let _client: Client | undefined;
export function getClient(): Client {
	if (!_client) {
		_client = new Client(getBaseUrl(), {
			auth: () => getToken() ?? "",
		});
	}
	return _client;
}

// Keep named export for backwards compatibility with existing query files
export const client = new Proxy({} as Client, {
	get(_target, prop) {
		return (getClient() as unknown as Record<string | symbol, unknown>)[prop];
	},
});

export function isApiError(
	err: unknown,
): err is { code: string; message: string; status: number } {
	return (
		typeof err === "object" &&
		err !== null &&
		"code" in err &&
		"message" in err &&
		"status" in err &&
		typeof (err as Record<string, unknown>).status === "number"
	);
}

export function handleUnauthorized(err: unknown): boolean {
	if (isAPIError(err) && err.status === 401) {
		clearToken();
		if (typeof window !== "undefined") {
			window.location.href = "/login";
		}
		return true;
	}
	return false;
}
