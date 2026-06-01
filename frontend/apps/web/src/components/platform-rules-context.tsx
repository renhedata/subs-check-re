import { createContext, useContext, useMemo } from "react";
import type { checker } from "@/lib/client.gen";
import { useRules } from "@/queries";

type PlatformRule = checker.PlatformRule;

const PlatformRulesContext = createContext<Map<string, PlatformRule>>(
	new Map(),
);

export function PlatformRulesProvider({
	children,
}: {
	children: React.ReactNode;
}) {
	const { data } = useRules();

	const ruleMap = useMemo(
		() =>
			new Map<string, PlatformRule>((data?.rules ?? []).map((r) => [r.key, r])),
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
