import { Link, useNavigate, useRouterState } from "@tanstack/react-router";
import {
	Bell,
	Clock,
	LayoutDashboard,
	List,
	LogOut,
	Moon,
	Settings,
	Sun,
	User,
} from "lucide-react";
import { clearToken } from "@/lib/auth";
import { useTheme } from "@/lib/theme";

const NAV_ITEMS = [
	{ to: "/", label: "Dashboard", icon: LayoutDashboard, exact: true },
	{ to: "/subscriptions", label: "Subscriptions", icon: List, exact: false },
	{ to: "/scheduler", label: "Scheduler", icon: Clock, exact: true },
	{ to: "/settings/notify", label: "Notify", icon: Bell, exact: false },
] as const;

const BOTTOM_ITEMS = [
	{ to: "/settings/general", label: "Settings", icon: Settings, exact: false },
] as const;

function NavItem({
	to,
	label,
	icon: Icon,
	exact,
}: {
	to: string;
	label: string;
	icon: React.ElementType;
	exact: boolean;
}) {
	const pathname = useRouterState({ select: (s) => s.location.pathname });
	const isActive = exact ? pathname === to : pathname.startsWith(to);

	return (
		<Link
			to={to}
			className={[
				"flex items-center gap-2.5 rounded-md px-2.5 py-2 text-sm transition-colors",
				isActive
					? "border font-medium text-foreground"
					: "border border-transparent text-muted-foreground hover:bg-white/5 hover:text-foreground",
			].join(" ")}
			style={
				isActive
					? {
							background: "var(--color-active-bg)",
							borderColor: "var(--color-active-border)",
						}
					: undefined
			}
		>
			<Icon size={14} strokeWidth={1.5} />
			{label}
		</Link>
	);
}

export function Sidebar() {
	const navigate = useNavigate();
	const { theme, toggle } = useTheme();

	function logout() {
		clearToken();
		navigate({ to: "/login" });
	}

	return (
		<aside className="flex h-screen w-[220px] flex-shrink-0 flex-col border-r bg-card border-border">
			{/* Logo */}
			<div className="flex items-center gap-2.5 border-b border-border px-4 py-3">
				<div
					className="flex h-6 w-6 items-center justify-center rounded-md font-bold text-xs"
					style={{
						background: "var(--primary)",
						color: "var(--primary-foreground)",
					}}
				>
					S
				</div>
				<span className="font-semibold text-foreground text-sm">
					subs-check
				</span>
			</div>

			{/* Primary nav */}
			<nav className="flex flex-1 flex-col gap-0.5 p-2">
				{NAV_ITEMS.map((item) => (
					<NavItem key={item.to} {...item} />
				))}
			</nav>

			{/* Bottom nav */}
			<div className="flex flex-col gap-0.5 border-t border-border p-2">
				{BOTTOM_ITEMS.map((item) => (
					<NavItem key={item.to} {...item} />
				))}

				{/* Theme toggle */}
				<button
					type="button"
					onClick={toggle}
					className="flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-muted-foreground text-sm transition-colors hover:bg-white/5 hover:text-foreground"
					title={theme === "dark" ? "Switch to light mode" : "Switch to dark mode"}
				>
					{theme === "dark" ? (
						<Sun size={14} strokeWidth={1.5} />
					) : (
						<Moon size={14} strokeWidth={1.5} />
					)}
					{theme === "dark" ? "Light mode" : "Dark mode"}
				</button>

				<button
					type="button"
					onClick={logout}
					className="flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-muted-foreground text-sm transition-colors hover:bg-white/5 hover:text-foreground"
				>
					<div
						className="flex h-5 w-5 items-center justify-center rounded-full bg-secondary"
					>
						<User size={10} strokeWidth={1.5} />
					</div>
					{/* TODO: replace "admin" with actual username once user-profile API is available */}
					<span className="flex-1 text-left">admin</span>
					<LogOut size={12} strokeWidth={1.5} />
				</button>
			</div>
		</aside>
	);
}
