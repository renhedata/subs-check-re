import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { setToken } from "@/lib/auth";
import { isApiError } from "@/lib/client";
import { useLogin, useRegister } from "@/queries";

export const Route = createFileRoute("/login")({
	component: LoginPage,
});

function LoginPage() {
	const navigate = useNavigate();
	const [mode, setMode] = useState<"login" | "register">("login");
	const [username, setUsername] = useState("");
	const [password, setPassword] = useState("");
	const [remember, setRemember] = useState(false);
	const [inviteCode, setInviteCode] = useState("");

	const loginMut = useLogin();
	const registerMut = useRegister();
	const pending = loginMut.isPending || registerMut.isPending;

	function submit(e: React.FormEvent) {
		e.preventDefault();
		if (!username || !password) return;
		const onError = (err: unknown) =>
			toast.error(
				isApiError(err)
					? err.message
					: mode === "login"
						? "Login failed"
						: "Registration failed",
			);
		if (mode === "login") {
			loginMut.mutate(
				{ username, password, remember },
				{
					onSuccess: (resp) => {
						setToken(resp.token, remember);
						navigate({ to: "/" });
					},
					onError,
				},
			);
		} else {
			registerMut.mutate(
				{ username, password, invite_code: inviteCode },
				{
					onSuccess: () => {
						toast.success("Account created — please sign in");
						setMode("login");
					},
					onError,
				},
			);
		}
	}

	return (
		<div className="w-full max-w-sm px-4">
			<form
				onSubmit={submit}
				className="rounded-xl border border-border bg-card p-6 shadow-[var(--shadow-popover)]"
			>
				<div className="mx-auto mb-3 flex size-10 items-center justify-center rounded-xl bg-primary font-bold text-lg text-primary-foreground">
					S
				</div>
				<h1 className="text-center font-semibold text-[15px] text-foreground">
					subs-check
				</h1>
				<p className="mt-0.5 mb-5 text-center text-muted-foreground text-xs">
					{mode === "login" ? "Sign in to your account" : "Create an account"}
				</p>

				<div className="space-y-3">
					<div className="space-y-1.5">
						<Label htmlFor="username" className="text-xs">
							Username
						</Label>
						<Input
							id="username"
							value={username}
							autoComplete="username"
							onChange={(e) => setUsername(e.target.value)}
						/>
					</div>
					<div className="space-y-1.5">
						<Label htmlFor="password" className="text-xs">
							Password
						</Label>
						<Input
							id="password"
							type="password"
							value={password}
							autoComplete={
								mode === "login" ? "current-password" : "new-password"
							}
							onChange={(e) => setPassword(e.target.value)}
						/>
					</div>
					{mode === "register" ? (
						<div className="space-y-1.5">
							<Label htmlFor="invite" className="text-xs">
								Invite code
							</Label>
							<Input
								id="invite"
								value={inviteCode}
								onChange={(e) => setInviteCode(e.target.value)}
							/>
						</div>
					) : null}
					{mode === "login" ? (
						<label className="flex cursor-pointer items-center gap-2 text-muted-foreground text-xs">
							<Checkbox
								checked={remember}
								onCheckedChange={(v) => setRemember(v === true)}
							/>
							Remember me
						</label>
					) : null}
				</div>

				<Button
					type="submit"
					variant="success"
					className="mt-5 w-full"
					loading={pending}
					disabled={!username || !password || (mode === "register" && !inviteCode)}
				>
					{mode === "login" ? "Sign in" : "Create account"}
				</Button>

				<button
					type="button"
					onClick={() => setMode(mode === "login" ? "register" : "login")}
					className="mt-4 w-full text-center text-primary text-xs hover:underline"
				>
					{mode === "login"
						? "Create an account"
						: "Already have an account? Sign in"}
				</button>
			</form>
		</div>
	);
}
