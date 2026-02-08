# Agents and Tools Architecture

This file documents the current agent structure in this repository, including:

- Runtime agents in the Go backend (`/Users/ackberry/Desktop/College/haraesume/backend/main.go`)
- LangGraph multi-node agents in Python (`/Users/ackberry/Desktop/College/haraesume/backend/agents`)
- Development-time Codex skills available in this workspace/session

## 1) Agent Types (What Exists Today)

| Agent Type | Location | Purpose | Trigger |
|---|---|---|---|
| Resume Optimizer Agent | `/Users/ackberry/Desktop/College/haraesume/backend/main.go` | Tailor LaTeX resume toward a job description while preserving core content | `POST /api/optimize`, `POST /api/generate-application-package` |
| Cover Letter Agent | `/Users/ackberry/Desktop/College/haraesume/backend/main.go` | Generate and refine a full LaTeX cover letter | `POST /api/cover-letter`, `POST /api/generate-application-package` |
| Company Name Extraction Agent | `/Users/ackberry/Desktop/College/haraesume/backend/main.go` | Extract hiring company from job description (JSON contract) | Called during `POST /api/generate-application-package` |
| Resume Match LangGraph Agent Flow | `/Users/ackberry/Desktop/College/haraesume/backend/agents/graph.py` | ATS checks, requirement parsing, scoring, gaps, recommendations, final report | Python CLI (`python -m agents ...`) or import `run_resume_match` |

## 2) Go Backend Agent Stack

### 2.1 Shared LLM Runtime

- Endpoint: OpenRouter Chat Completions (`https://openrouter.ai/api/v1/chat/completions`)
- Default model: `anthropic/claude-sonnet-4`
- API key: `OPENROUTER_API_KEY`
- Core function: `runLLMWithTemperature(...)`
- Message format: system + user messages, max token limit, optional temperature

### 2.2 Resume Optimizer Agent

Function: `optimizeResumeWithLLM(resumeLatex, jobDescription)`

Behavior:
- Builds missing skill suggestions from a curated list (priority-scored).
- Prompts model with strict constraints:
  - Keep valid compilable LaTeX
  - Preserve structure
  - Avoid fabricated facts
  - Focus edits on Technical Skills
  - Add at most 5 missing skills
- Expects output format:
  - Full LaTeX document
  - Separator `---CHANGES---`
  - Bullet change summary
- Post-processes output:
  - Parses and extracts full LaTeX doc
  - Restores locked sections from original resume:
    - `experience`
    - `projects`
    - `leadership`

### 2.3 Cover Letter Agent (Two-Pass)

Function: `generateCoverLetterWithLLM(resumeLatex, jobDescription)`

Behavior:
- Converts resume LaTeX to plaintext context.
- Picks top resume/job highlights.
- Pass 1 (draft):
  - Temperature `0.65`
  - Produces full LaTeX business-letter style output.
- Pass 2 (refine):
  - Temperature `0.35`
  - Edits draft for polish, precision, and LaTeX correctness.
- Fallback strategy:
  - If refinement fails, returns draft.
  - If draft parse fails, returns error.

### 2.4 Company Extraction Agent

Function: `extractCompanyNameWithAgent(jobDescription)`

Behavior:
- Uses a strict system prompt requiring exact JSON shape:
  - `{"company_name":"..."}`
- Parses agent output robustly.
- If agent fails, pipeline falls back to heuristic extraction:
  - Regex patterns such as `Company:`, `join X`, `at X`, `X is ...`.

### 2.5 Package Generation Orchestration

`POST /api/generate-application-package` does:

1. Optimize resume (agent)
2. Generate cover letter (agent)
3. Extract company name (agent + heuristic fallback)
4. Save `.tex` in `applications/<company>/`
5. Compile PDFs
6. Return structured response with paths/warnings
7. Delete generated `.tex` files after packaging

## 3) LangGraph Agent Flow (`backend/agents`)

### 3.1 Graph Nodes

Defined in `/Users/ackberry/Desktop/College/haraesume/backend/agents/graph.py`:

1. `validate`
2. `resume_check`
3. `job_analysis`
4. `matching`
5. `recommendations`
6. `finalize`

Edges:

- `START -> validate`
- Conditional:
  - if invalid input -> `finalize`
  - else -> `resume_check`
- `resume_check -> job_analysis -> matching -> recommendations -> finalize -> END`

### 3.2 LangGraph Tools (Exact Tool Set)

Defined in `/Users/ackberry/Desktop/College/haraesume/backend/agents/tools.py`:

1. `extract_keywords(text, max_keywords=30)`
- Tokenizes text, removes stopwords, counts term frequency, returns top terms.

2. `ats_lint_resume(resume_text)`
- Checks:
  - word count
  - bullet count
  - core sections (`experience`, `skills`, `education`, `projects`)
  - contact info (email, phone)
- Returns warnings and diagnostics.

3. `parse_job_requirements(job_description)`
- Extracts required/preferred skills via:
  - keyword extraction
  - skill hint list
  - required-signal regex (`must|required|requirement`)
- Also extracts:
  - responsibilities
  - minimum years of experience

4. `compute_match_score(resume_keywords, jd_requirements)`
- Computes:
  - `required_coverage`
  - `preferred_coverage`
  - `overall_match = 0.7 * required + 0.3 * preferred`

5. `identify_skill_gaps(resume_keywords, jd_requirements)`
- Returns:
  - `matched`
  - `missing`

### 3.3 Recommendations Node Behavior

`recommendations_node` in `/Users/ackberry/Desktop/College/haraesume/backend/agents/graph.py`:

- Always builds deterministic fallback recommendations first.
- If model credentials exist:
  - uses `ChatOpenAI` (OpenRouter-compatible config)
  - asks model for JSON array (max 10 short improvements)
  - parses model output safely
- If model unavailable or parse fails:
  - falls back to deterministic recommendation list

### 3.4 State Contract

Typed state in `/Users/ackberry/Desktop/College/haraesume/backend/agents/state.py`:

- Inputs:
  - `resume_text`
  - `job_description`
- Intermediates:
  - `ats_checks`
  - `resume_keywords`
  - `jd_requirements`
  - `jd_keywords`
  - `match_scores`
  - `missing_keywords`
  - `matched_keywords`
  - `strengths`
  - `recommendations`
  - `error`
- Output:
  - `final_report`

### 3.5 CLI Entrypoint

`/Users/ackberry/Desktop/College/haraesume/backend/agents/cli.py` supports:

- `--resume-file <path>`
- `--jd-file <path>`
- `--json` (optional full state output)

Run example:

```bash
cd /Users/ackberry/Desktop/College/haraesume/backend
python -m agents --resume-file ../sample_resume.tex --jd-file ./job_description.txt
```

## 4) End-to-End Functional View

```text
User Input
  -> Go API route
    -> LLM agent(s) (optimize / cover letter / company extraction)
      -> parse/validate outputs
        -> compile PDFs + store files
          -> API response

or

User Input (CLI)
  -> LangGraph flow
    -> tool-based analysis + optional recommendation LLM
      -> markdown final report (or JSON state)
```

## 5) Environment and Dependencies

### 5.1 Go Agent Path

- Required:
  - `OPENROUTER_API_KEY`
- Optional:
  - `RESUME_STORE_PATH`
  - `APPLICATIONS_ROOT_PATH`

### 5.2 Python LangGraph Path

Dependencies in `/Users/ackberry/Desktop/College/haraesume/backend/agents/requirements.txt`:

- `langchain>=0.3.0`
- `langchain-core>=0.3.0`
- `langchain-openai>=0.2.0`
- `langgraph>=0.2.0`

Model environment for LangGraph path:

- API key: `OPENROUTER_API_KEY` or `OPENAI_API_KEY`
- Optional model override: `OPENROUTER_MODEL`
- Optional base URL override: `OPENROUTER_BASE_URL`

## 6) Development-Time Codex Skills (Not Production Runtime)

From workspace/session configuration:

1. `interactive-coding-coach`
- Local file:
  - `/Users/ackberry/Desktop/College/haraesume/.codex/skills/interactive-coding-coach/SKILL.md`
- Role:
  - interactive step-by-step coaching style

2. `skill-creator`
- System skill for creating/updating skills

3. `skill-installer`
- System skill for installing curated or repo-based skills

These skills affect Codex assistant behavior during development sessions, not the backend app runtime.

## 7) Current Structure Summary

- The repository currently has two distinct agent systems:
  - A Go HTTP agent pipeline for resume optimization, cover-letter generation, and company extraction.
  - A Python LangGraph analysis pipeline for ATS linting and resume-job match reporting.
- The LangGraph pipeline has 5 explicit tools and 6 graph nodes with deterministic fallbacks.
- The Go pipeline uses prompt-driven agents with parsing/validation and fallback logic integrated into endpoint orchestration.
