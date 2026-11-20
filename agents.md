# Haraesume Architecture and Agent Harness

## Scope

This guide describes the source as of `89a8baa`, the `main` revision used as the documentation baseline.

## 1. Runtime boundaries

| Component | Responsibility | Source |
| --- | --- | --- |
| React and Vite frontend | Firebase Google sign-in, waitlist UI, resume upload and tailoring, resume builder, and PDF download | [App](frontend/src/App.tsx), [Firebase setup](frontend/src/firebase.ts) |
| Go HTTP service | Authenticated resume workflows, Firestore persistence, OpenRouter calls, LaTeX compilation, and waitlist APIs | [Routes and startup](backend/main.go) |
| Python LangGraph | Standalone CLI for resume and job matching | [LangChain and LangGraph specification](backend/agents/README.md) |

The browser calls the Go API through the Vite proxy in development and Nginx in the frontend container. The Go route handlers have no dependency on the Python graph. The backend image includes the graph and its Python dependencies alongside the Go binary and TeX Live. [Vite configuration](frontend/vite.config.ts), [Nginx configuration](frontend/nginx.conf.template), [Backend image](backend/Dockerfile)

## 2. Go web workflows

### Resume tailoring and export

1. The user uploads a UTF-8 `.tex` resume or reuses their saved base resume, then submits a job description. Both inputs go through length checks and sanitization. [Resume handlers](backend/resume/handler.go)
2. The web app runs a deterministic Go fit check on the base resume and job description before tailoring. The report covers skill coverage, missing must-have skills, strengths, ATS warnings, weak bullets, and recommendations. [Fit analyzer](backend/analysis/analyzer.go), [App](frontend/src/App.tsx)
3. `llm.OptimizeResume` ranks candidate technical skills using a curated dictionary and job-description signals. An optional user-supplied project pool lets the model select among original and additional projects. The optimizer wraps the inputs as untrusted data and asks OpenRouter for complete LaTeX followed by a `---CHANGES---` summary. The prompt asks for at most five new technical skills and a one-page result. [Optimizer](backend/llm/optimizer.go), [Input handling](backend/llm/sanitize.go)
4. The response parser requires LaTeX document boundaries and extracts the summary when available. The optimizer restores matching Experience and Leadership sections from the base resume, restores Projects when the additional pool is empty, and reorders technical skills for job relevance. Restoration depends on recognizable section names in both documents. The five-skill limit and factual accuracy depend on model compliance. [Optimizer](backend/llm/optimizer.go), [LaTeX parser](backend/latex/parser.go)
5. Export compiles the active resume, preferring the optimized version when present. Company resolution prompts for a JSON object, parses the result permissively, and uses regex heuristics on agent errors. It can return `Unknown Company`. The download API returns base64 PDF data and metadata. [Resume handlers](backend/resume/handler.go), [Company resolution](backend/llm/company.go)
6. The application-package endpoint runs optimization, resolves the company, then writes a resume PDF under a user- and company-specific directory. It cleans up its intermediate `.tex` and reports PDF generation or save errors in `pdf_warnings`. Cover-letter generation was removed in `51d7733`. [Resume handlers](backend/resume/handler.go), [Routes](backend/main.go)

### Resume builder and compilation

The builder validates structured form data, escapes user-entered LaTeX text, renders a server-side template, and returns a PDF. This path uses no LLM. [Builder handler](backend/builder/handler.go), [Builder schema](backend/builder/types.go), [Escaping](backend/builder/escape.go)

Both web flows use `CompileToSinglePagePDF`. The compiler checks declared LaTeX packages against a supported list, then tries available compilers in the order `latexmk`, `tectonic`, `xelatex`, and `pdflatex`. When the page-count probe reports multiple pages, it retries with tighter formatting. It can return a multipage PDF if those retries fail to reach one page. [Compiler](backend/latex/compiler.go), [Supported packages](backend/latex/package_support.go)

### Identity, access, and persistence

- With `AUTH_PROVIDER=firebase`, the API validates Firebase ID tokens and uses the token subject as the user ID. When the provider is unset, the middleware uses a shared anonymous identity. The React app gates its workflows on waitlist approval. Resume and builder routes enforce authentication through the middleware, while the admin handlers check `ADMIN_UIDS`. [Auth middleware](backend/auth/firebase.go), [Routes](backend/main.go), [Waitlist handlers](backend/waitlist/handler.go), [App](frontend/src/App.tsx)
- The frontend uses Firestore for email waitlist records. The Go status and admin endpoints use a separate JSON-backed store. The public signup notification endpoint uses Resend for confirmation email when configured. [App](frontend/src/App.tsx), [Waitlist store](backend/waitlist/store.go), [Waitlist handlers](backend/waitlist/handler.go)
- The backend persists base LaTeX in `resumes/<uid>` with an `expire_at` timestamp seven days after upload. The storage layer checks expiry when loading from Firestore. An already-cached base resume is served from memory without rechecking that timestamp. Physical deletion depends on Firestore TTL configuration. [Resume storage](backend/resume/storage.go)
- The base resume cache, current job description, optimized LaTeX, and additional project pool are process-local. Changing the job description clears the optimized version. Uploading a base resume also resets the additional project pool. PDF artifacts written to disk follow the configured application output path and deployment volume. [Resume state](backend/resume/state.go), [Output path configuration](backend/config/config.go)
- The server initializes the resume handlers in the background. `GET /health` returns HTTP 200 with `starting` or `healthy`, and Firestore-dependent routes return 503 before the readiness flag is set. The startup code discards the Firestore probe result, so the health status represents application initialization. [Startup and routes](backend/main.go)

## 3. Agent harness for development

The workspace's [senior engineering manager skill](.codex/skills/senior-eng-manager/SKILL.md) guides architecture, feature planning, and code review. The [interactive coding coach skill](.codex/skills/interactive-coding-coach/SKILL.md) guides step-by-step work when the user wants to learn or implement a change together.

### Working in the repo

- Start every new change on a fresh branch from the intended PR base. Use the current checkout for a small, isolated change. Create a separate worktree on a new branch for larger or parallel work, or when another task has changes in the current checkout.
- Make focused commits that each describe a meaningful change. Run the relevant checks, push the branch, and open a pull request that summarizes the changes and verification. Keep follow-up changes for the same task on its open PR branch.
- Inspect the checked-out revision and working tree before planning. Trace the affected user flow through its normal entry point and check the API, state, and output contracts it relies on.
- Before adding or refactoring code, search for existing helpers, types, and standard-library functions that already cover the behavior. Reuse them when they preserve caller contracts, including validation, defaults, error responses, and side effects. Update all callers before removing a duplicate and verify the affected paths through their normal entry points.
- Keep web feature changes in the React and Go path. Follow the [Python agent specification](backend/agents/README.md) for changes to the standalone CLI. Document changes to shared concepts such as fit scoring wherever their behavior changes.
- For work on prompts or scoring, use synthetic resumes and job descriptions as fixtures. Check deterministic behavior with model credentials removed or with model responses stubbed. Report any behavior that depends on a live provider.
- Update this guide when a change affects runtime boundaries or architecture decisions. Update the Python specification when graph routing, tools, state, or the CLI contract changes.

### Verification by area

| Area | Checks |
| --- | --- |
| Go API or domain logic | Run `go test ./...` from `backend/`. Exercise the affected handler when changing request or response behavior. |
| Frontend | Run `npm run lint` and `npm run build` from `frontend/`. Walk through the affected browser flow for user-facing changes. |
| PDF export | Compile a representative synthetic resume and inspect the generated PDF when changing the template, export, or compiler behavior. |
| Python CLI | Follow the [local verification commands](backend/agents/README.md#local-verification) for graph or tool changes. |

Report the checks run and the ones that still need a configured service or local dependency.

## 4. Runtime interfaces and configuration

| Interface | Routes |
| --- | --- |
| Health | `GET /health` |
| Resume workflow | `GET /api/resume-status`, `POST /api/upload-resume`, `POST /api/additional-projects`, `POST /api/job-description`, `POST /api/analyze-fit`, `POST /api/optimize`, `POST /api/generate-pdf`, `POST /api/generate-application-package` |
| Builder | `POST /api/builder/generate-pdf` |
| Waitlist | `GET /api/waitlist/status`, `POST /api/waitlist/notify-signup` |
| Waitlist admin | `GET /api/admin/waitlist`, `POST /api/admin/waitlist/update` |

The optimization and application-package routes share a per-user rate limiter. [Route registration](backend/main.go), [Rate limiter](backend/httputil/ratelimit.go)

| Runtime | Configuration |
| --- | --- |
| Frontend | `VITE_FIREBASE_API_KEY`, `VITE_FIREBASE_AUTH_DOMAIN`, and `VITE_FIREBASE_PROJECT_ID` configure Firebase. |
| Go identity and storage | `AUTH_PROVIDER=firebase` enables token validation. `FIREBASE_PROJECT_ID` is required for Firestore, which uses Google application credentials. The container entrypoint can load credentials from `GOOGLE_APPLICATION_CREDENTIALS_JSON`. |
| Go inference | `OPENROUTER_API_KEY` is read when making an LLM call. The endpoint and `anthropic/claude-sonnet-4` model are constants in Go. |
| Go operations | `PORT` defaults to `3001`. `APPLICATIONS_ROOT_PATH` overrides the PDF artifact root. `ADMIN_UIDS`, `RESEND_API_KEY`, and `RESEND_FROM_EMAIL` configure waitlist administration and email. |

[Frontend configuration](frontend/src/firebase.ts), [Go configuration](backend/config/config.go), [Container entrypoint](backend/entrypoint.sh), [Go inference client](backend/llm/client.go)

## 5. Architecture decisions in Git history

| Decision | Implementation record |
| --- | --- |
| Use Firebase for sign-in and Firestore for persistent base resumes. | `1be638a` migrated auth and introduced Firestore. `a322870` moved base-resume storage to Firestore. |
| Focus the web workflow on resumes and offer a separate template-based builder. | `51d7733` removed cover-letter generation. `07fce92` added the LaTeX resume builder. |
| Keep resume analysis separate from LLM tailoring in the web flow. | `d3eee1b` adds a deterministic Go analyzer before optimization. The Python LangGraph CLI remains independently runnable. |
| Allow users to supply project evidence for tailoring. | `25c4f9d` adds a pool of user-entered projects for the optimizer. Its presence changes how the Projects section is handled. |
| Define supported LaTeX packages for PDF export. | `37f4221` adds package validation and updates TeX Live support in the backend image. |
