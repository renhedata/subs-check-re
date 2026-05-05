import { useQuery } from "@tanstack/react-query";
import { createContext, useContext, useMemo } from "react";
import { client } from "@/lib/client";
import type { checker } from "@/lib/client.gen";

type PlatformRule = checker.PlatformRule;

const PlatformRulesContext = createContext<Map<string, PlatformRule>>(new Map());

export function PlatformRulesProvider({ children }: { children: React.ReactNode }) {
	const { data } = useQuery({
		queryKey: ["platform-rules"],
		queryFn: () => client.checker.ListRules(),
		staleTime: 60_000,
	});

	const ruleMap = useMemo(
		() => new Map<string, PlatformRule>((data?.rules ?? []).map((r) => [r.key, r])),
		[data],
	);

	return (
		<PlatformRulesContext.Provider value={ruleMap}>
			{children}
		</PlatformRulesContext.Provider>
	);
}

export function usePlatformRules(): Map<string, PlatformRule> {
	return useContext(PlatformRulesContext);
}
