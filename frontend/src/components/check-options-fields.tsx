import { RulePlatformIcon } from "@/components/rule-icon";
import { Checkbox } from "@/components/ui/checkbox";
import type { CheckFormOptions } from "@/lib/checkOptions";
import { cn } from "@/lib/utils";

// Controlled fieldset for check options. Immutable updates only — parent owns
// the state. `availablePlatforms` is the set of selectable platform keys,
// derived from the user's enabled rules by the parent. Connectivity + latency
// are always measured and are not represented here.
export function CheckOptionsFields({
	value,
	onChange,
	availablePlatforms,
	showDebug = false,
}: {
	value: CheckFormOptions;
	onChange: (next: CheckFormOptions) => void;
	availablePlatforms: string[];
	showDebug?: boolean;
}) {
	const toggleApp = (app: string) =>
		onChange({
			...value,
			media_apps: value.media_apps.includes(app)
				? value.media_apps.filter((a) => a !== app)
				: [...value.media_apps, app],
		});

	const aliveOnly = () =>
		onChange({
			...value,
			speed_test: false,
			upload_speed_test: false,
			media_apps: [],
		});

	const selectAll = () =>
		onChange({ ...value, media_apps: [...availablePlatforms] });

	return (
		<div className="space-y-3">
			<p className="rounded-md bg-secondary px-2 py-1.5 text-[11px] text-muted-foreground">
				Connectivity + latency: always tested.
			</p>

			<div className="flex items-center justify-between">
				<p className="font-medium text-[11px] text-muted-foreground uppercase tracking-[0.4px]">
					Check options
				</p>
				<button
					type="button"
					onClick={aliveOnly}
					className="rounded-full border border-border px-2 py-0.5 text-[11px] text-muted-foreground hover:bg-secondary"
				>
					Alive only
				</button>
			</div>

			<div className="space-y-1.5">
				<label className="flex cursor-pointer items-center gap-2 text-sm">
					<Checkbox
						checked={value.speed_test}
						onCheckedChange={(v) =>
							onChange({ ...value, speed_test: v === true })
						}
					/>
					Speed test{" "}
					<span className="text-muted-foreground text-xs">(download)</span>
				</label>
				<label className="flex cursor-pointer items-center gap-2 text-sm">
					<Checkbox
						checked={value.upload_speed_test}
						onCheckedChange={(v) =>
							onChange({ ...value, upload_speed_test: v === true })
						}
					/>
					Upload test
				</label>
				{showDebug ? (
					<label className="flex cursor-pointer items-center gap-2 text-sm">
						<Checkbox
							checked={value.debug}
							onCheckedChange={(v) => onChange({ ...value, debug: v === true })}
						/>
						Debug mode
					</label>
				) : null}
			</div>

			<div>
				<div className="mb-1.5 flex items-center justify-between">
					<p className="font-medium text-[11px] text-muted-foreground uppercase tracking-[0.4px]">
						Media platforms
					</p>
					<button
						type="button"
						onClick={selectAll}
						className="text-[11px] text-muted-foreground hover:text-foreground"
					>
						All
					</button>
				</div>
				<div className="flex flex-wrap gap-1.5">
					{availablePlatforms.map((app) => {
						const active = value.media_apps.includes(app);
						return (
							<button
								key={app}
								type="button"
								aria-pressed={active}
								onClick={() => toggleApp(app)}
								className={cn(
									"inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs transition-colors",
									active
										? "border-info-line bg-info-muted text-info"
										: "border-border text-muted-foreground hover:bg-secondary",
								)}
							>
								<RulePlatformIcon platformKey={app} size={12} showLabel />
							</button>
						);
					})}
				</div>
			</div>
		</div>
	);
}
