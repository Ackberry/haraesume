from __future__ import annotations

from typing import Any, TypedDict


class ResumeMatchState(TypedDict, total=False):
    resume_text: str
    job_description: str
    error: str
    ats_checks: dict[str, Any]
    resume_keywords: list[str]
    jd_requirements: dict[str, Any]
    jd_keywords: list[str]
    match_scores: dict[str, float]
    missing_keywords: list[str]
    matched_keywords: list[str]
    strengths: list[str]
    recommendations: list[str]
    final_report: str
