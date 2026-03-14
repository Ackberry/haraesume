from __future__ import annotations

import json
import os
import re
from functools import lru_cache
from typing import Any

from langchain_core.messages import HumanMessage, SystemMessage
from langchain_openai import ChatOpenAI
from langgraph.graph import END, START, StateGraph

from .state import ResumeMatchState
from .tools import (
    ats_lint_resume,
    compute_match_score,
    extract_keywords,
    identify_skill_gaps,
    parse_job_requirements,
)


@lru_cache(maxsize=1)
def _build_model() -> ChatOpenAI | None:
    api_key = os.getenv("OPENROUTER_API_KEY") or os.getenv("OPENAI_API_KEY")
    if not api_key:
        return None

    model_name = os.getenv("OPENROUTER_MODEL", "anthropic/claude-sonnet-4")
    base_url = os.getenv("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1")
    return ChatOpenAI(
        model=model_name,
        api_key=api_key,
        base_url=base_url,
        temperature=0.2,
    )


_ADJACENT_SKILL_THRESHOLD = 5


def _infer_adjacent_skills(jd_text: str, jd_requirements: dict[str, Any], model) -> list[str]:
    """Ask the LLM for adjacent high-demand skills when the JD has sparse skill signals."""
    existing = jd_requirements.get("required_skills", []) + jd_requirements.get("preferred_skills", [])
    prompt = (
        "You are a technical recruiting expert. A job description has few explicit skill signals. "
        "Based on its content and responsibilities, list adjacent high-demand skills that commonly "
        "appear in similar job postings at other companies. "
        "Respond with ONLY a JSON array of lowercase skill strings. Max 10 items."
    )
    payload = {
        "job_description_excerpt": jd_text[:1500],
        "detected_skills": existing,
        "responsibilities": jd_requirements.get("responsibilities", [])[:4],
    }
    result = model.invoke(
        [
            SystemMessage(content=prompt),
            HumanMessage(content=json.dumps(payload)),
        ]
    )
    content = getattr(result, "content", "").strip()
    # Strip markdown code fences if present
    content = re.sub(r"^```[a-z]*\n?", "", content).rstrip("`").strip()
    try:
        skills = json.loads(content)
        if isinstance(skills, list):
            return [str(s).lower().strip() for s in skills if str(s).strip()][:10]
    except json.JSONDecodeError:
        pass
    return []


def validate_input_node(state: ResumeMatchState) -> ResumeMatchState:
    resume = (state.get("resume_text") or "").strip()
    job_description = (state.get("job_description") or "").strip()
    if not resume or not job_description:
        return {"error": "Both resume_text and job_description are required."}
    return {}


def route_from_validation(state: ResumeMatchState) -> str:
    if state.get("error"):
        return "finalize"
    return "resume_check"


def resume_check_node(state: ResumeMatchState) -> ResumeMatchState:
    resume_text = state["resume_text"]
    ats_checks = ats_lint_resume.invoke({"resume_text": resume_text})
    resume_keywords = extract_keywords.invoke({"text": resume_text, "max_keywords": 40})
    return {
        "ats_checks": ats_checks,
        "resume_keywords": resume_keywords,
    }


def job_analysis_node(state: ResumeMatchState) -> ResumeMatchState:
    job_description = state["job_description"]
    jd_requirements = parse_job_requirements.invoke({"job_description": job_description})
    jd_keywords = extract_keywords.invoke({"text": job_description, "max_keywords": 40})

    total_skills = len(jd_requirements.get("required_skills", [])) + len(jd_requirements.get("preferred_skills", []))
    if total_skills < _ADJACENT_SKILL_THRESHOLD:
        model = _build_model()
        if model is not None:
            adjacent = _infer_adjacent_skills(job_description, jd_requirements, model)
            if adjacent:
                existing_required = set(jd_requirements.get("required_skills", []))
                existing_preferred = set(jd_requirements.get("preferred_skills", []))
                augmented_preferred = sorted(existing_preferred | (set(adjacent) - existing_required))
                jd_requirements = {**jd_requirements, "preferred_skills": augmented_preferred}

    return {
        "jd_requirements": jd_requirements,
        "jd_keywords": jd_keywords,
    }


def matching_node(state: ResumeMatchState) -> ResumeMatchState:
    scores = compute_match_score.invoke(
        {
            "resume_keywords": state.get("resume_keywords", []),
            "jd_requirements": state.get("jd_requirements", {}),
        }
    )
    gaps = identify_skill_gaps.invoke(
        {
            "resume_keywords": state.get("resume_keywords", []),
            "jd_requirements": state.get("jd_requirements", {}),
        }
    )
    strengths = gaps.get("matched", [])[:8]
    return {
        "match_scores": scores,
        "missing_keywords": gaps.get("missing", []),
        "matched_keywords": gaps.get("matched", []),
        "strengths": strengths,
    }


def _parse_recommendations_from_text(text: Any) -> list[str]:
    if isinstance(text, list):
        cleaned = [str(item).strip() for item in text if str(item).strip()]
        return cleaned[:10]
    if not isinstance(text, str):
        return []

    raw = text.strip()
    if not raw:
        return []

    try:
        parsed = json.loads(raw)
        if isinstance(parsed, list):
            return [str(item).strip() for item in parsed if str(item).strip()][:10]
    except json.JSONDecodeError:
        pass

    lines = []
    for line in raw.splitlines():
        stripped = line.strip().lstrip("-*0123456789. ").strip()
        if stripped:
            lines.append(stripped)
    return lines[:10]


def recommendations_node(state: ResumeMatchState) -> ResumeMatchState:
    default_recommendations = []
    for item in state.get("missing_keywords", [])[:6]:
        default_recommendations.append(f"Add quantified evidence for {item} experience if applicable.")
    for warning in state.get("ats_checks", {}).get("warnings", [])[:4]:
        default_recommendations.append(warning)
    if not default_recommendations:
        default_recommendations.append("Refine bullet points to show impact using numbers and outcomes.")

    model = _build_model()
    if model is None:
        return {"recommendations": default_recommendations[:10]}

    prompt = (
        "You are a senior resume reviewer. Generate a JSON array with up to 10 concrete resume improvements. "
        "Prioritize gaps vs job requirements and ATS issues. Keep each item under 18 words."
    )
    payload = {
        "match_scores": state.get("match_scores", {}),
        "missing_keywords": state.get("missing_keywords", []),
        "ats_warnings": state.get("ats_checks", {}).get("warnings", []),
        "top_job_keywords": state.get("jd_keywords", [])[:20],
        "top_resume_keywords": state.get("resume_keywords", [])[:20],
    }
    result = model.invoke(
        [
            SystemMessage(content=prompt),
            HumanMessage(content=json.dumps(payload, indent=2)),
        ]
    )
    recommendations = _parse_recommendations_from_text(getattr(result, "content", ""))
    if not recommendations:
        recommendations = default_recommendations
    return {"recommendations": recommendations[:10]}


def finalize_node(state: ResumeMatchState) -> ResumeMatchState:
    if state.get("error"):
        return {"final_report": f"Error: {state['error']}"}

    scores = state.get("match_scores", {})
    report_lines = [
        "# Resume Match Report",
        "",
        f"- Overall Match: {scores.get('overall_match', 0.0)}%",
        f"- Required Skill Coverage: {scores.get('required_coverage', 0.0)}%",
        f"- Preferred Skill Coverage: {scores.get('preferred_coverage', 0.0)}%",
        "",
        "## Strengths",
    ]
    strengths = state.get("strengths", [])
    if strengths:
        report_lines.extend([f"- {item}" for item in strengths])
    else:
        report_lines.append("- No strong skill overlap detected yet.")

    report_lines.append("")
    report_lines.append("## Skill Gaps")
    missing = state.get("missing_keywords", [])
    if missing:
        report_lines.extend([f"- {item}" for item in missing[:12]])
    else:
        report_lines.append("- No major keyword gaps found.")

    report_lines.append("")
    report_lines.append("## ATS Warnings")
    ats_warnings = state.get("ats_checks", {}).get("warnings", [])
    if ats_warnings:
        report_lines.extend([f"- {item}" for item in ats_warnings])
    else:
        report_lines.append("- No ATS warnings from baseline checks.")

    report_lines.append("")
    report_lines.append("## Recommended Improvements")
    recommendations = state.get("recommendations", [])
    if recommendations:
        report_lines.extend([f"- {item}" for item in recommendations])
    else:
        report_lines.append("- Improve alignment between project bullets and job requirements.")

    return {"final_report": "\n".join(report_lines)}


def build_resume_match_graph():
    graph = StateGraph(ResumeMatchState)

    graph.add_node("validate", validate_input_node)
    graph.add_node("resume_check", resume_check_node)
    graph.add_node("job_analysis", job_analysis_node)
    graph.add_node("matching", matching_node)
    graph.add_node("recommendations", recommendations_node)
    graph.add_node("finalize", finalize_node)

    graph.add_edge(START, "validate")
    graph.add_conditional_edges(
        "validate",
        route_from_validation,
        {
            "resume_check": "resume_check",
            "finalize": "finalize",
        },
    )
    graph.add_edge("resume_check", "job_analysis")
    graph.add_edge("job_analysis", "matching")
    graph.add_edge("matching", "recommendations")
    graph.add_edge("recommendations", "finalize")
    graph.add_edge("finalize", END)
    return graph.compile()


def run_resume_match(resume_text: str, job_description: str) -> ResumeMatchState:
    app = build_resume_match_graph()
    return app.invoke({"resume_text": resume_text, "job_description": job_description})
