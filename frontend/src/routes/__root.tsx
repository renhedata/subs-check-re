/// <reference types="vite/client" />

import {
	MutationCache,
	QueryCache,
	QueryClient,
	QueryClientProvider,
} from "@tanstack/react-query";
import {
	createRootRouteWithContext,
	HeadContent,
	Outlet,
	redirect,
	Scripts,
	useRouterState,
} from "@tanstack/react-router";
import type { ReactNode } from "react";
import { PlatformRulesProvider } from "@/components/platform-rules-context";
import { Sidebar } from "@/components/sidebar";
import { Toaster } from "@/components/ui/sonner";
import { isAuthenticated } from "@/lib/auth";
import { handleUnauthorized, isApiError } from "@/lib/client";
import appCss from "../styles.css?url";

// biome-ignore lint/complexity/noBannedTypes: intentionally empty context for TanStack Router
export type RouterAppContext = {};

const queryClient = new QueryClient({
	queryCache: new QueryCache({
		onError: (err) => handleUnauthorized(err),
	}),
	mutationCache: new MutationCache({
		onError: (err) => handleUnauthorized(err),
	}),
	defaultOptions: {
		queries: {
			staleTime: 30_000,
			retry: (failureCount, err) => {
				if (isApiError(err) && err.status === 401) return false;
				return failureCount < 2;
			},
		},
	},
});

export const Route = createRootRouteWithContext<RouterAppContext>()({
	beforeLoad: ({ location }) => {
		const authed = isAuthenticated();
		const isLoginPage = location.pathname === "/login";
		if (!authed && !isLoginPage) {
			throw redirect({ to: "/login" });
		}
		if (authed && isLoginPage) {
			throw redirect({ to: "/" });
		}
	},
	head: () => ({
		meta: [
			{ charSet: "utf-8" },
			{ name: "viewport", content: "width=device-width, initial-scale=1" },
			{ title: "subs-check" },
			{ name: "description", content: "Proxy subscription checker" },
		],
		links: [
			{ rel: "stylesheet", href: appCss },
			{ rel: "icon", href: "/favicon.ico" },
		],
	}),
	component: RootComponent,
});

function RootDocument({ children }: { children: ReactNode }) {
	return (
		<html lang="en">
			<head>
				{/* Inline theme detection — must run before first paint */}
				<script
					// biome-ignore lint/security/noDangerouslySetInnerHtml: intentional inline script for theme flash prevention
					dangerouslySetInnerHTML={{
						__html: `(()=>{var s=localStorage.getItem("theme"),t=s==="light"||s==="dark"?s:window.matchMedia("(prefers-color-scheme: dark)").matches?"dark":"light";document.documentElement.classList.toggle("dark",t==="dark")})()`,
					}}
				/>
				<HeadContent />
			</head>
			<body>
				{children}
				<Scripts />
			</body>
		</html>
	);
}

function RootComponent() {
	const { location } = useRouterState();
	const authed = isAuthenticated() && location.pathname !== "/login";

	return (
		<RootDocument>
			<QueryClientProvider client={queryClient}>
				{authed ? (
					<PlatformRulesProvider>
						<div className="flex h-screen overflow-hidden">
							<Sidebar />
							<main className="flex-1 overflow-y-auto px-6 py-6">
								<div className="mx-auto max-w-5xl">
									<Outlet />
								</div>
							</main>
						</div>
					</PlatformRulesProvider>
				) : (
					<div className="flex min-h-screen items-center justify-center">
						<Outlet />
					</div>
				)}
				<Toaster richColors />
			</QueryClientProvider>
		</RootDocument>
	);
}
