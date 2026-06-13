import { createFileRoute, Link, Outlet } from "@tanstack/react-router";
import { cn } from "@/lib/utils";

export const Route = createFileRoute("/settings")({
	component: SettingsLayout,
});

const TABS = [
	{ to: "/settings/general" as const, label: "General" },
	{ to: "/settings/notify" as const, label: "Notifications" },
	{ to: "/settings/platforms" as const, label: "Platform Rules" },
	{ to: "/settings/export" as const, label: "Export API" },
];

function SettingsLayout() {
	return (
		<div className="h-full overflow-y-auto">
			<div className="mx-auto max-w-3xl space-y-5 p-4 pb-8 md:p-6">
				<h1 className="font-semibold text-foreground text-lg">Settings</h1>
				<nav className="flex gap-1 overflow-x-auto border-border border-b">
					{TABS.map((tab) => (
						<Link
							key={tab.to}
							to={tab.to}
							activeProps={{
								className: "border-primary font-medium text-foreground",
							}}
							inactiveProps={{
								className:
									"border-transparent text-muted-foreground hover:text-foreground",
							}}
							className={cn(
								"-mb-px shrink-0 border-b-2 px-3.5 py-2 text-sm transition-colors",
							)}
						>
							{tab.label}
						</Link>
					))}
				</nav>
				<Outlet />
			</div>
		</div>
	);
}
