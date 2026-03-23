// frontend/apps/web/src/lib/client.ts
import { clearToken, getToken } from "./auth";
import Client, { isAPIError } from "./client.gen";

export const client = new Client(
	`${window.location.origin}/api`,
	// auth is called per-request (lazy), not captured at construction time
	{ auth: () => getToken() ?? "" },
);

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
		window.location.href = "/login";
		return true;
	}
	return false;
}
