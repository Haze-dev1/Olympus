#!/usr/bin/env python3
"""Check relative Markdown links and heading anchors in Olympus docs."""

from __future__ import annotations

import html
import re
import sys
import unicodedata
from pathlib import Path
from urllib.parse import unquote, urlsplit

ROOT = Path(__file__).resolve().parent.parent
LINK = re.compile(r"!?\[[^\]]*\]\(([^)\s]+)(?:\s+\"[^\"]*\")?\)")
FENCE = re.compile(r"^\s*(```|~~~)")
HEADING = re.compile(r"^#{1,6}\s+(.*?)\s*#*\s*$")
INLINE_CODE = re.compile(r"`+[^`]*`+")


def markdown_files() -> list[Path]:
    files = [p for base in ("docs", "examples", "recipes") for p in (ROOT / base).rglob("*.md")]
    files.extend(ROOT.glob("*.md"))
    return sorted(files)


def slug(text: str) -> str:
    """Return the GitHub heading anchor for a heading's source text."""
    text = re.sub(r"!?\[([^\]]*)\]\([^)]*\)", r"\1", text)
    kept = "".join(c for c in html.unescape(text).lower() if c in "-_ " or c.isalnum())
    return "-".join(kept.split())


def anchors(text: str) -> set[str]:
    """Return all heading anchors in a Markdown file, including duplicate suffixes."""
    found: set[str] = set()
    seen: dict[str, int] = {}
    fence = ""
    for line in text.splitlines():
        match = FENCE.match(line)
        if match:
            marker = match.group(1)
            fence = "" if fence == marker else marker if not fence else fence
            continue
        if fence:
            continue
        match = HEADING.match(line)
        if not match:
            continue
        base = slug(match.group(1))
        count = seen.get(base, 0)
        seen[base] = count + 1
        found.add(base if count == 0 else f"{base}-{count}")
    return found


def check() -> list[str]:
    errors: list[str] = []
    cache: dict[Path, set[str]] = {}

    def headings(path: Path) -> set[str]:
        if path not in cache:
            cache[path] = anchors(path.read_text(encoding="utf-8"))
        return cache[path]

    for source in markdown_files():
        fence = ""
        for number, line in enumerate(source.read_text(encoding="utf-8").splitlines(), 1):
            match = FENCE.match(line)
            if match:
                marker = match.group(1)
                fence = "" if fence == marker else marker if not fence else fence
                continue
            if fence:
                continue
            for target in LINK.findall(INLINE_CODE.sub("", line)):
                parts = urlsplit(target)
                if parts.scheme or target.startswith("//"):
                    continue
                path, fragment = unquote(parts.path), parts.fragment
                resolved = (source.parent / path).resolve() if path else source
                if not resolved.exists():
                    errors.append(f"{source.relative_to(ROOT)}:{number}: missing target {target}")
                    continue
                if not fragment or fragment.startswith("L"):
                    continue
                if resolved.is_dir():
                    resolved = resolved / "README.md"
                if resolved.suffix != ".md" or not resolved.exists():
                    continue
                if unquote(fragment) not in headings(resolved):
                    errors.append(
                        f"{source.relative_to(ROOT)}:{number}: missing anchor #{fragment} in {path}"
                    )
    return errors


def main() -> int:
    errors = check()
    if errors:
        print(f"{len(errors)} broken Markdown link(s):", file=sys.stderr)
        for error in errors:
            print(error, file=sys.stderr)
        return 1
    print(f"Checked Markdown links in {len(markdown_files())} files: all resolve")
    return 0


if __name__ == "__main__":
    sys.exit(main())
