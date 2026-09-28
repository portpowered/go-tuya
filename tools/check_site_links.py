"""Check that links from rendered guide pages resolve within the Pages site."""

from html.parser import HTMLParser
from pathlib import Path
import sys
from urllib.parse import urljoin, urlparse, unquote


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
    guides = root / "docs" / "guides"
    if not guides.is_dir():
        raise SystemExit(f"missing rendered guides: {guides}")

    checked = 0
    broken = []
    for page in guides.rglob("index.html"):
        route = "/" + page.parent.relative_to(root).as_posix() + "/"
        parser = Links()
        parser.feed(page.read_text(encoding="utf-8"))
        for href in parser.hrefs:
            resolved = urlparse(urljoin("https://site.invalid" + route, href))
            if resolved.netloc != "site.invalid":
                continue
            path = unquote(resolved.path)
            marker = path.find("/docs/")
            if marker < 0:
                continue
            site_path = path[marker + 1 :].strip("/")
            target = root / site_path
            checked += 1
            if not target.is_file() and not (target / "index.html").is_file():
                broken.append(f"{page.relative_to(root)}: {href}")

    if broken:
        raise SystemExit("broken internal guide links:\n  " + "\n  ".join(broken))
    print(f"Checked {checked} internal links from rendered guides")


if __name__ == "__main__":
    main()
