import type { PlatformKey } from "@/components/platform-icons";
import { PlatformIcon } from "@/components/platform-icons";
import { Checkbox } from "@/components/ui/checkbox";
import type { CheckFormOptions } from "@/lib/checkOptions";
import { MEDIA_APPS } from "@/lib/checkOptions";
import { cn } from "@/lib/utils";

// Controlled fieldset for check options. Immutable updates only — parent owns
// the state (Run Check popover persists to localStorage, Schedule dialog
// submits to the scheduler).
export function CheckOptionsFields({
	value,
	onChange,
	showDebug = false,
}: {
	value: CheckFormOptions;
	onChange: (next: CheckFormOptions) => void;
	showDebug?: boolean;
}) {
	const toggleApp = (app: string) =>
		onChange({
			...value,
			media_apps: value.media_apps.includes(app)
				? value.media_apps.filter((a) => a !== app)
				: [...value.media_apps, app],
		});

	return (
		<div className="space-y-3">
			<div>
				<p className="mb-1.5 font-medium text-[11px] text-muted-foreground uppercase tracking-[0.4px]">
					Check options
				</p>
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
								onCheckedChange={(v) =>
									onChange({ ...value, debug: v === true })
								}
							/>
							Debug mode
						</label>
					) : null}
				</div>
			</div>

			<div>
				<p className="mb-1.5 font-medium text-[11px] text-muted-foreground uppercase tracking-[0.4px]">
					Media platforms
				</p>
				<div className="flex flex-wrap gap-1.5">
					{MEDIA_APPS.map((app) => {
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
								<PlatformIcon
									platform={app as PlatformKey}
									size={12}
									showLabel
								/>
							</button>
						);
					})}
				</div>
			</div>
		</div>
	);
}
