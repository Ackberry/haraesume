# LangChain and LangGraph Agent Specification

The Python CLI analyzes a resume against a job description and produces an ATS and skill-coverage report. The Go web API has its own execution path. See the [repository architecture and development harness](../../agents.md).

## Graph and state

```text
Valid input:   START → validate → resume_check → job_analysis → matching → recommendations → finalize → END
Invalid input: START → validate → finalize → END
```

| Node | Behavior |
| --- | --- |
| `validate` | Checks for missing or oversized inputs, sanitizes accepted text, and routes invalid input to `finalize`. |
| `resume_check` | Runs ATS checks and extracts resume keywords. |
| `job_analysis` | Parses requirements and extracts job keywords. With a configured model and fewer than five detected skills, it can add inferred preferred skills. |
| `matching` | Scores required and preferred coverage and derives matched skills, missing skills, and strengths. |
| `recommendations` | Builds deterministic suggestions and optionally asks the model for improvements. |
| `finalize` | Renders a Markdown report or a validation error. |

[`ResumeMatchState`](state.py) holds the input text, validation error, ATS checks, keywords, parsed requirements, scores, skill gaps, strengths, recommendations, and final report. [Graph implementation](graph.py)

## LangChain tools

| Tool | Contract |
| --- | --- |
| `extract_keywords` | Returns frequency-ranked keywords after tokenization and stopword filtering. |
| `ats_lint_resume` | Returns word and bullet counts, section and contact checks, and warnings. |
| `parse_job_requirements` | Returns likely required and preferred skills, responsibilities, and a years-of-experience signal. |
| `compute_match_score` | Returns required, preferred, and overall coverage percentages. Overall coverage weights required skills at 70% and preferred skills at 30%. |
| `identify_skill_gaps` | Returns matched and missing skills from the resume keywords and parsed requirements. |

Coverage measures heuristic keyword overlap. [Tool implementation](tools.py)

## Model calls and fallbacks

`OPENROUTER_API_KEY` or `OPENAI_API_KEY` enables model calls. `OPENROUTER_MODEL` and `OPENROUTER_BASE_URL` override the Python defaults of `anthropic/claude-sonnet-4` and `https://openrouter.ai/api/v1`.

The recommendation parser accepts a JSON list and can fall back to line parsing. The graph uses deterministic suggestions when credentials are absent or parsing yields no recommendations. Model invocation errors can propagate. [Model configuration and recommendation node](graph.py)

## Local verification

From the repository root, install the [Python dependencies](requirements.txt), then use a local synthetic resume and job description. The default output is Markdown. Add `--json` to inspect the full state. [CLI implementation](cli.py)

```bash
python -m pip install -r backend/agents/requirements.txt
cd backend
env -u OPENROUTER_API_KEY -u OPENAI_API_KEY python -m agents \
  --resume-file /path/to/synthetic-resume.txt --jd-file /path/to/job-description.txt
env -u OPENROUTER_API_KEY -u OPENAI_API_KEY python -m agents \
  --resume-file /path/to/synthetic-resume.txt --jd-file /path/to/job-description.txt --json
```

Check that valid inputs produce scores and recommendations. Repeat with an empty input file to check that validation returns an error report. For changes to model-assisted behavior, exercise model response parsing with a stub before making live calls.
