#!/usr/bin/env python3

import argparse
import html
import subprocess
from pathlib import Path

import markdown


CHROME_BIN = Path("/Applications/Google Chrome.app/Contents/MacOS/Google Chrome")

CSS = """
body {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
  max-width: 920px;
  margin: 40px auto;
  padding: 0 24px 40px;
  color: #111827;
  line-height: 1.55;
  font-size: 14px;
}
h1, h2, h3, h4 {
  color: #111827;
  line-height: 1.25;
  margin-top: 1.4em;
  margin-bottom: 0.5em;
}
h1 { font-size: 28px; border-bottom: 1px solid #e5e7eb; padding-bottom: 8px; }
h2 { font-size: 22px; border-bottom: 1px solid #f1f5f9; padding-bottom: 6px; }
h3 { font-size: 18px; }
h4 { font-size: 16px; }
p, li { color: #1f2937; }
code {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  background: #f3f4f6;
  padding: 0.1em 0.3em;
  border-radius: 4px;
}
pre {
  background: #0f172a;
  color: #e2e8f0;
  padding: 14px;
  border-radius: 8px;
  overflow-x: auto;
}
pre code { background: transparent; color: inherit; padding: 0; }
table {
  border-collapse: collapse;
  width: 100%;
  margin: 16px 0;
  font-size: 13px;
}
th, td {
  border: 1px solid #d1d5db;
  padding: 8px 10px;
  text-align: left;
  vertical-align: top;
}
th { background: #f8fafc; }
img {
  max-width: 100%;
  height: auto;
  display: block;
  margin: 16px auto;
  border: 1px solid #e5e7eb;
}
blockquote {
  border-left: 4px solid #cbd5e1;
  margin: 16px 0;
  padding: 4px 0 4px 16px;
  color: #475569;
}
@page {
  size: Letter;
  margin: 0.55in;
}
"""


def render_markdown(md_path: Path) -> str:
    source = md_path.read_text(encoding="utf-8")
    body = markdown.markdown(
        source,
        extensions=[
            "extra",
            "tables",
            "fenced_code",
            "toc",
            "sane_lists",
        ],
        output_format="html5",
    )
    title = html.escape(md_path.stem.replace("_", " "))
    return f"""<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{title}</title>
  <style>{CSS}</style>
</head>
<body>
{body}
</body>
</html>
"""


def convert(md_path: Path, pdf_path: Path) -> None:
    if not CHROME_BIN.exists():
        raise FileNotFoundError(f"Chrome binary not found at {CHROME_BIN}")

    html_path = pdf_path.with_suffix(".html")
    html_path.write_text(render_markdown(md_path), encoding="utf-8")

    cmd = [
        str(CHROME_BIN),
        "--headless=new",
        "--disable-gpu",
        "--allow-file-access-from-files",
        f"--print-to-pdf={pdf_path}",
        str(html_path.resolve()),
    ]
    subprocess.run(cmd, check=True)


def main() -> int:
    parser = argparse.ArgumentParser(description="Convert Markdown files to PDF using headless Chrome.")
    parser.add_argument("paths", nargs="+", help="Markdown file paths to convert")
    args = parser.parse_args()

    for raw_path in args.paths:
        md_path = Path(raw_path).resolve()
        if md_path.suffix.lower() != ".md":
            raise ValueError(f"Expected a .md file, got {md_path}")
        pdf_path = md_path.with_suffix(".pdf")
        convert(md_path, pdf_path)
        print(f"wrote {pdf_path}")

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
