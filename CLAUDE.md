# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Haraesume is an AI-powered resume optimization platform. Users upload a LaTeX (.tex) resume, provide a job description, and the system uses Claude (via OpenRouter API) to tailor the resume for that job. It also generates cover letters and compiles PDFs via TeX Live.

## Architecture

- **Frontend** (`frontend/`): React 19 + TypeScript SPA built with Vite. Uses Chakra UI + TailwindCSS for styling, Firebase Auth (Google sign-in) for authentication, Firestore for document storage. Single `App.tsx` implements a 4-step wizard flow: upload → job description → optimize → download.
- **Backend** (`backend/`): Go HTTP server on port 3001. Package-per-feature layout (`auth/`, `resume/`, `llm/`, `latex/`, `email/`, `waitlist/`, `httputil/`). Uses in-memory per-user state with Firestore persistence, OpenRouter API for LLM calls (Claude Sonnet 4), and pdflatex for PDF compilation.
- **Python Agents** (`backend/agents/`): LangGraph-based analytical pipeline for resume analysis (keyword extraction, ATS linting, match scoring). Invoked via CLI.

### Data Flow

User authenticates via Firebase → uploads .tex resume (stored in Firestore with 7-day TTL) → provides job description → backend calls LLM optimizer → returns modified LaTeX + summary → compiles PDF via pdflatex → organizes output in `applications/<user_hash>/<company>/`.

## Build & Development Commands

### Backend
```bash
cd backend
go run .              # Run server (port 3001)
go test ./...         # Run all tests
go build -o backend . # Build binary
```

### Frontend
```bash
cd frontend
npm run dev           # Vite dev server (port 5173, proxies API to backend)
npm run build         # Production build (tsc -b && vite build)
npm run lint          # ESLint
```

### Full Stack (Docker)
```bash
docker-compose up     # Frontend @ :8080, Backend @ :3001
```

## Key Environment Variables

**Backend**: `OPENROUTER_API_KEY`, `OPENROUTER_MODEL` (default: `anthropic/claude-sonnet-4`), `AUTH_PROVIDER=firebase`, `FIREBASE_PROJECT_ID=harae-86aff`, `RESEND_API_KEY`, `RESEND_FROM_EMAIL`, `APP_URL`

**Frontend**: `VITE_FIREBASE_API_KEY`, `VITE_FIREBASE_AUTH_DOMAIN`, `VITE_FIREBASE_PROJECT_ID`

## Security Considerations

- Prompt injection prevention via regex filtering + ChatML token sanitization (`llm/sanitize.go`)
- Firebase JWT validation with JWKS caching (`auth/firebase.go`)
- Per-user LLM rate limiting: 1 request/minute (`httputil/ratelimit.go`)
- User directories are SHA256-hashed
