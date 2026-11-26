import { problemResponse } from "@/lib/bff";
import { forwardWorkspaceRequest } from "@/lib/workspace-api";

export async function POST(request: Request, { params }: { params: Promise<{ id: string; action: string }> }) {
  const { id, action } = await params;
  if (action !== "confirm" && action !== "reject") {
    return problemResponse(404, "match_proposal_not_found", "The Match Proposal could not be found.");
  }
  return forwardWorkspaceRequest(request, `/api/match-proposals/${encodeURIComponent(id)}/${action}`, {
    code: "review_unavailable",
    detail: "The decision could not be saved. Try again.",
  });
}
