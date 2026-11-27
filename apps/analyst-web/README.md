# Analyst web

The analyst workspace is a Next.js app for importing CSVs, tracking reconciliation jobs, comparing Match Proposals, and inspecting discrepancies and decision history.

## Architecture

- **Page rendering.** The App Router uses Server Components for workspace layouts and pages that read from the Go API. Client Components handle interactions such as file selection, form submissions, proposal decisions, and job polling. Each import, job, proposal, and reconciliation has a detail URL that can be opened directly or reloaded. [Workspace layout](app/workspace/layout.tsx) · [Server-side reads](lib/workspace-api.ts)
- **Backend for frontend (BFF).** Browser requests use same-origin Route Handlers under `app/api`. Next.js stores the JWT issued by Go in an `HttpOnly`, `SameSite=Lax` cookie, checks the configured origin for browser mutations, and forwards authenticated requests to Go as bearer requests. The Go API owns authorization, CSV validation, and reconciliation rules. Request and workflow correlation IDs connect browser activity to API logs and background jobs. [Session handling](lib/auth.ts) · [Request forwarding](lib/workspace-api.ts) · [Correlation IDs](proxy.ts)
- **Localization.** The server uses an explicit language preference when present, otherwise it negotiates the browser’s `Accept-Language` header and falls back to English. The interface supports `en` and `pt-BR`, and shared URLs stay the same across locales. Stable API problem codes map to localized messages. The UI formats Money from decimal strings, preserves the calendar day for Transaction dates, and displays recorded timestamps in the browser’s time zone. [Locale selection](i18n/request.ts) · [Problem messages](lib/problem-message.ts) · [Formatting](lib/transaction-format.ts)

## Stack choices

| Tool | Why it was chosen |
| --- | --- |
| **Next.js, React, and TypeScript** | The App Router keeps page routing, server rendering, and the BFF in one deployable app. React provides interactive UI where needed, and TypeScript checks component props and the shapes the frontend expects from the API. |
| **next-intl, Negotiator, and `Intl`** | next-intl shares translation catalogs between server and client. Negotiator parses browser language preferences, and `Intl` handles localized money and date formatting. |
| **Tailwind CSS and shadcn/ui** | Tailwind utilities use shared CSS tokens for color, focus, and layout. Local shadcn/ui primitives give forms a consistent baseline and let the project own their styling and behavior. |
| **Radix UI and Lucide** | Radix provides keyboard and focus behavior for menus and selects. Lucide provides a consistent icon set for navigation and transaction cues alongside text labels. |
| **Node.js, npm, mise, and Compose** | mise pins the Node version and exposes the same commands for local work and CI. `npm ci` installs from the lockfile, and Compose runs the full stack in an isolated environment for browser tests. |

The frontend ADR records the stack and localization decisions. Impeccable was used during development to review and refine the interface. [Frontend ADR](../../docs/adr/0010-build-the-frontend-with-react-and-typescript.md) · [AI usage](../../docs/AI_USAGE.md)

## Key components

| Area | Components and responsibilities |
| --- | --- |
| **Workspace shell** | The layout provides the sidebar, narrow-screen navigation, account menu, and language control. [Layout](app/workspace/layout.tsx) · [Navigation](app/workspace/workspace-navigation.tsx) · [Account menu](app/workspace/account-menu.tsx) |
| **CSV import** | `ImportForm` collects the Source, account assignment, and CSV and shows validation issues. The import details page shows inferred coverage, conflicts, and the receipt. `ConfirmImport` handles confirmation and conflict decisions. [Upload form](app/workspace/imports/import-form.tsx) · [Import details](app/workspace/imports/[id]/page.tsx) · [Confirmation](app/workspace/imports/[id]/confirm-import.tsx) |
| **Processing** | `JobProgress` polls while a job is active and shows its outcome, counts, and any partial results. [Job progress](app/workspace/jobs/[id]/job-progress.tsx) |
| **Review and history** | `TransactionRecord`, `ProposalEvidence`, and `RankingAssessment` show the records and their evidence. `ProposalActions` submits a decision with the displayed ranking evaluation ID when available. [Review components](components/review/) · [Reconciliation detail](app/workspace/reconciliations/[id]/page.tsx) |
| **Shared forms** | `Button`, `Input`, `Label`, and `FormSelect` share styles for focus, invalid, and disabled states. `AccountForm` handles user creation, profile updates, and password changes. [UI primitives](components/ui/) · [Account form](components/accounts/account-form.tsx) · [CSS tokens](app/globals.css) |

## Test strategy

`mise run check:web` generates Next.js types, checks TypeScript, and runs ESLint. Playwright covers workflows in Chromium against the real Next.js app, Go API, worker, and PostgreSQL. The smoke script starts a fresh Compose project, bootstraps an administrator, and cleans up the test resources. GitHub Actions runs both smoke modes. [Playwright configuration](playwright.config.ts) · [Smoke setup](../../scripts/smoke.sh) · [CI workflow](../../.github/workflows/ci.yml)

| Playwright spec | Coverage |
| --- | --- |
| [Sign-in](e2e/sign-in.spec.ts) | Sign-in, reload, sign-out, session cookie, origin checks, and keyboard navigation at desktop and narrow widths. |
| [Accounts](e2e/accounts.spec.ts) | Administrator creates an analyst, who can update their profile and password and sign in with the new credentials. |
| [Language](e2e/language.spec.ts) | Browser language detection, persistence through sign-in and sign-out, return to automatic detection, English fallback, and invalid preference requests. |
| [Import preview](e2e/import-preview.spec.ts) | CSV preview and reload, inferred coverage, and localized date, Money, and validation messages. |
| [Mixed import](e2e/mixed-import.spec.ts) | Account and Source setup, a CSV spanning multiple accounts, and refreshing a preview after a Source side change. |
| [Confirmation and review](e2e/confirm-import.spec.ts) | Automatic matching, conflicting reimports, competing proposals, open discrepancies, and preserved decision history. |
| [Reprocessing](e2e/reprocess-account.spec.ts) | Reprocessing a selected Financial Account and reopening the same job summary in both languages. |
| [Bilingual ranking](e2e/bilingual-ranking.spec.ts) | A controlled ranker provides repeatable scores and explanations. Switching languages or following shared links preserves the recorded assessment without another provider request. |

Run the default smoke suite with `mise run smoke`. Use `RECONCILIO_SMOKE_RANKING=true mise run smoke` to run the controlled ranking spec. See the setup instructions for installing browser dependencies, running the app on the host, or running the full stack. [Host development](../../README.md#run-the-nextjs-frontend-on-the-host) · [Test environment](../../docs/environment.md#tests)
