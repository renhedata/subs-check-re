import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { client } from "../lib/client";
import type { subscription } from "../lib/client.gen";
import { queryKeys } from "./queryKeys";

export function useSubscriptions() {
	return useQuery({
		queryKey: queryKeys.subscriptions(),
		queryFn: () => client.subscription.List(),
	});
}

export function useCreateSubscription() {
	const qc = useQueryClient();
	return useMutation({
		mutationFn: (p: subscription.CreateParams) => client.subscription.Create(p),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: queryKeys.subscriptions() });
		},
	});
}

export function useUpdateSubscription() {
	const qc = useQueryClient();
	return useMutation({
		mutationFn: (args: { id: string; params: subscription.UpdateParams }) =>
			client.subscription.Update(args.id, args.params),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: queryKeys.subscriptions() });
		},
	});
}

export function useDeleteSubscription() {
	const qc = useQueryClient();
	return useMutation({
		mutationFn: (id: string) => client.subscription.Delete(id),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: queryKeys.subscriptions() });
		},
	});
}
