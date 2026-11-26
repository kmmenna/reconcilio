export const requestIDHeader = "x-request-id";
export const correlationIDHeader = "x-correlation-id";

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export function validID(value: string | null | undefined) {
  return value && uuid.test(value) && value !== "00000000-0000-0000-0000-000000000000"
    ? value.toLowerCase()
    : undefined;
}

export function workflowURL(path: string, workflow: string) {
  return `${path}?workflow=${encodeURIComponent(workflow)}`;
}

export function correlatedFetch(path: string, init: RequestInit, workflow?: string) {
  const headers = new Headers(init.headers);
  headers.set(requestIDHeader, crypto.randomUUID());
  headers.set(correlationIDHeader, workflow ?? crypto.randomUUID());
  return fetch(path, { ...init, headers });
}
