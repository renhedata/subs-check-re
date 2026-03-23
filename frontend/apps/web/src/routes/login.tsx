import { Input } from "@frontend/ui/components/input";
import { Label } from "@frontend/ui/components/label";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { Loader2, Lock } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { setToken } from "@/lib/auth";
import { client, isApiError } from "@/lib/client";

export const Route = createFileRoute("/login")({
	component: LoginPage,
});

function LoginPage() {
	const navigate = useNavigate();
	const [mode, setMode] = useState<"login" | "register">("login");
	const [username, setUsername] = useState("");
	const [password, setPassword] = useState("");
	const [remember, setRemember] = useState(false);
	const [loading, setLoading] = useState(false);

	async function handleSubmit(e: React.FormEvent) {
		e.preventDefault();
		setLoading(true);
		try {
			if (mode === "register") {
				await client.auth.Register({ username, password });
				toast.success("Account created — please log in");
				setMode("login");
			} else {
				const resp = await client.auth.Login({ username, password });
				setToken(resp.token, remember);
				navigate({ to: "/" });
			}
		} catch (err) {
			toast.error(isApiError(err) ? err.message : "Something went wrong");
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
				<h1 className="font-semibold text-[#f0f6fc] text-base">
					{mode === "login" ? "Sign in" : "Create account"}
				</h1>
			</div>

			<form onSubmit={handleSubmit} className="space-y-4">
				<div className="space-y-1.5">
					<Label htmlFor="username" className="text-[#8b949e] text-xs">
						Username
					</Label>
					<Input
						id="username"
						value={username}
						onChange={(e) => setUsername(e.target.value)}
						required
						className="h-8 text-sm"
					/>
				</div>
				<div className="space-y-1.5">
					<Label htmlFor="password" className="text-[#8b949e] text-xs">
						Password
					</Label>
					<Input
						id="password"
						type="password"
						value={password}
						onChange={(e) => setPassword(e.target.value)}
						required
						className="h-8 text-sm"
					/>
				</div>
				{mode === "login" && (
					<label className="flex cursor-pointer items-center gap-2">
						<input
							type="checkbox"
							checked={remember}
							onChange={(e) => setRemember(e.target.checked)}
							className="h-3.5 w-3.5 rounded accent-[#238636]"
						/>
						<span className="text-[#8b949e] text-xs">Remember me</span>
					</label>
				)}
				<button
					type="submit"
					disabled={loading}
					className="flex w-full items-center justify-center gap-2 rounded-md py-2 font-medium text-sm text-white transition-opacity hover:opacity-90 disabled:opacity-50"
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
						className="underline transition-colors hover:text-[#f0f6fc]"
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
