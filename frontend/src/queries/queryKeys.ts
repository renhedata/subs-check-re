// Centralized query key factories. Every useQuery / invalidate call should pull
// its key from here so that invalidations always match and stale keys never drift.
export const queryKeys = {
	me: () => ["me"] as const,
	apiKey: () => ["api-key"] as const,
	settings: () => ["settings"] as const,
	subscriptions: () => ["subscriptions"] as const,
	jobs: (subscriptionId: string) => ["jobs", subscriptionId] as const,
	results: (subscriptionId: string, jobId?: string | null) =>
		jobId
			? (["results", subscriptionId, jobId] as const)
			: (["results", subscriptionId] as const),
	exportLogs: (subscriptionId: string) =>
		["export-logs", subscriptionId] as const,
	platformRules: () => ["platform-rules"] as const,
	testNodes: () => ["test-nodes"] as const,
	notifyChannels: () => ["notify-channels"] as const,
	latestJobs: () => ["latest-jobs"] as const,
	scheduler: () => ["scheduler"] as const,
	schedulerHistory: (subscriptionId: string) =>
		["scheduler-history", subscriptionId] as const,
	localUnlock: () => ["local-unlock"] as const,
	nodes: (subscriptionId: string) => ["nodes", subscriptionId] as const,
};
