from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path


def _read_text(path: str) -> str:
    return Path(path).expanduser().read_text(encoding="utf-8")


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Run LangGraph resume checker and job-match agents."
    )
    parser.add_argument("--resume-file", required=True, help="Path to resume text/tex file")
    parser.add_argument("--jd-file", required=True, help="Path to job description text file")
    parser.add_argument(
        "--json",
        action="store_true",
        help="Print full graph state as JSON instead of final report.",
    )
    args = parser.parse_args()

    resume_text = _read_text(args.resume_file)
    job_description = _read_text(args.jd_file)
    try:
        from .graph import run_resume_match
    except ModuleNotFoundError as exc:
        print(
            "Missing dependency: "
            f"{exc.name}. Install agent dependencies with "
            "`pip install -r backend/agents/requirements.txt`.",
            file=sys.stderr,
        )
        raise SystemExit(1) from exc

    state = run_resume_match(resume_text=resume_text, job_description=job_description)

    if args.json:
        print(json.dumps(state, indent=2))
        return

    print(state.get("final_report", "No report generated."))


if __name__ == "__main__":
    main()
