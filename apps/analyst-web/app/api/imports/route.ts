import { problemResponse } from "@/lib/bff";
import { forwardWorkspaceRequest } from "@/lib/workspace-api";

export async function POST(request: Request) {
  return forwardWorkspaceRequest(request, "/api/imports", {
    code: "preview_unavailable",
    detail: "The import preview is temporarily unavailable.",
  }, validateUpload);
}

function validateUpload(request: Request) {
  const contentType = request.headers.get("content-type") ?? "";
  if (!/^multipart\/form-data\s*;/i.test(contentType) || !request.body) {
    return problemResponse(422, "validation_failed", "Select a CSV file.");
  }
}
