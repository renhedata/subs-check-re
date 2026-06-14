import { Link, useRouterState } from "@tanstack/react-router";
import { Clock, List, Radar, Settings } from "lucide-react";
import { cn } from "@/lib/utils";

const TABS = [
	{ to: "/", label: "Subs", icon: List, exact: true, matchPrefix: "/" },
	{
		to: "/scheduler",
		label: "Scheduler",
		icon: Clock,
		exact: true,
		matchPrefix: "/scheduler",
	},
	{
		to: "/rules",
		label: "Rules",
		icon: Radar,
		exact: false,
		matchPrefix: "/rules",
	},
	{
		to: "/settings/general",
		label: "Settings",
		icon: Settings,
		exact: false,
		matchPrefix: "/settings",
	},
] as const;

export function MobileTabbar() {
	const pathname = useRouterState({ select: (s) => s.location.pathname });
	return (
		<nav className="flex shrink-0 border-border border-t bg-card pb-[env(safe-area-inset-bottom)] md:hidden">
			{TABS.map(({ to, label, icon: Icon, exact, matchPrefix }) => {
				const active = exact
					? pathname === matchPrefix
					: pathname.startsWith(matchPrefix);
				return (
					<Link
						key={to}
						to={to}
						className={cn(
							"flex min-h-12 flex-1 flex-col items-center justify-center gap-0.5 text-[10px] transition-colors",
							active ? "text-primary" : "text-muted-foreground",
						)}
					>
						<Icon size={18} strokeWidth={1.75} />
						{label}
					</Link>
				);
			})}
		</nav>
	);
}
