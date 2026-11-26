# Build the frontend and its BFF with Next.js

Build the MVP frontend in React and TypeScript with the Next.js App Router. The browser application supports CSV import, processing status, and analyst review. React manages interactions such as comparing competing proposals and updating the review queue after a decision.

Give proposal and reconciliation details their own URLs that work on direct visits and reloads. Use Next.js Route Handlers as the backend for frontend (BFF). The BFF handles the browser's session cookie and calls the Go API with `Authorization: Bearer`. The Go API owns authorization, CSV validation, and reconciliation rules.

## Styling and localization

Use Tailwind CSS for frontend styling. Localize all frontend surfaces in Portuguese and English. Detect the initial language from the browser, let users choose a language, and preserve that choice in the browser for future visits. Keep application URLs independent of locale so each person opening a shared link sees their own language preference.

Use next-intl with the App Router and the supported locales `pt-BR` and `en`. Resolve the locale on the server from a valid language preference cookie, then from `Accept-Language`, and fall back to `en` when neither provides a supported language. Set the document language on the first render from the resolved locale.

Show a native language selector on the sign-in page and a link-styled language menu in the authenticated workspace footer. Share preference handling for Automatic, Brazilian Portuguese, and English, and mark the selected preference inside the menu. Automatic clears the explicit preference so the browser language is used again. Keep the current page when the selection changes.

Localize interface text in Next.js and map stable API error codes to localized messages. Format dates and monetary values for the selected locale using the Transaction's currency. Display imported descriptions as supplied by the Source.

Adopt shadcn/ui components as needed, starting with Button, Input, Label, and Native Select for sign-in and language selection. Style them with Reconcilio's shared tokens and use Impeccable to review the interface and interaction states.

## Repository layout

Place the Go application in `apps/reconciliation` and the Next.js application in `apps/analyst-web`. Name executable projects under `apps/` by their responsibility. Keep Docker Compose and mise configuration at the repository root to coordinate both builds.

## Local execution

Build Next.js and Go independently and run them as separate Docker Compose services. The reviewer opens the Next.js server, which serves the UI and reaches the Go API over the Compose network. This uses the same Next.js server for page routing and browser session handling.

During development, run the Next.js development server with the Go API address configured on the server. The browser uses same-origin `/api` routes in both environments.
