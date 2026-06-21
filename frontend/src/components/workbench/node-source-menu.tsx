import { Database } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogClose,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogTitle,
} from "@/components/ui/dialog";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { isApiError } from "@/lib/client";
import {
	useImportNodes,
	useRefreshSubscription,
	useSetFetchProxy,
	useTestFetch,
	useTestNodes,
} from "@/queries";

// NodeSourceMenu controls where a subscription's nodes come from: refresh from
// the URL (optionally tunneled through an existing node), paste them in
// manually, or test whether the URL is reachable. Nodes only change through
// these explicit actions — a check never re-fetches.
export function NodeSourceMenu({
	subscriptionId,
	hasUrl,
	viaNode,
}: {
	subscriptionId: string;
	hasUrl: boolean;
	viaNode: boolean;
}) {
	const [importOpen, setImportOpen] = useState(false);
	const [content, setContent] = useState("");
	const [proxyOpen, setProxyOpen] = useState(false);
	const [selectedNodeId, setSelectedNodeId] = useState("");
	const importMut = useImportNodes(subscriptionId);
	const refreshMut = useRefreshSubscription(subscriptionId);
	const testMut = useTestFetch(subscriptionId);
	const setProxyMut = useSetFetchProxy();
	const testNodesQuery = useTestNodes();

	const handleRefresh = () => {
		refreshMut.mutate(undefined, {
			onSuccess: (r) => toast.success(`Refreshed ${r.count} nodes`),
			onError: (e) => toast.error(isApiError(e) ? e.message : "Refresh failed"),
		});
	};

	const handleTest = () => {
		testMut.mutate(undefined, {
			onSuccess: (r) =>
				r.ok
					? toast.success(`Reachable — ${r.count} nodes`)
					: toast.error(r.error || "Could not fetch subscription"),
			onError: (e) => toast.error(isApiError(e) ? e.message : "Test failed"),
		});
	};

	const handleImport = () => {
		const text = content.trim();
		if (!text) return;
		importMut.mutate(text, {
			onSuccess: (r) => {
				toast.success(`Imported ${r.count} nodes`);
				setContent("");
				setImportOpen(false);
			},
			onError: (e) => toast.error(isApiError(e) ? e.message : "Import failed"),
		});
	};

	const handleSaveProxy = () => {
		const node = (testNodesQuery.data?.nodes ?? []).find(
			(n) => n.id === selectedNodeId,
		);
		const config = node?.config ?? "";
		setProxyMut.mutate(
			{ id: subscriptionId, config },
			{
				onSuccess: () => {
					toast.success(config ? `Fetching via ${node?.name}` : "Direct fetch");
					setProxyOpen(false);
				},
				onError: (e) =>
					toast.error(isApiError(e) ? e.message : "Could not save"),
			},
		);
	};

	return (
		<>
			<DropdownMenu>
				<DropdownMenuTrigger
					render={
						<Button variant="outline" size="icon-sm" aria-label="Node source" />
					}
				>
					<Database size={14} />
				</DropdownMenuTrigger>
				<DropdownMenuContent align="end">
					<DropdownMenuItem
						disabled={!hasUrl || refreshMut.isPending}
						onClick={handleRefresh}
					>
						Refresh from URL
					</DropdownMenuItem>
					<DropdownMenuItem
						disabled={!hasUrl || testMut.isPending}
						onClick={handleTest}
					>
						Test subscription
					</DropdownMenuItem>
					<DropdownMenuItem onClick={() => setProxyOpen(true)}>
						{viaNode ? "Fetch via node ✓" : "Fetch via node…"}
					</DropdownMenuItem>
					<DropdownMenuItem onClick={() => setImportOpen(true)}>
						Import nodes…
					</DropdownMenuItem>
				</DropdownMenuContent>
			</DropdownMenu>

			<Dialog open={importOpen} onOpenChange={setImportOpen}>
				<DialogContent>
					<DialogTitle>Import nodes</DialogTitle>
					<DialogDescription>
						Paste Clash YAML (proxies:) or a V2Ray / base64 subscription. This
						replaces the subscription's node list.
					</DialogDescription>
					<textarea
						value={content}
						onChange={(e) => setContent(e.target.value)}
						rows={10}
						placeholder={
							"proxies:\n  - {name: ..., type: ss, server: ..., port: ...}"
						}
						className="mt-3 w-full resize-y rounded-md border border-border bg-background p-2 font-mono text-xs outline-none focus:border-ring"
					/>
					<DialogFooter>
						<DialogClose render={<Button variant="outline" />}>
							Cancel
						</DialogClose>
						<Button
							onClick={handleImport}
							loading={importMut.isPending}
							disabled={!content.trim()}
						>
							Import
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>

			<Dialog open={proxyOpen} onOpenChange={setProxyOpen}>
				<DialogContent>
					<DialogTitle>Fetch via node</DialogTitle>
					<DialogDescription>
						Route this subscription's download through one of your existing
						nodes — for URLs the server can't reach directly.
					</DialogDescription>
					<select
						value={selectedNodeId}
						onChange={(e) => setSelectedNodeId(e.target.value)}
						className="mt-3 w-full rounded-md border border-border bg-background p-2 text-sm outline-none focus:border-ring"
					>
						<option value="">Direct (no proxy)</option>
						{(testNodesQuery.data?.nodes ?? []).map((n) => (
							<option key={n.id} value={n.id}>
								{n.name}
							</option>
						))}
					</select>
					<DialogFooter>
						<DialogClose render={<Button variant="outline" />}>
							Cancel
						</DialogClose>
						<Button onClick={handleSaveProxy} loading={setProxyMut.isPending}>
							Save
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>
		</>
	);
}
