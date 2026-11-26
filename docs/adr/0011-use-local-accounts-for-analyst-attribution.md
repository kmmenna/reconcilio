# Use local accounts for analyst attribution

Use local accounts with stable UUIDs, unique usernames, editable display names, and Argon2id password hashes. Administrators can create users, and users can change their own display names and passwords. Attribute manual decisions to the user's UUID and automatic decisions to the reconciliation engine.

## API authentication

The identity module issues signed JWT access tokens after a successful login. Clients send the token in the `Authorization: Bearer` header on protected requests. The API validates the token and uses the local user UUID as the subject. This header can also carry credentials issued to service identities in the future.

After a successful login, the Next.js BFF sets an `HttpOnly` session cookie whose value is the signed JWT returned by Go. On authenticated browser requests, the BFF reads the JWT from the cookie and sends it to the Go API as `Authorization: Bearer`. The cookie carries the session state for the MVP.

Access tokens expire 30 minutes after issuance. The user signs in again when the token expires. Logout clears the browser cookie. A previously copied token can still be used until it expires.

Set the session cookie to `SameSite=Lax` so the browser can send it on direct link navigations. For state-changing browser requests, including login and logout, the BFF requires an `Origin` header that exactly matches the configured Next.js origin. Reject missing or mismatched origins before forwarding the request.

## First-time setup

After preparing a running, migrated database, run `mise run bootstrap-admin` to create the initial administrator. If the worktree's Compose backend is running, the task connects from that container with its database settings. Otherwise, the task runs the Go command on the host against the worktree's Compose PostgreSQL. Write the generated password once to the invoking terminal and store its hash. If the administrator already exists, report that it is configured and preserve its current credential.

## Future identity providers

An OpenID Connect provider such as Keycloak can identify users by issuer and subject and link those identities to local UUIDs. Existing decision history keeps its original attribution.
