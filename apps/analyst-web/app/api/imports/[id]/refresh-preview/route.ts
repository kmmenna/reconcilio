import { forwardWorkspaceRequest } from "@/lib/workspace-api";

export async function POST(request: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return forwardWorkspaceRequest(request, `/api/imports/${encodeURIComponent(id)}/refresh-preview`, {
    code: "refresh_unavailable",
    detail: "The import preview could not be refreshed.",
  });
}
