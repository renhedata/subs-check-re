const TOKEN_KEY = "jwt_token";

function isBrowser(): boolean {
	return typeof window !== "undefined";
}

export function getToken(): string | null {
	if (!isBrowser()) return null;
	return localStorage.getItem(TOKEN_KEY) ?? sessionStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string, remember: boolean): void {
	if (!isBrowser()) return;
	clearToken();
	if (remember) {
		localStorage.setItem(TOKEN_KEY, token);
	} else {
		sessionStorage.setItem(TOKEN_KEY, token);
	}
}

export function clearToken(): void {
	if (!isBrowser()) return;
	localStorage.removeItem(TOKEN_KEY);
	sessionStorage.removeItem(TOKEN_KEY);
}

export function isAuthenticated(): boolean {
	if (!isBrowser()) return false;
	const token = getToken();
	if (!token) return false;
	try {
		const payload = JSON.parse(atob(token.split(".")[1]));
		if (payload.exp && payload.exp * 1000 < Date.now()) {
			clearToken();
			return false;
		}
		return true;
	} catch {
		clearToken();
		return false;
	}
}
