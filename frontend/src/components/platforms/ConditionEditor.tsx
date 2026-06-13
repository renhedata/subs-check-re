export function ConditionEditor({
	def,
	onChange,
}: {
	def: Record<string, unknown>;
	onChange: (d: Record<string, unknown>) => void;
}) {
	const set = (k: string, v: unknown) => onChange({ ...def, [k]: v });
	const listVal = (v: unknown) =>
		Array.isArray(v) ? (v as string[]).join(", ") : "";
	const parseList = (s: string) =>
		s
			.split(",")
			.map((x) => x.trim())
			.filter(Boolean);
	const inp =
		"h-7 w-full rounded border border-border bg-background px-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring";

	return (
		<div className="max-w-lg space-y-3">
			<FL label="URL (required)">
				<input
					value={(def?.url as string) ?? ""}
					onChange={(e) => set("url", e.target.value)}
					placeholder="https://example.com/api"
					className={inp}
				/>
			</FL>
			<div className="grid grid-cols-2 gap-3">
				<FL label="Method">
					<select
						value={(def?.method as string) ?? "GET"}
						onChange={(e) => set("method", e.target.value)}
						className={inp}
					>
						{["GET", "HEAD", "POST"].map((m) => (
							<option key={m}>{m}</option>
						))}
					</select>
				</FL>
				<FL label="Expected status (0 = any)">
					<input
						type="number"
						value={(def?.status_code as number) ?? 0}
						onChange={(e) => set("status_code", Number(e.target.value))}
						className={inp}
					/>
				</FL>
			</div>
			<FL label="Body contains ALL (comma-separated)">
				<input
					value={listVal(def?.body_contains)}
					onChange={(e) => set("body_contains", parseList(e.target.value))}
					placeholder="keyword1, keyword2"
					className={inp}
				/>
			</FL>
			<FL label="Body contains ANY">
				<input
					value={listVal(def?.body_contains_any)}
					onChange={(e) => set("body_contains_any", parseList(e.target.value))}
					placeholder="alt1, alt2"
					className={inp}
				/>
			</FL>
			<FL label="Body must NOT contain">
				<input
					value={listVal(def?.body_not_contains)}
					onChange={(e) => set("body_not_contains", parseList(e.target.value))}
					placeholder="blocked, unavailable"
					className={inp}
				/>
			</FL>
			<FL label="Final URL contains">
				<input
					value={(def?.final_url_contains as string) ?? ""}
					onChange={(e) => set("final_url_contains", e.target.value)}
					className={inp}
				/>
			</FL>
			<FL label="Final URL must NOT contain">
				<input
					value={(def?.final_url_not_contains as string) ?? ""}
					onChange={(e) => set("final_url_not_contains", e.target.value)}
					className={inp}
				/>
			</FL>
		</div>
	);
}

function FL({ label, children }: { label: string; children: React.ReactNode }) {
	return (
		<div className="space-y-1">
			<label className="text-muted-foreground text-xs">{label}</label>
			{children}
		</div>
	);
}
