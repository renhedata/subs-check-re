import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { client } from "../lib/client";
import type { notify } from "../lib/client.gen";
import { queryKeys } from "./queryKeys";

export function useNotifyChannels() {
	return useQuery({
		queryKey: queryKeys.notifyChannels(),
		queryFn: () => client.notify.ListChannels(),
	});
}

export function useCreateNotifyChannel() {
	const qc = useQueryClient();
	return useMutation({
		mutationFn: (p: notify.CreateChannelParams) =>
			client.notify.CreateChannel(p),
		onSuccess: () =>
			qc.invalidateQueries({ queryKey: queryKeys.notifyChannels() }),
	});
}

export function useUpdateNotifyChannel() {
	const qc = useQueryClient();
	return useMutation({
		mutationFn: (args: { id: string; params: notify.UpdateChannelParams }) =>
			client.notify.UpdateChannel(args.id, args.params),
		onSuccess: () =>
			qc.invalidateQueries({ queryKey: queryKeys.notifyChannels() }),
	});
}

export function useDeleteNotifyChannel() {
	const qc = useQueryClient();
	return useMutation({
		mutationFn: (id: string) => client.notify.DeleteChannel(id),
		onSuccess: () =>
			qc.invalidateQueries({ queryKey: queryKeys.notifyChannels() }),
	});
}

export function useTestNotifyChannel() {
	return useMutation({
		mutationFn: (args: { id: string; params: notify.TestChannelParams }) =>
			client.notify.TestChannel(args.id, args.params),
	});
}
