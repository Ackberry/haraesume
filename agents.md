# Haraesume Agentic Architecture: A Systems Report on Hybrid LLM Orchestration and Graph-Based Resume Analysis

## Abstract
We present a technical study of the current agent architecture implemented in `haraesume`, a resume optimization system that combines prompt-constrained generation with deterministic analysis. The system is hybrid by design: a Go-based service layer orchestrates production-facing LLM agents for resume rewriting, cover-letter generation, and organization extraction, while a Python LangGraph pipeline executes structured ATS checks, requirement parsing, skill-gap analysis, and recommendation synthesis. We report the implemented state models, tool contracts, routing logic, fallback controls, and reproducibility interfaces. Our findings show that the architecture emphasizes practical robustness through layered constraints, parser-based output validation, and deterministic degradation paths; however, it remains limited by heuristic parsing and prompt-level guarantees rather than formal policy enforcement.

## 1. Introduction
Recent LLM applications often choose between two paradigms: direct prompt orchestration (fast to build, harder to constrain) and graph/tool-driven execution (more explicit control, additional engineering overhead). In this work, we implemented both paradigms in one repository and used each where it is operationally strongest.

Specifically, the production HTTP path uses prompt-defined agents to generate candidate artifacts (resume and cover letter), while a separate analytical path uses LangGraph and explicit tools to compute interpretable diagnostics (ATS warnings, coverage scores, and skill gaps). This report documents the architecture as implemented in code, not as a conceptual target.

## 2. Materials and Codebase Evidence
Our analysis uses direct inspection of the following artifacts:

- `/Users/ackberry/Desktop/College/haraesume/backend/main.go`
- `/Users/ackberry/Desktop/College/haraesume/backend/agents/graph.py`
- `/Users/ackberry/Desktop/College/haraesume/backend/agents/tools.py`
- `/Users/ackberry/Desktop/College/haraesume/backend/agents/state.py`
- `/Users/ackberry/Desktop/College/haraesume/backend/agents/cli.py`
- `/Users/ackberry/Desktop/College/haraesume/backend/agents/requirements.txt`
- `/Users/ackberry/Desktop/College/haraesume/.codex/skills/interactive-coding-coach/SKILL.md`

## 3. System Overview
We identify three agent strata:

1. **Production LLM orchestration (Go runtime)**
- Resume Optimizer Agent
- Cover Letter Agent
- Company Name Extraction Agent

2. **Analytical graph execution (Python runtime)**
- LangGraph pipeline with six nodes and five tools

3. **Development-time assistant skills (Codex layer)**
- Behavior-guidance skills for authoring/development workflows
- Not part of backend runtime execution

### 3.1 Architectural Separation
The Go layer owns user-facing API flows and artifact generation. The Python layer owns structured diagnostics and report synthesis. This separation is deliberate: generation and analysis are decoupled to preserve operational clarity and simplify failure handling.

## 4. Methods: Production Agent Design (Go)

### 4.1 Shared Inference Interface
All Go-side agents call a shared client function, `runLLMWithTemperature(...)`, with:

- Provider endpoint: `https://openrouter.ai/api/v1/chat/completions`
- Default model: `anthropic/claude-sonnet-4`
- Credential: `OPENROUTER_API_KEY`
- Message structure: system prompt + user prompt + token cap (+ optional temperature)

This yields a consistent inference substrate across all generation/extraction operations.

### 4.2 Resume Optimizer Agent
Implemented function: `optimizeResumeWithLLM(resumeLatex, jobDescription)`.

#### 4.2.1 Prompt Policy
We enforce hard constraints in prompt form:
- LaTeX must remain compilable.
- Structure must be preserved.
- Experience/Projects/Leadership claims must not be invented.
- Changes should focus on Technical Skills.
- Skill additions are capped at 5.

#### 4.2.2 Pre-LLM Heuristic Conditioning
Before generation, we score candidate technical skills from a curated dictionary using keyword frequency and requirement emphasis signals. This narrows the model’s search space and reduces prompt drift.

#### 4.2.3 Post-LLM Validation and Repair
Returned content is parsed for full-document boundaries (`\documentclass` to `\end{document}`), then section-locked restoration is applied to preserve original `experience`, `projects`, and `leadership` blocks. This acts as a semantic guardrail independent of model compliance.

### 4.3 Cover Letter Agent (Two-Pass)
Implemented function: `generateCoverLetterWithLLM(resumeLatex, jobDescription)`.

#### 4.3.1 Generation Procedure
- Pass 1 (draft, `temperature=0.65`): produce complete LaTeX letter.
- Pass 2 (refine, `temperature=0.35`): improve precision, style, and robustness.

#### 4.3.2 Grounding Strategy
We convert resume LaTeX to plaintext and extract salient lines from both resume and job description. These are inserted as factual context to reduce fabrication and force role-relevant specificity.

#### 4.3.3 Fallback Behavior
If refinement fails, we return the draft. If neither output contains a complete LaTeX document, the endpoint returns an error.

### 4.4 Company Extraction Agent
Implemented function: `extractCompanyNameWithAgent(jobDescription)`.

This micro-agent is constrained to output strict JSON: `{"company_name":"..."}`. We parse permissively across format variants, then normalize. If extraction fails, we execute regex heuristics (`Company:`, `join X`, `at X`, `X is ...`) as deterministic fallback.

### 4.5 Production Orchestration Endpoint
`POST /api/generate-application-package` composes all three agents:

1. Optimize resume
2. Generate cover letter
3. Extract company name
4. Write intermediate `.tex` files
5. Compile PDFs
6. Return package metadata
7. Delete intermediate `.tex` files

The orchestration captures warnings for partial PDF failures and returns file paths for successful outputs.

## 5. Methods: Graph-Based Analytical Design (Python)

### 5.1 Graph Topology
In `/Users/ackberry/Desktop/College/haraesume/backend/agents/graph.py`, we define:

- Nodes: `validate`, `resume_check`, `job_analysis`, `matching`, `recommendations`, `finalize`
- Flow:
  - `START -> validate`
  - if invalid input: `validate -> finalize`
  - else: `validate -> resume_check -> job_analysis -> matching -> recommendations -> finalize -> END`

This topology cleanly isolates validation, extraction, scoring, and reporting phases.

### 5.2 State Contract
`/Users/ackberry/Desktop/College/haraesume/backend/agents/state.py` defines `ResumeMatchState` with typed fields for inputs, intermediates, and terminal outputs, including:

- Inputs: `resume_text`, `job_description`
- Diagnostics: `ats_checks`, `jd_requirements`, `match_scores`
- Alignment outputs: `missing_keywords`, `matched_keywords`, `strengths`
- Final output: `final_report`

### 5.3 Tooling Layer
Five tools are implemented in `/Users/ackberry/Desktop/College/haraesume/backend/agents/tools.py`:

1. `extract_keywords`
- Frequency-ranked keyword extraction with stopword filtering.

2. `ats_lint_resume`
- Checks word count, bullet density, section headers, and contact fields.

3. `parse_job_requirements`
- Extracts required/preferred skills using keyword overlap + requirement-language cues.
- Captures responsibilities and minimum years when detectable.

4. `compute_match_score`
- Computes required and preferred coverage; combines them via weighted sum:
- `overall_match = 0.7 * required_coverage + 0.3 * preferred_coverage`

5. `identify_skill_gaps`
- Returns matched and missing skill sets by set operations.

### 5.4 Recommendation Synthesis Node
`recommendations_node` performs dual-path synthesis:
- Deterministic recommendation seed list (always computed)
- Optional LLM enhancement via `ChatOpenAI` (OpenRouter-compatible)
- Strict JSON-list parsing with fallback to deterministic recommendations

This ensures recommendation output even without model credentials.

## 6. Runtime Interfaces and Reproducibility

### 6.1 Go API Surface
Implemented endpoints in `/Users/ackberry/Desktop/College/haraesume/backend/main.go`:

- `GET /health`
- `GET /api/resume-status`
- `POST /api/upload-resume`
- `POST /api/job-description`
- `POST /api/optimize`
- `POST /api/cover-letter`
- `POST /api/generate-application-package`
- `POST /api/generate-cover-letter-pdf`
- `POST /api/generate-pdf`

### 6.2 LangGraph CLI Surface
`/Users/ackberry/Desktop/College/haraesume/backend/agents/cli.py`:

```bash
cd /Users/ackberry/Desktop/College/haraesume/backend
python -m agents --resume-file ../sample_resume.tex --jd-file ./job_description.txt
```

Optional full state:

```bash
python -m agents --resume-file ../sample_resume.tex --jd-file ./job_description.txt --json
```

### 6.3 Environment Variables
Go runtime:
- Required: `OPENROUTER_API_KEY`
- Optional: `RESUME_STORE_PATH`, `APPLICATIONS_ROOT_PATH`

Python runtime:
- Credential: `OPENROUTER_API_KEY` or `OPENAI_API_KEY`
- Optional overrides: `OPENROUTER_MODEL`, `OPENROUTER_BASE_URL`

## 7. Results: Observed Robustness Properties
From implemented code paths, we observe the following robustness mechanisms:

1. **Layered fallback logic**
- Cover-letter pipeline falls back from refined output to draft output.
- Recommendation node falls back from model output to deterministic heuristics.
- Company extraction falls back from model JSON extraction to regex heuristics.

2. **Output-shape constraints**
- Company extraction uses explicit JSON schema constraints.
- Resume and cover-letter agents require full LaTeX document delimiters.

3. **Post-generation repair controls**
- Resume section locking restores high-risk sections after generation.

4. **Failure-visible orchestration**
- Package endpoint returns warnings for partial PDF compilation failures rather than silently failing.

5. **Deterministic early routing**
- LangGraph `validate` node short-circuits invalid requests to `finalize` with explicit error messaging.

## 8. Discussion
The architecture demonstrates a pragmatic hybrid model: prompt-driven generation where linguistic flexibility is needed, and graph/tool-based execution where interpretability and repeatability are required. This separation improves maintainability and debuggability, but it introduces semantic duplication risk between Go-side optimization logic and Python-side analytical logic.

A notable strength is the explicit use of parser and restore steps after LLM generation, which is a concrete mechanism beyond prompt-only control. A current weakness is that many constraints are still soft (instructional) rather than hard (schema-enforced with strict validators across all outputs).

## 9. Threats to Validity and Limitations

1. **Prompt-level enforcement is not formal safety**
- Constraint text does not guarantee semantic compliance under all model behaviors.

2. **Heuristic extraction bias**
- ATS and requirement parsing use lightweight heuristics and may underperform for atypical resumes/JDs.

3. **Uncertainty not quantified**
- Match and recommendation outputs expose no confidence intervals or calibration metadata.

4. **Cross-runtime drift risk**
- Go and Python logic may evolve independently without a shared canonical scoring contract.

## 10. Conclusion
We implemented and analyzed a dual-runtime agentic system that combines constrained generation with structured analysis. The production path emphasizes artifact generation and packaging, while the LangGraph path emphasizes interpretable diagnostics and recommendation synthesis. The current system is operationally robust for iterative resume tailoring and provides multiple deterministic fallbacks; future work should prioritize stronger structured-output validation and alignment between generation and analysis semantics.

## References
[1] Go `net/http` package documentation. [https://pkg.go.dev/net/http](https://pkg.go.dev/net/http)

[2] Go `os/exec` package documentation. [https://pkg.go.dev/os/exec](https://pkg.go.dev/os/exec)

[3] Python `argparse` documentation. [https://docs.python.org/3/library/argparse.html](https://docs.python.org/3/library/argparse.html)

[4] Python `typing.TypedDict` documentation. [https://docs.python.org/3/library/typing.html#typing.TypedDict](https://docs.python.org/3/library/typing.html#typing.TypedDict)

[5] LangGraph documentation. [https://langchain-ai.github.io/langgraph/](https://langchain-ai.github.io/langgraph/)

[6] LangChain tools concept documentation. [https://python.langchain.com/docs/concepts/tools/](https://python.langchain.com/docs/concepts/tools/)

[7] LangChain OpenAI chat integration documentation. [https://python.langchain.com/docs/integrations/chat/openai/](https://python.langchain.com/docs/integrations/chat/openai/)

[8] OpenRouter API reference documentation. [https://openrouter.ai/docs/api-reference/overview](https://openrouter.ai/docs/api-reference/overview)

[9] JSON Data Interchange Standard (RFC 8259). [https://www.rfc-editor.org/rfc/rfc8259](https://www.rfc-editor.org/rfc/rfc8259)
