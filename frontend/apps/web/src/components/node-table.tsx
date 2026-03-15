// frontend/apps/web/src/components/node-table.tsx
import type { NodeResult } from "@/lib/api";

interface Props {
  results: NodeResult[];
}

export function NodeTable({ results }: Props) {
  const alive = results.filter((r) => r.alive);
  const dead = results.filter((r) => !r.alive);
  const sorted = [...alive, ...dead];

  if (sorted.length === 0) {
    return <p className="text-muted-foreground text-sm">No results yet.</p>;
  }

  return (
    <div className="overflow-x-auto rounded border">
      <table className="w-full text-sm">
        <thead className="bg-muted text-left">
          <tr>
            <th className="px-3 py-2">Node</th>
            <th className="px-3 py-2">Status</th>
            <th className="px-3 py-2">Latency</th>
            <th className="px-3 py-2">Speed</th>
            <th className="px-3 py-2">Country</th>
            <th className="px-3 py-2">Platforms</th>
          </tr>
        </thead>
        <tbody>
          {sorted.map((r) => (
            <tr key={r.node_id} className="border-t hover:bg-muted/40">
              <td className="px-3 py-2 max-w-[200px] truncate font-mono text-xs">{r.node_name}</td>
              <td className="px-3 py-2">
                <span className={r.alive ? "text-green-600" : "text-red-500"}>
                  {r.alive ? "✓ alive" : "✗ dead"}
                </span>
              </td>
              <td className="px-3 py-2">{r.alive ? `${r.latency_ms}ms` : "—"}</td>
              <td className="px-3 py-2">
                {r.alive && r.speed_kbps
                  ? r.speed_kbps >= 1024
                    ? `${(r.speed_kbps / 1024).toFixed(1)} MB/s`
                    : `${r.speed_kbps} KB/s`
                  : "—"}
              </td>
              <td className="px-3 py-2">{r.country || "—"}</td>
              <td className="px-3 py-2 flex gap-1 flex-wrap">
                {r.netflix && (
                  <span className="rounded bg-red-100 px-1 text-red-700 text-xs">NF</span>
                )}
                {r.youtube && (
                  <span className="rounded bg-red-100 px-1 text-red-700 text-xs">YT</span>
                )}
                {r.openai && (
                  <span className="rounded bg-green-100 px-1 text-green-700 text-xs">GPT</span>
                )}
                {r.claude && (
                  <span className="rounded bg-orange-100 px-1 text-orange-700 text-xs">CL</span>
                )}
                {r.gemini && (
                  <span className="rounded bg-blue-100 px-1 text-blue-700 text-xs">GM</span>
                )}
                {r.disney && (
                  <span className="rounded bg-blue-100 px-1 text-blue-700 text-xs">D+</span>
                )}
                {r.tiktok && (
                  <span className="rounded bg-gray-100 px-1 text-xs">TK</span>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
