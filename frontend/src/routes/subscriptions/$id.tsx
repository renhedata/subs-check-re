import { createFileRoute, redirect } from "@tanstack/react-router";

export const Route = createFileRoute("/subscriptions/$id")({
	beforeLoad: ({ params }) => {
		throw redirect({ to: "/", search: { sub: params.id } });
	},
});
