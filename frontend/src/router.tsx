import { createRouter as createTanStackRouter } from "@tanstack/react-router";
import Loader from "@/components/loader";
import type { RouterAppContext } from "./routes/__root";
import { routeTree } from "./routeTree.gen";

export function createRouter() {
	return createTanStackRouter({
		routeTree,
		defaultPreload: "intent",
		defaultPendingComponent: () => <Loader />,
		context: {} satisfies RouterAppContext,
	});
}

// Required by TanStack Start SSR: routerEntry must export getRouter
export function getRouter() {
	return Promise.resolve(createRouter());
}

declare module "@tanstack/react-router" {
	interface Register {
		router: ReturnType<typeof createRouter>;
	}
}
