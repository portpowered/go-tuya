"""Check internal links from every rendered HTML page in a Pages site."""

from html.parser import HTMLParser
from pathlib import Path
import os
import re
import sys
from urllib.parse import unquote, urljoin, urlparse


class Links(HTMLParser):
    def __init__(self):
        super().__init__()
        self.hrefs = []

    def handle_starttag(self, tag, attrs):
        if tag == "a":
            self.hrefs.extend(value for key, value in attrs if key == "href" and value)


def main():
    if len(sys.argv) != 2:
        raise SystemExit("usage: check_site_links.py SITE_DIRECTORY")
    root = Path(sys.argv[1]).resolve()
    if not (root / "index.html").is_file():
        raise SystemExit(f"missing rendered site: {root / 'index.html'}")
    project = os.environ.get("GITHUB_REPOSITORY", "").split("/")[-1] or Path.cwd().name

    checked = 0
    broken = []
    external = set()
    pages = list(root.rglob("*.html"))
    # API renderers load externalDocs URLs from schemas at runtime; those links
    # need not appear as anchors in the generated HTML. Include description URLs
    # as well as externalDocs, without treating sentence punctuation as a path.
    schema_links = re.compile(r"https://portpowered\.github\.io/[^\s'\"<>\)\]]+")
    sources = [(page, page.relative_to(root).as_posix(), False) for page in pages]
    sources.extend((path, path.as_posix(), True) for path in Path("api").rglob("*.yaml"))
    for page, relative, schema in sources:
        route = "" if schema else relative[: -len("index.html")] if relative.endswith("index.html") else relative
        base = f"https://site.invalid/{project}/{route}"
        content = page.read_text(encoding="utf-8")
        if schema:
            hrefs = [url.rstrip(".,;:") for url in schema_links.findall(content)]
        else:
            parser = Links()
            parser.feed(content)
            hrefs = parser.hrefs
        for href in hrefs:
            resolved = urlparse(urljoin(base, href))
            if resolved.scheme not in ("http", "https"):
                continue
            if resolved.netloc not in ("site.invalid", "portpowered.github.io"):
                external.add(resolved.geturl())
                continue
            path = unquote(resolved.path).lstrip("/")
            if path == project:
                path = ""
            elif path.startswith(project + "/"):
                path = path[len(project) + 1 :]
            elif resolved.netloc == "portpowered.github.io":
                external.add(resolved.geturl())
                continue
            else:
                broken.append(f"{relative}: {href} (outside Pages project base)")
                continue
            target = root / path
            checked += 1
            if not target.is_file() and not (target / "index.html").is_file():
                broken.append(f"{relative}: {href}")

    if broken:
        raise SystemExit("broken internal site links:\n  " + "\n  ".join(broken))
    print(f"Checked {checked} internal links across {len(pages)} rendered pages")
    if external:
        print("External destinations for manual review:")
        for destination in sorted(external):
            print("  " + destination)


if __name__ == "__main__":
    main()
