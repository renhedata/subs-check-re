// Cron helpers for the scheduler UI: next-run computation (cron-parser),
// human description (cronstrue) and relative formatting.
import { CronExpressionParser } from "cron-parser";
import cronstrue from "cronstrue";

export function nextRun(expr: string, from: Date = new Date()): Date | null {
	try {
		const parsed = CronExpressionParser.parse(expr, {
			currentDate: from,
			tz: "UTC",
		});
		return parsed.next().toDate();
	} catch {
		return null;
	}
}

export function describeCron(expr: string): string {
	try {
		return cronstrue.toString(expr, { verbose: false });
	} catch {
		return expr;
	}
}

export function formatUntil(target: Date, now: Date = new Date()): string {
	const ms = target.getTime() - now.getTime();
	if (ms < 60_000) return "in <1m";
	const totalMinutes = Math.floor(ms / 60_000);
	const days = Math.floor(totalMinutes / (60 * 24));
	const hours = Math.floor((totalMinutes % (60 * 24)) / 60);
	const minutes = totalMinutes % 60;
	if (days > 0) return `in ${days}d ${hours}h`;
	if (hours > 0) return `in ${hours}h ${minutes}m`;
	return `in ${minutes}m`;
}
