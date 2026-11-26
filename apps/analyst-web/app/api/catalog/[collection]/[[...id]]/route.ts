import { problemResponse } from "@/lib/bff";
import { forwardWorkspaceRequest } from "@/lib/workspace-api";

type Route = { params: Promise<{ collection: string; id?: string[] }> };

async function writeCatalog(request: Request, { params }: Route) {
  const { collection, id = [] } = await params;
  const validCollection = collection === "financial-accounts" || collection === "sources";
  if (!validCollection || id.length !== (request.method === "PATCH" ? 1 : 0)) {
    return problemResponse(404, "catalog_entry_not_found", "The catalog entry could not be found.");
  }
  const suffix = id.length ? `/${encodeURIComponent(id[0])}` : "";
  return forwardWorkspaceRequest(request, `/api/${collection}${suffix}`, {
    code: "catalog_unavailable",
    detail: "The catalog is temporarily unavailable.",
  });
}

export { writeCatalog as PATCH, writeCatalog as POST };
