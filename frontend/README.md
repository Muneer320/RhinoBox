# RhinoBox frontend

Vite and plain JavaScript provide the local file manager. Monaco supplies the JSON/code editor. Start the API from `../backend`, then run `npm ci` and `npm run dev` here. Node.js 24 is used in CI and Docker.

`VITE_API_BASE_URL` defaults to `/api`. Vite proxies that prefix to `http://127.0.0.1:8090`; the Compose frontend uses nginx to forward it to the backend service. Override the setting at build time only when using a different API origin.

Run `npm run lint`, `npm test` and `npm run build`. Vitest/jsdom executes the unit suites and fails on assertion errors; it replaces the original exception-swallowing runner. The type-selector layout tests load the real application stylesheet. Real uploads, downloads, notes, persistence and responsive layout also need a browser check; jsdom tests alone do not establish browser behavior.

Home accepts files or JSON; Files shows indexed collections and file controls. Data lists retained JSON archives grouped by routing destination/namespace. It does not execute arbitrary SQL or display live database row totals. The API reports the actual backend and partial-write warnings. Authentication controls remain disabled because the backend is a local single-user demo.

See the [repository README](../README.md) and [demo walkthrough](../docs/DEMO.md) for the complete stack.
