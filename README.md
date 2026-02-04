# Haraesume - AI-Powered Resume Optimizer

A full-stack web application that optimizes LaTeX resumes for specific job descriptions using AI and generates tailored cover letters.

![Rust](https://img.shields.io/badge/Rust-000000?style=flat&logo=rust&logoColor=white)
![React](https://img.shields.io/badge/React-20232A?style=flat&logo=react&logoColor=61DAFB)
![TypeScript](https://img.shields.io/badge/TypeScript-007ACC?style=flat&logo=typescript&logoColor=white)

## Features

- 📄 **LaTeX Resume Upload** - Drag-and-drop your `.tex` resume
- 🎯 **AI-Powered Optimization** - Tailors your resume to match job descriptions
- ✉️ **Cover Letter Generation** - Creates personalized cover letters
- 📥 **PDF Export** - Compiles optimized LaTeX to downloadable PDF
- 🔍 **ATS-Friendly** - Ensures compatibility with Applicant Tracking Systems

## Tech Stack

| Layer | Technology |
|-------|------------|
| Backend | Rust, Axum, Tokio |
| Frontend | React, TypeScript, Vite, TailwindCSS |
| LLM | OpenRouter API (Claude) |
| PDF | TeX Live (pdflatex) |

## Project Structure

```
haraesume/
├── backend/                 # Rust API server
│   ├── Cargo.toml
│   └── src/
│       ├── main.rs          # Axum routes & server
│       ├── llm.rs           # OpenRouter integration
│       └── latex.rs         # PDF compilation
├── frontend/                # React SPA
│   ├── package.json
│   ├── vite.config.ts
│   └── src/
│       ├── App.tsx          # 4-step wizard UI
│       └── index.css        # Dark theme styles
└── sample_resume.tex        # Test resume
```

## Prerequisites

- **Rust** (1.70+)
- **Node.js** (20+)
- **TeX Live** (for `pdflatex`)
- **OpenRouter API Key**

## Quick Start

### 1. Clone & Setup

```bash
git clone https://github.com/Ackberry/haraesume.git
cd haraesume
```

### 2. Start Backend

```powershell
cd backend
$env:OPENROUTER_API_KEY = "your-openrouter-api-key"
cargo run --release
# Server: http://localhost:3001
```

### 3. Start Frontend

```powershell
cd frontend
npm install
npm run dev
# App: http://localhost:5173
```

### 4. Use the App

1. Open http://localhost:5173
2. Upload a `.tex` resume (use `sample_resume.tex` to test)
3. Paste the job description
4. Click **Optimize Resume**
5. Download PDF or generate a cover letter

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/api/upload-resume` | POST | Upload LaTeX file (multipart) |
| `/api/job-description` | POST | Set job description |
| `/api/optimize` | POST | Optimize resume with AI |
| `/api/cover-letter` | POST | Generate cover letter |
| `/api/generate-pdf` | POST | Compile to PDF (base64) |

## Environment Variables

| Variable | Description |
|----------|-------------|
| `OPENROUTER_API_KEY` | Your OpenRouter API key |
| `RUST_LOG` | Log level (default: `info`) |

## Development

### Backend

```bash
cd backend
cargo check      # Type check
cargo test       # Run tests
cargo run        # Development server
```

### Frontend

```bash
cd frontend
npm run dev      # Dev server with HMR
npm run build    # Production build
npm run preview  # Preview production build
```

## License

MIT

---

Built with ♥ using Rust and React
