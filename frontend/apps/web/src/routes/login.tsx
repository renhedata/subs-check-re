import { useState } from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { toast } from "sonner";
import { Lock, Loader2 } from "lucide-react";
import { Input } from "@frontend/ui/components/input";
import { Label } from "@frontend/ui/components/label";

import { api, ApiError } from "@/lib/api";
import { setToken } from "@/lib/auth";

export const Route = createFileRoute("/login")({
	component: LoginPage,
});

function LoginPage() {
	const navigate = useNavigate();
	const [mode, setMode] = useState<"login" | "register">("login");
	const [username, setUsername] = useState("");
	const [password, setPassword] = useState("");
	const [loading, setLoading] = useState(false);

	async function handleSubmit(e: React.FormEvent) {
		e.preventDefault();
		setLoading(true);
		try {
			if (mode === "register") {
				await api.post("/auth/register", { username, password });
				toast.success("Account created — please log in");
				setMode("login");
			} else {
				const resp = await api.post<{ token: string }>("/auth/login", { username, password });
				setToken(resp.token);
				navigate({ to: "/" });
			}
		} catch (err) {
			toast.error(err instanceof ApiError ? err.message : "Something went wrong");
		} finally {
			setLoading(false);
		}
	}

	return (
		<div
			className="w-full max-w-sm rounded-lg border p-6"
			style={{ background: "#161b22", borderColor: "#30363d" }}
		>
			<div className="mb-6 flex items-center gap-2">
				<Lock size={16} strokeWidth={1.5} style={{ color: "#58a6ff" }} />
				<h1 className="text-base font-semibold text-[#f0f6fc]">
					{mode === "login" ? "Sign in" : "Create account"}
				</h1>
			</div>

			<form onSubmit={handleSubmit} className="space-y-4">
				<div className="space-y-1.5">
					<Label htmlFor="username" className="text-xs text-[#8b949e]">Username</Label>
					<Input
						id="username"
						value={username}
						onChange={(e) => setUsername(e.target.value)}
						required
						className="h-8 text-sm"
					/>
				</div>
				<div className="space-y-1.5">
					<Label htmlFor="password" className="text-xs text-[#8b949e]">Password</Label>
					<Input
						id="password"
						type="password"
						value={password}
						onChange={(e) => setPassword(e.target.value)}
						required
						className="h-8 text-sm"
					/>
				</div>
				<button
					type="submit"
					disabled={loading}
					className="flex w-full items-center justify-center gap-2 rounded-md py-2 text-sm font-medium text-white transition-opacity hover:opacity-90 disabled:opacity-50"
					style={{ background: "#238636" }}
				>
					{loading ? (
						<Loader2 size={14} className="animate-spin" />
					) : mode === "login" ? (
						"Sign in"
					) : (
						"Register"
					)}
				</button>

				<p className="text-center text-xs" style={{ color: "#8b949e" }}>
					{mode === "login" ? "No account? " : "Have an account? "}
					<button
						type="button"
						className="underline hover:text-[#f0f6fc] transition-colors"
						style={{ color: "#58a6ff" }}
						onClick={() => setMode(mode === "login" ? "register" : "login")}
					>
						{mode === "login" ? "Register" : "Sign in"}
					</button>
				</p>
			</form>
		</div>
	);
}
