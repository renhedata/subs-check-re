import { defineEventHandler, proxyRequest } from "h3";

export default defineEventHandler((event) => {
	const base = process.env.ENCORE_URL ?? "http://localhost:4000";
	const target = base + event.path.replace(/^\/api/, "");
	return proxyRequest(event, target);
});
