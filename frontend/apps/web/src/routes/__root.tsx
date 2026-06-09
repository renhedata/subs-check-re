import { Toaster } from "@frontend/ui/components/sonner";
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
	useRouterState,
} from "@tanstack/react-router";
import { TanStackRouterDevtools } from "@tanstack/react-router-devtools";
import { MobileNav } from "@/components/mobile-nav";
import { PlatformRulesProvider } from "@/components/platform-rules-context";
import { Sidebar } from "@/components/sidebar";
import { isAuthenticated } from "@/lib/auth";
import { handleUnauthorized, isApiError } from "@/lib/client";

import "../index.css";

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
	// useRouterState subscribes this component to navigation changes,
	// ensuring isAuthenticated() is re-evaluated after login/logout
	const { location } = useRouterState();
	const authed = isAuthenticated() && location.pathname !== "/login";

	return (
		<QueryClientProvider client={queryClient}>
			<HeadContent />
			{authed ? (
				<PlatformRulesProvider>
					<div className="flex h-screen overflow-hidden">
						<Sidebar />
						<div className="flex min-w-0 flex-1 flex-col">
							<MobileNav />
							<main className="flex-1 overflow-y-auto px-4 py-4 md:px-6 md:py-6">
								<div className="mx-auto max-w-5xl">
									<Outlet />
								</div>
							</main>
						</div>
					</div>
				</PlatformRulesProvider>
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
