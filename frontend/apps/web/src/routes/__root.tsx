import { Toaster } from "@frontend/ui/components/sonner";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
	createRootRouteWithContext,
	HeadContent,
	Outlet,
} from "@tanstack/react-router";
import { TanStackRouterDevtools } from "@tanstack/react-router-devtools";

import { Sidebar } from "@/components/sidebar";
import { isAuthenticated } from "@/lib/auth";

import "../index.css";

// biome-ignore lint/complexity/noBannedTypes: intentionally empty context for TanStack Router
export type RouterAppContext = {};

const queryClient = new QueryClient({
	defaultOptions: {
		queries: { staleTime: 30_000, retry: 2 },
	},
});

export const Route = createRootRouteWithContext<RouterAppContext>()({
	component: RootComponent,
	head: () => ({
		meta: [
			{ title: "subs-check" },
			{ name: "description", content: "Proxy subscription checker" },
		],
		links: [{ rel: "icon", href: "/favicon.ico" }],
	}),
});

function RootComponent() {
	const authed = isAuthenticated();

	return (
		<QueryClientProvider client={queryClient}>
			<HeadContent />
			{authed ? (
				<div className="flex h-screen overflow-hidden">
					<Sidebar />
					<main className="flex-1 overflow-y-auto px-6 py-6">
						<div className="mx-auto max-w-5xl">
							<Outlet />
						</div>
					</main>
				</div>
			) : (
				<div className="flex min-h-screen items-center justify-center">
					<Outlet />
				</div>
			)}
			<Toaster richColors />
			<TanStackRouterDevtools position="bottom-left" />
		</QueryClientProvider>
	);
}
