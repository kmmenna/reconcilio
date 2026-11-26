import { forwardWorkspaceRequest } from "@/lib/workspace-api";

export async function POST(request: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return forwardWorkspaceRequest(request, `/api/imports/${encodeURIComponent(id)}/confirm`, {
    code: "confirmation_unavailable",
    detail: "Import confirmation is temporarily unavailable.",
  });
}
