import { useQuery } from "@tanstack/react-query";
import { client } from "../lib/client";
import { queryKeys } from "./queryKeys";

// useNodes returns the subscription's persisted nodes with their latest-known
// results. Populated right after fetch/import — independent of any check job.
export function useNodes(subscriptionId: string) {
	return useQuery({
		queryKey: queryKeys.nodes(subscriptionId),
		queryFn: () => client.checker.ListNodes(subscriptionId),
		enabled: !!subscriptionId,
	});
}
