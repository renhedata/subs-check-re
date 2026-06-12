import { StartClient } from "@tanstack/react-start/client";
import { StrictMode } from "react";
import { hydrateRoot } from "react-dom/client";

hydrateRoot(
	document,
	<StrictMode>
		{/* NOTE: StartClient resolves the router via the Vite plugin's #tanstack-router-entry
		// virtual module, which maps to the `getRouter()` export in ./router.tsx.
		// Do not remove getRouter() from router.tsx. */}
		<StartClient />
	</StrictMode>,
);
