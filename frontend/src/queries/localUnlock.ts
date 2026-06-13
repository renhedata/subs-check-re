import { useQuery } from "@tanstack/react-query";
import { client } from "../lib/client";
import { queryKeys } from "./queryKeys";

export function useLocalUnlock() {
	return useQuery({
		queryKey: queryKeys.localUnlock(),
		queryFn: () => client.checker.GetLocalUnlock(),
		staleTime: 5 * 60 * 1000,
		retry: false,
	});
}
