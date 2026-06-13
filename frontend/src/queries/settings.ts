import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { client } from "../lib/client";
import type { settings } from "../lib/client.gen";
import { queryKeys } from "./queryKeys";

export function useSettings() {
	return useQuery({
		queryKey: queryKeys.settings(),
		queryFn: () => client.settings.GetSettings(),
	});
}

export function useUpdateSettings() {
	const qc = useQueryClient();
	return useMutation({
		mutationFn: (p: settings.UserSettings) => client.settings.UpdateSettings(p),
		onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.settings() }),
	});
}

export function useAPIKey(options: { enabled?: boolean } = {}) {
	return useQuery({
		queryKey: queryKeys.apiKey(),
		queryFn: () => client.settings.GetAPIKey(),
		enabled: options.enabled ?? true,
		staleTime: Number.POSITIVE_INFINITY,
	});
}

export function useRegenerateAPIKey() {
	const qc = useQueryClient();
	return useMutation({
		mutationFn: () => client.settings.RegenerateAPIKey(),
		onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.apiKey() }),
	});
}
