from __future__ import annotations

import re
from collections import Counter
from typing import Any

from langchain_core.tools import tool

STOPWORDS = {
    "a",
    "an",
    "and",
    "are",
    "as",
    "at",
    "be",
    "by",
    "for",
    "from",
    "has",
    "in",
    "is",
    "it",
    "of",
    "on",
    "or",
    "that",
    "the",
    "to",
    "was",
    "were",
    "will",
    "with",
    "you",
    "your",
    "our",
    "we",
}

SKILL_HINTS = {
    "python",
    "go",
    "java",
    "javascript",
    "typescript",
    "react",
    "node",
    "docker",
    "kubernetes",
    "aws",
    "gcp",
    "azure",
    "sql",
    "postgres",
    "mysql",
    "redis",
    "graphql",
    "rest",
    "api",
    "terraform",
    "ansible",
    "linux",
    "machine",
    "learning",
    "langchain",
    "langgraph",
    "llm",
    "nlp",
}


def _tokenize(text: str) -> list[str]:
    tokens = re.findall(r"[A-Za-z][A-Za-z0-9+\-#.]{1,}", text.lower())
    return [token for token in tokens if token not in STOPWORDS]


@tool("extract_keywords")
def extract_keywords(text: str, max_keywords: int = 30) -> list[str]:
    """Extract weighted keywords from resume or job description text."""
    if max_keywords <= 0:
        return []
    tokens = _tokenize(text)
    if not tokens:
        return []
    counts = Counter(tokens)
    ranked = [word for word, _ in counts.most_common(max_keywords)]
    return ranked


@tool("ats_lint_resume")
def ats_lint_resume(resume_text: str) -> dict[str, Any]:
    """Run ATS-friendly checks against resume text and return warnings."""
    text = resume_text.strip()
    word_count = len(_tokenize(text))
    bullet_count = len(re.findall(r"(?m)^\s*[-*\u2022]\s+", resume_text))

    sections = {}
    lowered = resume_text.lower()
    for name in ("experience", "skills", "education", "projects"):
        sections[name] = name in lowered

    has_email = bool(re.search(r"[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}", resume_text))
    has_phone = bool(re.search(r"(\+\d{1,3}[\s-]?)?(\(?\d{3}\)?[\s-]?\d{3}[\s-]?\d{4})", resume_text))

    warnings = []
    if word_count < 250:
        warnings.append("Resume appears too short for most professional roles.")
    if bullet_count < 6:
        warnings.append("Add more bullet points with measurable impact.")
    if not sections["experience"]:
        warnings.append("Missing clear Experience section heading.")
    if not sections["skills"]:
        warnings.append("Missing dedicated Skills section heading.")
    if not has_email:
        warnings.append("Missing email address in contact info.")
    if not has_phone:
        warnings.append("Missing phone number in contact info.")

    return {
        "word_count": word_count,
        "bullet_count": bullet_count,
        "sections": sections,
        "contact": {"email": has_email, "phone": has_phone},
        "warnings": warnings,
    }


@tool("parse_job_requirements")
def parse_job_requirements(job_description: str) -> dict[str, Any]:
    """Parse likely required and preferred skills from a job description."""
    lowered = job_description.lower()
    jd_keywords = extract_keywords.invoke({"text": job_description, "max_keywords": 40})

    required_skills = []
    preferred_skills = []
    for keyword in jd_keywords:
        if keyword in SKILL_HINTS:
            if re.search(rf"(must|required|requirement).{{0,80}}\b{re.escape(keyword)}\b", lowered):
                required_skills.append(keyword)
            else:
                preferred_skills.append(keyword)

    if not required_skills:
        required_skills = [k for k in jd_keywords if k in SKILL_HINTS][:8]
    if not preferred_skills:
        preferred_skills = [k for k in jd_keywords if k in SKILL_HINTS and k not in required_skills][:8]

    years = re.findall(r"(\d+)\+?\s+years?", lowered)
    responsibilities = []
    for line in job_description.splitlines():
        item = line.strip().lstrip("-*\u2022").strip()
        if not item:
            continue
        if len(item.split()) < 4:
            continue
        if any(token in item.lower() for token in ("build", "design", "lead", "develop", "manage", "own", "ship")):
            responsibilities.append(item)
        if len(responsibilities) == 6:
            break

    return {
        "required_skills": sorted(set(required_skills)),
        "preferred_skills": sorted(set(preferred_skills)),
        "responsibilities": responsibilities,
        "min_years_experience": max([int(y) for y in years], default=0),
    }


@tool("compute_match_score")
def compute_match_score(resume_keywords: list[str], jd_requirements: dict[str, Any]) -> dict[str, float]:
    """Compute coverage scores for required and preferred job skills."""
    resume_set = set(k.lower() for k in resume_keywords)
    required = set(k.lower() for k in jd_requirements.get("required_skills", []))
    preferred = set(k.lower() for k in jd_requirements.get("preferred_skills", []))

    required_coverage = (len(required & resume_set) / len(required) * 100.0) if required else 0.0
    preferred_coverage = (len(preferred & resume_set) / len(preferred) * 100.0) if preferred else 0.0

    overall = (required_coverage * 0.7) + (preferred_coverage * 0.3)
    return {
        "required_coverage": round(required_coverage, 2),
        "preferred_coverage": round(preferred_coverage, 2),
        "overall_match": round(overall, 2),
    }


@tool("identify_skill_gaps")
def identify_skill_gaps(resume_keywords: list[str], jd_requirements: dict[str, Any]) -> dict[str, list[str]]:
    """Return matched and missing skills between resume and job requirements."""
    resume_set = set(k.lower() for k in resume_keywords)
    required = set(k.lower() for k in jd_requirements.get("required_skills", []))
    preferred = set(k.lower() for k in jd_requirements.get("preferred_skills", []))
    target = required | preferred

    matched = sorted(target & resume_set)
    missing = sorted(target - resume_set)
    return {"matched": matched, "missing": missing}
