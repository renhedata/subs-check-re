import { Checkbox } from "@frontend/ui/components/checkbox";
import { Input } from "@frontend/ui/components/input";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@frontend/ui/components/select";
import cronstrue from "cronstrue";
import { useEffect, useState } from "react";

const DEFAULT_CRON = "0 * * * *";

type Period = "hour" | "day" | "week" | "month" | "custom";

interface ParsedCron {
	period: Period;
	minute: number;
	hour: number;
	weekday: number;
	monthday: number;
}

function parseCron(cron: string): ParsedCron {
	const defaults = { minute: 0, hour: 9, weekday: 1, monthday: 1 };

	const hourMatch = cron.match(/^(\d+) \* \* \* \*$/);
	if (hourMatch) {
		return { period: "hour", ...defaults, minute: Number(hourMatch[1]) };
	}

	const dayMatch = cron.match(/^(\d+) (\d+) \* \* \*$/);
	if (dayMatch) {
		return {
			period: "day",
			...defaults,
			minute: Number(dayMatch[1]),
			hour: Number(dayMatch[2]),
		};
	}

	const weekMatch = cron.match(/^(\d+) (\d+) \* \* (\d+)$/);
	if (weekMatch) {
		return {
			period: "week",
			...defaults,
			minute: Number(weekMatch[1]),
			hour: Number(weekMatch[2]),
			weekday: Number(weekMatch[3]),
		};
	}

	const monthMatch = cron.match(/^(\d+) (\d+) (\d+) \* \*$/);
	if (monthMatch) {
		return {
			period: "month",
			...defaults,
			minute: Number(monthMatch[1]),
			hour: Number(monthMatch[2]),
			monthday: Number(monthMatch[3]),
		};
	}

	return { period: "custom", ...defaults };
}

function buildCron(parsed: ParsedCron): string {
	const { period, minute, hour, weekday, monthday } = parsed;
	if (period === "hour") return `${minute} * * * *`;
	if (period === "day") return `${minute} ${hour} * * *`;
	if (period === "week") return `${minute} ${hour} * * ${weekday}`;
	if (period === "month") return `${minute} ${hour} ${monthday} * *`;
	return "";
}

const MINUTE_OPTIONS = [0, 5, 10, 15, 20, 30, 45];
const WEEKDAY_OPTIONS = [
	{ value: 1, label: "Monday" },
	{ value: 2, label: "Tuesday" },
	{ value: 3, label: "Wednesday" },
	{ value: 4, label: "Thursday" },
	{ value: 5, label: "Friday" },
	{ value: 6, label: "Saturday" },
	{ value: 0, label: "Sunday" },
];
const PERIOD_OPTIONS: { value: Period; label: string }[] = [
	{ value: "hour", label: "hour" },
	{ value: "day", label: "day" },
	{ value: "week", label: "week" },
	{ value: "month", label: "month" },
	{ value: "custom", label: "custom…" },
];

function pad(n: number): string {
	return String(n).padStart(2, "0");
}

interface CronPickerProps {
	value: string;
	onChange: (v: string) => void;
	allowDisable?: boolean;
}

export function CronPicker({ value, onChange, allowDisable }: CronPickerProps) {
	const enabled = value !== "";
	const parsed = parseCron(value || DEFAULT_CRON);
	const [activePeriod, setActivePeriod] = useState<Period>(parsed.period);

	// Sync activePeriod when the parent changes value externally,
	// but don't override when the user has selected "custom".
	useEffect(() => {
		const p = parseCron(value || DEFAULT_CRON).period;
		if (p !== "custom") {
			setActivePeriod(p);
		}
	}, [value]);

	function handleEnable(checked: boolean) {
		onChange(checked ? DEFAULT_CRON : "");
	}

	function updateParsed(update: Partial<ParsedCron>) {
		const next = { ...parsed, ...update };
		if (next.period !== "custom") {
			onChange(buildCron(next));
		}
	}

	function handlePeriodChange(period: Period) {
		setActivePeriod(period);
		if (period === "custom") {
			// User will type in the input; don't call onChange yet.
			return;
		}
		const next = { ...parsed, period };
		onChange(buildCron(next));
	}

	let description = "";
	try {
		if (value) {
			description = cronstrue.toString(value, { verbose: false });
		}
	} catch {
		// invalid cron — skip
	}

	const builder =
		enabled || !allowDisable ? (
			<div className="space-y-1">
				<div className="flex flex-wrap items-center gap-1.5 text-sm">
					<span className="text-muted-foreground">Every</span>

					<Select
						value={activePeriod}
						onValueChange={(v) => handlePeriodChange(v as Period)}
					>
						<SelectTrigger size="sm">
							<SelectValue />
						</SelectTrigger>
						<SelectContent>
							{PERIOD_OPTIONS.map((opt) => (
								<SelectItem key={opt.value} value={opt.value}>
									{opt.label}
								</SelectItem>
							))}
						</SelectContent>
					</Select>

					{activePeriod === "hour" && (
						<>
							<span className="text-muted-foreground">at minute</span>
							<Select
								value={String(parsed.minute)}
								onValueChange={(v) => updateParsed({ minute: Number(v) })}
							>
								<SelectTrigger size="sm">
									<SelectValue />
								</SelectTrigger>
								<SelectContent>
									{MINUTE_OPTIONS.map((m) => (
										<SelectItem key={m} value={String(m)}>
											{pad(m)}
										</SelectItem>
									))}
								</SelectContent>
							</Select>
						</>
					)}

					{activePeriod === "day" && (
						<>
							<span className="text-muted-foreground">at</span>
							<HourSelect
								value={parsed.hour}
								onChange={(h) => updateParsed({ hour: h })}
							/>
							<span className="text-muted-foreground">:</span>
							<MinuteSelect
								value={parsed.minute}
								onChange={(m) => updateParsed({ minute: m })}
							/>
						</>
					)}

					{activePeriod === "week" && (
						<>
							<Select
								value={String(parsed.weekday)}
								onValueChange={(v) => updateParsed({ weekday: Number(v) })}
							>
								<SelectTrigger size="sm">
									<SelectValue />
								</SelectTrigger>
								<SelectContent>
									{WEEKDAY_OPTIONS.map((d) => (
										<SelectItem key={d.value} value={String(d.value)}>
											{d.label}
										</SelectItem>
									))}
								</SelectContent>
							</Select>
							<span className="text-muted-foreground">at</span>
							<HourSelect
								value={parsed.hour}
								onChange={(h) => updateParsed({ hour: h })}
							/>
							<span className="text-muted-foreground">:</span>
							<MinuteSelect
								value={parsed.minute}
								onChange={(m) => updateParsed({ minute: m })}
							/>
						</>
					)}

					{activePeriod === "month" && (
						<>
							<span className="text-muted-foreground">on day</span>
							<Select
								value={String(parsed.monthday)}
								onValueChange={(v) => updateParsed({ monthday: Number(v) })}
							>
								<SelectTrigger size="sm">
									<SelectValue />
								</SelectTrigger>
								<SelectContent>
									{Array.from({ length: 31 }, (_, i) => i + 1).map((d) => (
										<SelectItem key={d} value={String(d)}>
											{d}
										</SelectItem>
									))}
								</SelectContent>
							</Select>
							<span className="text-muted-foreground">at</span>
							<HourSelect
								value={parsed.hour}
								onChange={(h) => updateParsed({ hour: h })}
							/>
							<span className="text-muted-foreground">:</span>
							<MinuteSelect
								value={parsed.minute}
								onChange={(m) => updateParsed({ minute: m })}
							/>
						</>
					)}

					{activePeriod === "custom" && (
						<Input
							className="h-7 w-40 font-mono text-xs"
							placeholder="* * * * *"
							value={value || DEFAULT_CRON}
							onChange={(e) => onChange(e.target.value)}
						/>
					)}
				</div>

				{description && (
					<p className="text-[11px] text-muted-foreground/70">{description}</p>
				)}
			</div>
		) : null;

	if (!allowDisable) {
		return builder;
	}

	return (
		<div className="space-y-2">
			<label className="flex cursor-pointer select-none items-center gap-2">
				<Checkbox
					checked={enabled}
					onCheckedChange={(v) => handleEnable(v === true)}
				/>
				<span className="text-muted-foreground text-sm">Enable</span>
			</label>
			{enabled && builder}
		</div>
	);
}

function HourSelect({
	value,
	onChange,
}: {
	value: number;
	onChange: (h: number) => void;
}) {
	return (
		<Select value={String(value)} onValueChange={(v) => onChange(Number(v))}>
			<SelectTrigger size="sm">
				<SelectValue />
			</SelectTrigger>
			<SelectContent>
				{Array.from({ length: 24 }, (_, i) => i).map((h) => (
					<SelectItem key={h} value={String(h)}>
						{pad(h)}:00
					</SelectItem>
				))}
			</SelectContent>
		</Select>
	);
}

function MinuteSelect({
	value,
	onChange,
}: {
	value: number;
	onChange: (m: number) => void;
}) {
	return (
		<Select value={String(value)} onValueChange={(v) => onChange(Number(v))}>
			<SelectTrigger size="sm">
				<SelectValue />
			</SelectTrigger>
			<SelectContent>
				{MINUTE_OPTIONS.map((m) => (
					<SelectItem key={m} value={String(m)}>
						{pad(m)}
					</SelectItem>
				))}
			</SelectContent>
		</Select>
	);
}
