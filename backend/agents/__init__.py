"""LangGraph resume matching agents package."""


def build_resume_match_graph():
    from .graph import build_resume_match_graph as _builder

    return _builder()


def run_resume_match(resume_text: str, job_description: str):
    from .graph import run_resume_match as _runner

    return _runner(resume_text=resume_text, job_description=job_description)


__all__ = ["build_resume_match_graph", "run_resume_match"]
