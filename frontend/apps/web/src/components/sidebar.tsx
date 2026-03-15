import { Link, useNavigate, useRouterState } from "@tanstack/react-router";
import {
	Bell,
	Clock,
	LayoutDashboard,
	List,
	LogOut,
	Settings,
	User,
} from "lucide-react";
import { clearToken } from "@/lib/auth";

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
					? "border font-medium text-[#f0f6fc]"
					: "border border-transparent text-[#8b949e] hover:bg-white/5 hover:text-[#e6edf3]",
			].join(" ")}
			style={
				isActive
					? {
							background: "rgba(31,111,235,0.13)",
							borderColor: "rgba(31,111,235,0.27)",
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

	function logout() {
		clearToken();
		navigate({ to: "/login" });
	}

	return (
		<aside
			className="flex h-screen w-[220px] flex-shrink-0 flex-col border-r"
			style={{ background: "#161b22", borderColor: "#30363d" }}
		>
			{/* Logo */}
			<div
				className="flex items-center gap-2.5 border-b px-4 py-3"
				style={{ borderColor: "#30363d" }}
			>
				<div
					className="flex h-6 w-6 items-center justify-center rounded-md font-bold text-xs"
					style={{ background: "#58a6ff", color: "#0d1117" }}
				>
					S
				</div>
				<span className="font-semibold text-[#f0f6fc] text-sm">subs-check</span>
			</div>

			{/* Primary nav */}
			<nav className="flex flex-1 flex-col gap-0.5 p-2">
				{NAV_ITEMS.map((item) => (
					<NavItem key={item.to} {...item} />
				))}
			</nav>

			{/* Bottom nav */}
			<div
				className="flex flex-col gap-0.5 border-t p-2"
				style={{ borderColor: "#30363d" }}
			>
				{BOTTOM_ITEMS.map((item) => (
					<NavItem key={item.to} {...item} />
				))}
				<button
					type="button"
					onClick={logout}
					className="flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-[#8b949e] text-sm transition-colors hover:bg-white/5 hover:text-[#e6edf3]"
				>
					<div
						className="flex h-5 w-5 items-center justify-center rounded-full"
						style={{ background: "#30363d" }}
					>
						<User size={10} strokeWidth={1.5} className="text-[#8b949e]" />
					</div>
					{/* TODO: replace "admin" with actual username once user-profile API is available */}
					<span className="flex-1 text-left">admin</span>
					<LogOut size={12} strokeWidth={1.5} />
				</button>
			</div>
		</aside>
	);
}
