import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Toaster } from "@frontend/ui/components/sonner";
import { HeadContent, Outlet, createRootRouteWithContext } from "@tanstack/react-router";
import { TanStackRouterDevtools } from "@tanstack/react-router-devtools";

import Header from "@/components/header";
import { ThemeProvider } from "@/components/theme-provider";

import "../index.css";

export interface RouterAppContext {}

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { staleTime: 30_000, retry: 2 },
  },
});

export const Route = createRootRouteWithContext<RouterAppContext>()({
  component: RootComponent,
  head: () => ({
    meta: [
      { title: "subs-check-re" },
      { name: "description", content: "Proxy subscription checker" },
    ],
    links: [{ rel: "icon", href: "/favicon.ico" }],
  }),
});

function RootComponent() {
  return (
    <QueryClientProvider client={queryClient}>
      <HeadContent />
      <ThemeProvider
        attribute="class"
        defaultTheme="dark"
        disableTransitionOnChange
        storageKey="vite-ui-theme"
      >
        <div className="grid grid-rows-[auto_1fr] h-svh">
          <Header />
          <main className="container mx-auto max-w-5xl px-4 py-6 overflow-y-auto">
            <Outlet />
          </main>
        </div>
        <Toaster richColors />
      </ThemeProvider>
      <TanStackRouterDevtools position="bottom-left" />
    </QueryClientProvider>
  );
}
