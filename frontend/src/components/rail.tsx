import { Link, useNavigate, useRouterState } from "@tanstack/react-router";
import { Clock, List, LogOut, Moon, Radar, Settings, Sun } from "lucide-react";
import type React from "react";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Tooltip } from "@/components/ui/tooltip";
import { clearToken } from "@/lib/auth";
import { useTheme } from "@/lib/theme";
import { cn } from "@/lib/utils";
import { useMe } from "@/queries";

const NAV_ITEMS = [
	{
		to: "/",
		label: "Subscriptions",
		icon: List,
		exact: true,
		matchPrefix: "/",
	},
	{
		to: "/scheduler",
		label: "Scheduler",
		icon: Clock,
		exact: true,
		matchPrefix: "/scheduler",
	},
	{
		to: "/rules",
		label: "Platform Rules",
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

function RailLink({
	to,
	label,
	icon: Icon,
	exact,
	matchPrefix,
}: {
	to: string;
	label: string;
	icon: React.ElementType;
	exact: boolean;
	matchPrefix: string;
}) {
	const pathname = useRouterState({ select: (s) => s.location.pathname });
	const isActive = exact
		? pathname === matchPrefix
		: pathname.startsWith(matchPrefix);
	return (
		<Tooltip content={label} side="right">
			<Link
				to={to}
				aria-label={label}
				className={cn(
					"flex size-9 items-center justify-center rounded-lg border border-transparent text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground",
					isActive && "text-foreground",
				)}
				style={
					isActive
						? {
								background: "var(--color-active-bg)",
								borderColor: "var(--color-active-border)",
							}
						: undefined
				}
			>
				<Icon size={16} strokeWidth={1.75} />
			</Link>
		</Tooltip>
	);
}

export function Rail() {
	const navigate = useNavigate();
	const { theme, toggle } = useTheme();
	const meQuery = useMe();
	const username = meQuery.data?.display_name || meQuery.data?.username || "…";

	function logout() {
		clearToken();
		navigate({ to: "/login" });
	}

	return (
		<aside className="hidden h-screen w-14 shrink-0 flex-col items-center gap-1.5 border-border border-r bg-card py-3 md:flex">
			<Link
				to="/"
				aria-label="subs-check home"
				className="mb-2 flex size-8 items-center justify-center rounded-lg bg-primary font-bold text-primary-foreground text-sm"
			>
				S
			</Link>

			{NAV_ITEMS.map((item) => (
				<RailLink key={item.to} {...item} />
			))}

			<div className="flex-1" />

			<Tooltip
				content={theme === "dark" ? "Light mode" : "Dark mode"}
				side="right"
			>
				<button
					type="button"
					onClick={toggle}
					aria-label="Toggle theme"
					className="flex size-9 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
				>
					{theme === "dark" ? (
						<Sun size={16} strokeWidth={1.75} />
					) : (
						<Moon size={16} strokeWidth={1.75} />
					)}
				</button>
			</Tooltip>

			<DropdownMenu>
				<DropdownMenuTrigger
					aria-label="Account menu"
					className="flex size-8 items-center justify-center rounded-full border border-border bg-secondary font-medium text-foreground text-xs outline-none transition-colors hover:bg-muted"
				>
					{username.slice(0, 1).toUpperCase()}
				</DropdownMenuTrigger>
				<DropdownMenuContent side="right" align="end" className="min-w-40">
					<div className="px-2 py-1.5 text-muted-foreground text-xs">
						Signed in as <span className="font-medium">{username}</span>
					</div>
					<DropdownMenuItem onClick={() => navigate({ to: "/settings/account" })}>
						<Settings size={14} /> Account settings
					</DropdownMenuItem>
					<DropdownMenuItem onClick={logout}>
						<LogOut size={14} /> Log out
					</DropdownMenuItem>
				</DropdownMenuContent>
			</DropdownMenu>
		</aside>
	);
}
