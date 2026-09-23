// Ports for the e2e run, shared by playwright.config.ts and the specs. Kept
// off the defaults (8080 for the app) so a dev server can stay up alongside.
export const APP_PORT = Number(process.env.E2E_APP_PORT ?? 8199)
export const STUB_PORT = Number(process.env.E2E_STUB_PORT ?? 8198)
export const STUB_URL = `http://127.0.0.1:${STUB_PORT}`
