import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { client, isApiError } from "../lib/client";
import type { checker } from "../lib/client.gen";
import { queryKeys } from "./queryKeys";

const DEFAULT_PAGE_SIZE = 20;

export function useSchedulerHistory(
	subscriptionId: string,
	opts: { enabled?: boolean; Limit?: number } = {},
) {
	return useQuery({
		queryKey: queryKeys.schedulerHistory(subscriptionId),
		queryFn: () =>
			client.checker.ListJobs(subscriptionId, {
				Limit: opts.Limit ?? 8,
				Offset: 0,
			}),
		enabled: (opts.enabled ?? true) && !!subscriptionId,
		staleTime: 15_000,
	});
}

export function useLatestJobs() {
	return useQuery({
		queryKey: queryKeys.latestJobs(),
		queryFn: () => client.checker.LatestJobs(),
		staleTime: 15_000,
	});
}

export function useJobs(
	subscriptionId: string,
	opts: { Limit?: number; Offset?: number } = {},
) {
	return useQuery({
		queryKey: queryKeys.jobs(subscriptionId),
		queryFn: () =>
			client.checker.ListJobs(subscriptionId, {
				Limit: opts.Limit ?? DEFAULT_PAGE_SIZE,
				Offset: opts.Offset ?? 0,
			}),
		enabled: !!subscriptionId,
	});
}

export function useResults(subscriptionId: string, jobId: string | null) {
	return useQuery({
		queryKey: queryKeys.results(subscriptionId, jobId),
		queryFn: () =>
			client.checker.GetResults(subscriptionId, { JobID: jobId ?? "" }),
		enabled: !!subscriptionId,
		retry: (failureCount, err) => {
			// 404 = no completed checks yet; don't hammer.
			if (isApiError(err) && err.status === 404) return false;
			return failureCount < 2;
		},
	});
}

export function useExportLogs(
	subscriptionId: string,
	options: { enabled?: boolean } = {},
) {
	return useQuery({
		queryKey: queryKeys.exportLogs(subscriptionId),
		queryFn: () => client.checker.GetExportLogs(subscriptionId),
		enabled: (options.enabled ?? true) && !!subscriptionId,
		staleTime: 30_000,
	});
}

export function useTriggerCheck() {
	const qc = useQueryClient();
	return useMutation({
		mutationFn: (args: {
			subscriptionId: string;
			params?: checker.TriggerParams;
		}) =>
			client.checker.TriggerCheck(
				args.subscriptionId,
				args.params ?? {
					speed_test: false,
					upload_speed_test: false,
					media_apps: [],
					debug: false,
				},
			),
		onSuccess: (_data, vars) => {
			qc.invalidateQueries({ queryKey: queryKeys.jobs(vars.subscriptionId) });
			qc.invalidateQueries({ queryKey: queryKeys.latestJobs() });
		},
	});
}

export function useCancelCheck(subscriptionId: string) {
	const qc = useQueryClient();
	return useMutation({
		mutationFn: (jobId: string) => client.checker.CancelCheck(jobId),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: queryKeys.jobs(subscriptionId) });
			qc.invalidateQueries({ queryKey: queryKeys.latestJobs() });
		},
	});
}

export function useSetNodeEnabled(subscriptionId: string) {
	const qc = useQueryClient();
	return useMutation({
		mutationFn: (args: { nodeId: string; enabled: boolean }) =>
			client.checker.SetNodeEnabled(args.nodeId, { enabled: args.enabled }),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: queryKeys.results(subscriptionId) });
		},
	});
}
