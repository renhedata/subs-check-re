import { createEnv } from "@t3-oss/env-core";

// Placeholder — extend with VITE_* client vars as needed.
export const env = createEnv({
	clientPrefix: "VITE_",
	client: {},
	runtimeEnv: import.meta.env,
});
