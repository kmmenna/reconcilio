import { forwardWorkspaceRequest } from "@/lib/workspace-api";

export async function POST(request: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return forwardWorkspaceRequest(request, `/api/financial-accounts/${encodeURIComponent(id)}/reprocess`, {
    code: "reprocessing_unavailable",
    detail: "Reprocessing is temporarily unavailable.",
  });
}
