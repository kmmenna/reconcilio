import { forwardWorkspaceRequest } from "@/lib/workspace-api";

export async function GET(request: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return forwardWorkspaceRequest(request, `/api/jobs/${encodeURIComponent(id)}`, {
    code: "job_unavailable",
    detail: "The job status is temporarily unavailable.",
  });
}
