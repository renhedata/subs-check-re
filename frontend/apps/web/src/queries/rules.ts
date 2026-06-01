import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { client } from "../lib/client";
import type { checker } from "../lib/client.gen";
import { queryKeys } from "./queryKeys";

export function useRules() {
	return useQuery({
		queryKey: queryKeys.platformRules(),
		queryFn: () => client.checker.ListRules(),
	});
}

export function useTestNodes() {
	return useQuery({
		queryKey: queryKeys.testNodes(),
		queryFn: () => client.checker.ListTestNodes(),
	});
}

export function useCreateRule() {
	const qc = useQueryClient();
	return useMutation({
		mutationFn: (p: checker.CreateRuleParams) => client.checker.CreateRule(p),
		onSuccess: () =>
			qc.invalidateQueries({ queryKey: queryKeys.platformRules() }),
	});
}

export function useUpdateRule() {
	const qc = useQueryClient();
	return useMutation({
		mutationFn: (args: { id: string; params: checker.UpdateRuleParams }) =>
			client.checker.UpdateRule(args.id, args.params),
		onSuccess: () =>
			qc.invalidateQueries({ queryKey: queryKeys.platformRules() }),
	});
}

export function useDeleteRule() {
	const qc = useQueryClient();
	return useMutation({
		mutationFn: (id: string) => client.checker.DeleteRule(id),
		onSuccess: () =>
			qc.invalidateQueries({ queryKey: queryKeys.platformRules() }),
	});
}

export function useTestRule() {
	return useMutation({
		mutationFn: (p: checker.TestRuleParams) => client.checker.TestRule(p),
	});
}
