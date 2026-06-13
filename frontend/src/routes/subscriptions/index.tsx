import { createFileRoute, redirect } from "@tanstack/react-router";

export const Route = createFileRoute("/subscriptions/")({
	beforeLoad: () => {
		throw redirect({ to: "/" });
	},
});
