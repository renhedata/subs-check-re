// frontend/apps/web/src/lib/auth.ts
const TOKEN_KEY = "jwt_token";

export function getToken(): string | null {
	return localStorage.getItem(TOKEN_KEY) ?? sessionStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string, remember: boolean): void {
	clearToken();
	if (remember) {
		localStorage.setItem(TOKEN_KEY, token);
	} else {
		sessionStorage.setItem(TOKEN_KEY, token);
	}
}

export function clearToken(): void {
	localStorage.removeItem(TOKEN_KEY);
	sessionStorage.removeItem(TOKEN_KEY);
}

export function isAuthenticated(): boolean {
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
