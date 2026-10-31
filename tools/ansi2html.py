#!/usr/bin/env python3
"""Render an ANSI terminal capture to a styled HTML page (for README screenshots)."""
import html
import re
import sys

PALETTE = {
    0: "#0d1117", 1: "#ff7b72", 2: "#3fb950", 3: "#d29922", 4: "#58a6ff",
    5: "#bc8cff", 6: "#39c5cf", 7: "#b1bac4", 8: "#6e7681", 9: "#ffa198",
    10: "#56d364", 11: "#e3b341", 12: "#79c0ff", 13: "#d2a8ff", 14: "#56d4dd",
    15: "#f0f6fc",
}


def xterm(n):
    if n < 16:
        return PALETTE[n]
    if n < 232:
        n -= 16
        r, g, b = n // 36, (n % 36) // 6, n % 6
        f = lambda v: 0 if v == 0 else 55 + v * 40
        return "#%02x%02x%02x" % (f(r), f(g), f(b))
    v = 8 + (n - 232) * 10
    return "#%02x%02x%02x" % (v, v, v)


SGR = re.compile(r"\x1b\[([0-9;]*)m")


def segments(data):
    """Split raw ANSI text into (text, css-tuple) runs."""
    out, style, pos = [], (), 0
    for m in SGR.finditer(data):
        if m.start() > pos:
            out.append((data[pos:m.start()], style))
        pos = m.end()
        params = [p for p in m.group(1).split(";") if p != ""] or ["0"]
        cur = list(style)
        i = 0
        while i < len(params):
            p = params[i]
            if p == "0":
                cur = []
            elif p == "1":
                cur.append("font-weight:700")
            elif p == "2":
                cur.append("opacity:.7")
            elif p == "3":
                cur.append("font-style:italic")
            elif p == "22":
                cur = [c for c in cur if not c.startswith("font-weight")]
            elif p == "38" and i + 2 < len(params) and params[i + 1] == "5":
                cur = [c for c in cur if not c.startswith("color")]
                cur.append("color:" + xterm(int(params[i + 2])))
                i += 2
            elif p == "38" and i + 4 < len(params) and params[i + 1] == "2":
                r, g, b = (int(x) for x in params[i + 2:i + 5])
                cur = [c for c in cur if not c.startswith("color")]
                cur.append("color:#%02x%02x%02x" % (r, g, b))
                i += 4
            i += 1
        style = tuple(cur)
    if pos < len(data):
        out.append((data[pos:], style))
    return out


def to_html(data):
    """Flatten runs into per-line spans so styling never bleeds across lines."""
    lines, cur = [], []
    for text, style in segments(data):
        parts = text.split("\n")
        for i, part in enumerate(parts):
            if part:
                cur.append((part, style))
            if i < len(parts) - 1:
                lines.append(cur)
                cur = []
    lines.append(cur)

    rendered = []
    for line in lines:
        buf = []
        for text, style in line:
            esc = html.escape(text)
            buf.append('<span style="%s">%s</span>' % (";".join(style), esc) if style else esc)
        rendered.append("".join(buf))
    return "\n".join(rendered)


def main():
    src, dst = sys.argv[1], sys.argv[2]
    with open(src, "r", encoding="utf-8") as f:
        data = f.read()
    body = to_html(data)
    page = """<!doctype html>
<meta charset="utf-8">
<title>morning</title>
<style>
  html,body{{margin:0;background:#0d1117}}
  body{{padding:28px 32px;display:inline-block}}
  pre{{
    margin:0; font:14px/1.6 "JetBrains Mono","Cascadia Mono","SF Mono",Consolas,monospace;
    color:#c9d1d9; white-space:pre; letter-spacing:.1px;
  }}
</style>
<pre>{body}</pre>
""".format(body=body)
    with open(dst, "w", encoding="utf-8") as f:
        f.write(page)


if __name__ == "__main__":
    main()
