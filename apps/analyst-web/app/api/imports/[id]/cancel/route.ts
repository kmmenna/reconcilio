import { forwardWorkspaceRequest } from "@/lib/workspace-api";

export async function POST(request: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return forwardWorkspaceRequest(request, `/api/imports/${encodeURIComponent(id)}/cancel`, {
    code: "cancellation_unavailable",
    detail: "Import cancellation is temporarily unavailable.",
  });
}
