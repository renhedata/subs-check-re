import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { client } from "../lib/client";
import type { scheduler } from "../lib/client.gen";
import { queryKeys } from "./queryKeys";

export function useScheduledJobs() {
	return useQuery({
		queryKey: queryKeys.scheduler(),
		queryFn: () => client.scheduler.List(),
	});
}

export function useCreateScheduledJob() {
	const qc = useQueryClient();
	return useMutation({
		mutationFn: (p: scheduler.CreateParams) => client.scheduler.Create(p),
		onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.scheduler() }),
	});
}

export function useDeleteScheduledJob() {
	const qc = useQueryClient();
	return useMutation({
		mutationFn: (id: string) => client.scheduler.Delete(id),
		onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.scheduler() }),
	});
}
