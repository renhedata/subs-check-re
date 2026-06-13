import { Download } from "lucide-react";
import { CopyButton } from "@/components/copy-button";
import { Button } from "@/components/ui/button";
import {
	Popover,
	PopoverContent,
	PopoverTrigger,
} from "@/components/ui/popover";
import { Spinner } from "@/components/ui/spinner";
import { useAPIKey } from "@/queries";

const FORMATS = ["clash", "base64", "routeros"] as const;

export function ExportPopover({ subscriptionId }: { subscriptionId: string }) {
	const apiKeyQuery = useAPIKey();
	const apiKey = apiKeyQuery.data?.api_key ?? "";
	const base = `${window.location.origin}/api/export/${subscriptionId}`;

	return (
		<Popover>
			<PopoverTrigger render={<Button variant="outline" size="sm" />}>
				<Download size={13} /> Export
			</PopoverTrigger>
			<PopoverContent className="w-96">
				<p className="mb-2 font-medium text-foreground text-xs">
					Subscription URLs
				</p>
				{apiKeyQuery.isLoading ? (
					<Spinner />
				) : (
					<div className="space-y-1.5">
						{FORMATS.map((t) => {
							const url = `${base}?token=${apiKey}&target=${t}`;
							return (
								<div key={t} className="flex items-center gap-2">
									<span className="w-16 shrink-0 text-[11px] text-muted-foreground">
										{t}
									</span>
									<code className="min-w-0 flex-1 truncate rounded bg-secondary px-2 py-1 font-mono text-[11px] text-foreground">
										{url}
									</code>
									<CopyButton text={url} />
								</div>
							);
						})}
						<p className="pt-1 text-[11px] text-muted-foreground">
							All-subscriptions URLs live in Settings → Export API.
						</p>
					</div>
				)}
			</PopoverContent>
		</Popover>
	);
}
