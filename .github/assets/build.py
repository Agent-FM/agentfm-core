#!/usr/bin/env python3
"""Generate the pixel-art SVGs used by the root README.

Every glyph and sprite is drawn as <rect>s on a grid, so the SVGs need no
fonts, no CSS and no JS, and render the same on GitHub, in dark and light mode.

    python3 .github/assets/build.py      # writes .github/assets/*.svg
"""
from __future__ import annotations

import os

OUT = os.path.dirname(os.path.abspath(__file__))

# 5x7 bitmap font, one string of 7 rows per glyph. '#' is a lit pixel.
_GLYPHS = """
A .###. #...# #...# ##### #...# #...# #...#
B ####. #...# #...# ####. #...# #...# ####.
C .###. #...# #.... #.... #.... #...# .###.
D ####. #...# #...# #...# #...# #...# ####.
E ##### #.... #.... ####. #.... #.... #####
F ##### #.... #.... ####. #.... #.... #....
G .###. #...# #.... #.### #...# #...# .####
H #...# #...# #...# ##### #...# #...# #...#
I .###. ..#.. ..#.. ..#.. ..#.. ..#.. .###.
J ..### ...#. ...#. ...#. ...#. #..#. .##..
K #...# #..#. #.#.. ##... #.#.. #..#. #...#
L #.... #.... #.... #.... #.... #.... #####
M #...# ##.## #.#.# #.#.# #...# #...# #...#
N #...# ##..# #.#.# #..## #...# #...# #...#
O .###. #...# #...# #...# #...# #...# .###.
P ####. #...# #...# ####. #.... #.... #....
Q .###. #...# #...# #...# #.#.# #..#. .##.#
R ####. #...# #...# ####. #.#.. #..#. #...#
S .#### #.... #.... .###. ....# ....# ####.
T ##### ..#.. ..#.. ..#.. ..#.. ..#.. ..#..
U #...# #...# #...# #...# #...# #...# .###.
V #...# #...# #...# #...# #...# .#.#. ..#..
W #...# #...# #...# #.#.# #.#.# #.#.# .#.#.
X #...# #...# .#.#. ..#.. .#.#. #...# #...#
Y #...# #...# .#.#. ..#.. ..#.. ..#.. ..#..
Z ##### ....# ...#. ..#.. .#... #.... #####
0 .###. #...# #..## #.#.# ##..# #...# .###.
1 ..#.. .##.. ..#.. ..#.. ..#.. ..#.. .###.
2 .###. #...# ....# ...#. ..#.. .#... #####
3 ##### ...#. ..#.. ...#. ....# #...# .###.
4 ...#. ..##. .#.#. #..#. ##### ...#. ...#.
5 ##### #.... ####. ....# ....# #...# .###.
6 ..##. .#... #.... ####. #...# #...# .###.
7 ##### ....# ...#. ..#.. .#... .#... .#...
8 .###. #...# #...# .###. #...# #...# .###.
9 .###. #...# #...# .#### ....# ...#. .##..
. ..... ..... ..... ..... ..... ..... ..#..
, ..... ..... ..... ..... ..... ..#.. .#...
: ..... ..#.. ..#.. ..... ..#.. ..#.. .....
- ..... ..... ..... .###. ..... ..... .....
_ ..... ..... ..... ..... ..... ..... #####
/ ....# ....# ...#. ..#.. .#... #.... #....
> .#... ..#.. ...#. ....# ...#. ..#.. .#...
< ...#. ..#.. .#... #.... .#... ..#.. ...#.
! ..#.. ..#.. ..#.. ..#.. ..#.. ..... ..#..
? .###. #...# ....# ...#. ..#.. ..... ..#..
+ ..... ..#.. ..#.. ##### ..#.. ..#.. .....
$ ..#.. .#### #.... .###. ....# ####. ..#..
[ .###. .#... .#... .#... .#... .#... .###.
] .###. ...#. ...#. ...#. ...#. ...#. .###.
# .#.#. ##### .#.#. .#.#. .#.#. ##### .#.#.
' ..#.. ..#.. ..... ..... ..... ..... .....
= ..... ..... ##### ..... ##### ..... .....
* ..... .#.#. ..#.. ##### ..#.. .#.#. .....
| ..#.. ..#.. ..#.. ..#.. ..#.. ..#.. ..#..
( ...#. ..#.. .#... .#... .#... ..#.. ...#.
) .#... ..#.. ...#. ...#. ...#. ..#.. .#...
~ ..... ..... .#... #.#.# ...#. ..... .....
"""
FONT = {" ": "." * 35}
for _line in _GLYPHS.strip().splitlines():
    _ch, *_rows = _line.split(" ")
    assert len(_rows) == 7 and all(len(r) == 5 for r in _rows), _line
    FONT[_ch] = "".join(_rows)

# Hexagon node sprite, points left/right like the AgentFM logo.
HEX = [
    "...######...",
    "..########..",
    ".###....###.",
    "###......###",
    "###......###",
    ".###....###.",
    "..########..",
    "...######...",
]

THEMES = {
    "dark": dict(bg="#0b0d12", grid="#1a1e27", text="#f2f3f5", dim="#7d8590",
                 a1="#f57c1f", a2="#fbbf24", ok="#3fce8b", panel="#12151c", edge="#262b36"),
    "light": dict(bg="#f7f3ea", grid="#e7e0d0", text="#1b1d23", dim="#5d636e",
                  a1="#d9620b", a2="#c98a06", ok="#1f8f5a", panel="#efe9dc", edge="#d6cdb9"),
}


class Canvas:
    def __init__(self, w: int, h: int, title: str, desc: str):
        self.w, self.h = w, h
        self.title, self.desc = title, desc
        self.defs: list[str] = []
        self.body: list[str] = []

    def rect(self, x, y, w, h, fill, extra=""):
        self.body.append(f'<rect x="{x}" y="{y}" width="{w}" height="{h}" fill="{fill}"{extra}/>')

    def text(self, s: str, x: int, y: int, px: int, fill: str, extra: str = "") -> int:
        """Draw pixel text with its top-left at (x, y). Returns the width drawn."""
        cells = []
        cx = x
        for ch in s.upper():
            g = FONT.get(ch, FONT["?"])
            for i, c in enumerate(g):
                if c == "#":
                    cells.append(f"M{cx + (i % 5) * px} {y + (i // 5) * px}h{px}v{px}h-{px}z")
            cx += 6 * px
        self.body.append(f'<path fill="{fill}" d="{"".join(cells)}"{extra}/>')
        return cx - x - px

    @staticmethod
    def text_width(s: str, px: int) -> int:
        return len(s) * 6 * px - px

    def sprite(self, rows: list[str], x: int, y: int, px: int, fill: str, extra=""):
        d = "".join(
            f"M{x + c * px} {y + r * px}h{px}v{px}h-{px}z"
            for r, row in enumerate(rows)
            for c, ch in enumerate(row)
            if ch == "#"
        )
        self.body.append(f'<path fill="{fill}" d="{d}"{extra}/>')

    def svg(self) -> str:
        defs = f"<defs>{''.join(self.defs)}</defs>" if self.defs else ""
        return (
            f'<svg xmlns="http://www.w3.org/2000/svg" width="{self.w}" height="{self.h}" '
            f'viewBox="0 0 {self.w} {self.h}" shape-rendering="crispEdges" role="img" '
            f'aria-labelledby="t d"><title id="t">{self.title}</title><desc id="d">{self.desc}</desc>'
            f"{defs}{''.join(self.body)}</svg>\n"
        )


def line_cells(x0, y0, x1, y1):
    """Bresenham line between grid cells."""
    cells = []
    dx, dy = abs(x1 - x0), -abs(y1 - y0)
    sx, sy = (1 if x0 < x1 else -1), (1 if y0 < y1 else -1)
    err = dx + dy
    while True:
        cells.append((x0, y0))
        if x0 == x1 and y0 == y1:
            return cells
        e2 = 2 * err
        if e2 >= dy:
            err += dy
            x0 += sx
        if e2 <= dx:
            err += dx
            y0 += sy


def dot_grid(cv: Canvas, t: dict, step: int = 16):
    d = "".join(f"M{x} {y}h2v2h-2z" for x in range(step // 2, cv.w, step) for y in range(step // 2, cv.h, step))
    cv.body.append(f'<path fill="{t["grid"]}" d="{d}"/>')


def logo(cv: Canvas, t: dict, ox: int, oy: int, u: int, grad_id: str, packets: bool = True):
    """Pixel version of the AgentFM 'A' mesh: three outer nodes, one hub, links."""
    cv.defs.append(
        f'<linearGradient id="{grad_id}" x1="0" y1="0" x2="1" y2="1">'
        f'<stop offset="0" stop-color="{t["a1"]}"/><stop offset="1" stop-color="{t["a2"]}"/></linearGradient>'
    )
    fill = f"url(#{grad_id})"
    # node top-left corners in grid units (sprite is 12x8)
    top, left, right, hub = (14, 0), (0, 24), (28, 24), (14, 13)
    links = [((17, 8), (5, 24)), ((23, 8), (35, 24)), ((13, 17), (9, 24)), ((27, 17), (31, 24))]
    d = []
    paths = []
    for (a, b) in links:
        cells = line_cells(*a, *b)
        paths.append(cells)
        for (cx, cy) in cells:
            d.append(f"M{ox + cx * u} {oy + cy * u}h{2 * u}v{u}h-{2 * u}z")
    cv.body.append(f'<path fill="{fill}" opacity="0.85" d="{"".join(d)}"/>')
    for (nx, ny) in (top, left, right, hub):
        cv.sprite(HEX, ox + nx * u, oy + ny * u, u, fill)
    if packets:
        # LED-chaser "packets" running down the links, crisp because nothing moves sub-pixel.
        dur = 2.4
        for k, cells in enumerate(paths):
            n = len(cells)
            for i, (cx, cy) in enumerate(cells):
                begin = (i / n) * dur + k * 0.6
                cv.body.append(
                    f'<rect x="{ox + cx * u}" y="{oy + cy * u}" width="{2 * u}" height="{u}" fill="{t["text"]}" opacity="0">'
                    f'<animate attributeName="opacity" values="0;1;0;0" keyTimes="0;0.04;0.12;1" dur="{dur}s" '
                    f'begin="{begin:.2f}s" repeatCount="indefinite"/></rect>'
                )




def hero(theme: str):
    t = THEMES[theme]
    cv = Canvas(1280, 400, "AgentFM",
                "Pixel-art banner: the AgentFM mesh logo and the line Your agents. Your machines. One mesh.")
    cv.rect(0, 0, cv.w, cv.h, t["bg"])
    dot_grid(cv, t)
    logo(cv, t, 84, 96, 6, f"g-{theme}")
    x0 = 380
    w = cv.text("AGENT", x0, 104, 14, t["text"])
    cv.text("FM", x0 + w + 14, 104, 14, f"url(#g-{theme})")
    cv.text("YOUR AGENTS. YOUR MACHINES. ONE MESH.", x0, 230, 4, t["dim"])
    # terminal status strip
    cv.rect(0, 336, cv.w, 64, t["panel"])
    cv.rect(0, 336, cv.w, 4, t["edge"])
    w = cv.text("$ AGENTFM -MODE WORKER", 40, 358, 3, t["text"])
    cv.body.append(
        f'<rect x="{40 + w + 12}" y="358" width="15" height="21" fill="{t["a2"]}">'
        f'<animate attributeName="opacity" values="1;1;0;0" keyTimes="0;0.5;0.5;1" dur="1s" '
        f'repeatCount="indefinite" calcMode="discrete"/></rect>'
    )
    s = "P2P / E2E ENCRYPTED / OPENAI API"
    sw = cv.text_width(s, 3)
    cv.rect(cv.w - 40 - sw - 30, 364, 9, 9, t["ok"])
    cv.text(s, cv.w - 40 - sw, 358, 3, t["dim"])
    return cv.svg()


def chaser(cv: Canvas, pts: list[tuple[int, int]], size: int, fill: str, dur: float, offset: float = 0.0):
    """Squares that light up one after another along pts, like packets on a wire."""
    n = len(pts)
    for i, (x, y) in enumerate(pts):
        cv.body.append(
            f'<rect x="{x}" y="{y}" width="{size}" height="{size}" fill="{fill}" opacity="0">'
            f'<animate attributeName="opacity" values="0;1;0;0" keyTimes="0;0.03;0.1;1" dur="{dur}s" '
            f'begin="{offset + i / n * dur:.2f}s" repeatCount="indefinite"/></rect>'
        )


def polyline_steps(pts: list[tuple[int, int]], step: int) -> list[tuple[int, int]]:
    """Points every `step` px along an axis-aligned polyline."""
    out = []
    for (x0, y0), (x1, y1) in zip(pts, pts[1:]):
        length = abs(x1 - x0) + abs(y1 - y0)
        for d in range(0, length, step):
            out.append((x0 + (d if x1 > x0 else -d if x1 < x0 else 0),
                        y0 + (d if y1 > y0 else -d if y1 < y0 else 0)))
    out.append(pts[-1])
    return out


def dashed(cv: Canvas, pts, fill: str, th: int = 4, dash: int = 8, gap: int = 8):
    d = []
    for (x0, y0), (x1, y1) in zip(pts, pts[1:]):
        horiz = y0 == y1
        lo, hi = sorted((x0, x1) if horiz else (y0, y1))
        for p in range(lo, hi, dash + gap):
            ln = min(dash, hi - p)
            if horiz:
                d.append(f"M{p} {y0 - th // 2}h{ln}v{th}h-{ln}z")
            else:
                d.append(f"M{x0 - th // 2} {p}h{th}v{ln}h-{th}z")
    cv.body.append(f'<path fill="{fill}" d="{"".join(d)}"/>')


def arrow_down(cv: Canvas, x: int, y: int, fill: str, u: int = 4):
    """Pixel arrowhead whose tip is at (x, y)."""
    for r in range(4):
        cv.rect(x - (r + 1) * u, y - (4 - r) * u, (r + 1) * 2 * u, u, fill)


def blocks(cv: Canvas, ox: int, oy: int, u: int, items: list[tuple], colors: dict):
    """Draw a sprite given as (x, y, w, h, color-key) rects in grid units."""
    for (x, y, w, h, k) in items:
        cv.rect(ox + x * u, oy + y * u, w * u, h * u, colors[k])


LAPTOP = [  # 22 x 14
    (2, 0, 18, 12, "line"), (3, 1, 16, 10, "panel"),
    (5, 3, 6, 1, "a2"), (5, 5, 9, 1, "dim"), (5, 7, 4, 1, "dim"),
    (0, 12, 22, 2, "line"), (9, 12, 4, 1, "dim"),
]
TOWER = [  # 14 x 22
    (5, 4, 4, 3, "a2"), (6, 7, 2, 14, "line"),
    (4, 12, 6, 1, "line"), (3, 16, 8, 1, "line"), (0, 21, 14, 1, "line"),
]
SERVER = [  # 20 x 20
    (0, 0, 20, 20, "line"), (1, 1, 18, 18, "panel"),
]
for _i in range(3):
    SERVER += [(2, 2 + _i * 6, 16, 4, "edge"), (3, 3 + _i * 6, 8, 1, "dim"), (3, 4 + _i * 6, 5, 1, "dim")]


def diagram(theme: str):
    t = dict(THEMES[theme])
    t["line"] = t["text"] if theme == "light" else "#c9ced6"
    cv = Canvas(1280, 420, "How AgentFM works",
                "Three pixel-art machines. Left, a laptop labelled Boss or Desktop: you dispatch here. "
                "Middle, a relay tower: discovery and NAT traversal. Right, a server labelled Worker: runs your agent "
                "in a Podman sandbox. A task stream goes from the laptop to the worker, and output plus artifacts "
                "come back, peer to peer. Both machines find each other through the relay.")
    cv.rect(0, 0, cv.w, cv.h, t["bg"])
    dot_grid(cv, t)
    u = 8
    boss, tower, worker = (88, 190), (584, 126), (1032, 142)  # top-left corners
    blocks(cv, *boss, u, LAPTOP, t)
    blocks(cv, *tower, u, TOWER, t)
    # tower legs as pixel diagonals
    for (a, b) in (((6, 8), (1, 21)), ((7, 8), (12, 21))):
        for (cx, cy) in line_cells(*a, *b):
            cv.rect(tower[0] + cx * u, tower[1] + cy * u, u, u, t["line"])
    # beacon signal arcs, blinking
    for k, (dx, dy) in enumerate(((3, 4), (2, 3), (10, 4), (11, 3))):
        arc = [(0, 0), (0, 1), (0, 2)] if k % 2 == 0 else [(0, -1), (0, 0), (0, 1), (0, 2), (0, 3)]
        for (ax, ay) in arc:
            cv.body.append(
                f'<rect x="{tower[0] + (dx if dx < 7 else dx) * u}" y="{tower[1] + (dy + ay) * u}" width="{u}" height="{u}" fill="{t["a1"]}">'
                f'<animate attributeName="opacity" values="1;1;0.2;0.2" keyTimes="0;0.5;0.5;1" dur="1.6s" '
                f'begin="{0.4 * (k // 2) if k < 2 else 0.4 * (k - 2)}s" repeatCount="indefinite" calcMode="discrete"/></rect>'
            )
    blocks(cv, *worker, u, SERVER, t)
    for i in range(3):  # drive LEDs
        cv.body.append(
            f'<rect x="{worker[0] + 15 * u}" y="{worker[1] + (3 + i * 6) * u}" width="{u}" height="{u}" fill="{t["ok"]}">'
            f'<animate attributeName="opacity" values="1;0.2;1" dur="{0.7 + i * 0.3:.1f}s" repeatCount="indefinite" calcMode="discrete"/></rect>'
        )

    # task stream: boss -> worker, over the top
    bx, wx = boss[0] + 7 * u, worker[0] + 7 * u
    out_line = [(bx, 190), (bx, 64), (wx, 64), (wx, 126)]
    dashed(cv, out_line, t["a2"])
    arrow_down(cv, wx, 142, t["a2"])
    chaser(cv, polyline_steps(out_line, 16)[:-1], 8, t["text"], 3.0)
    # output back: worker -> boss
    bx2, wx2 = boss[0] + 15 * u, worker[0] + 15 * u
    back_line = [(wx2, 142), (wx2, 112), (bx2, 112), (bx2, 174)]
    dashed(cv, back_line, t["ok"])
    arrow_down(cv, bx2, 190, t["ok"])
    chaser(cv, polyline_steps(back_line, 16)[:-1], 8, t["text"], 3.0, 1.5)
    # labels on the lines, with a background box so the dashes don't cross the text
    for (label, cx, y, col) in (("TASK STREAM >", 640, 64, t["a2"]), ("< OUTPUT + ARTIFACTS", 840, 112, t["ok"])):
        w = cv.text_width(label, 3)
        cv.rect(cx - w // 2 - 12, y - 18, w + 24, 36, t["bg"])
        cv.text(label, cx - w // 2, y - 10, 3, col)
    # discovery via relay
    for (x0, x1) in ((boss[0] + 22 * u + 16, tower[0] - 8), (tower[0] + 14 * u + 8, worker[0] - 16)):
        dashed(cv, [(x0, 250), (x1, 250)], t["dim"], th=4, dash=4, gap=8)
    for cx in (boss[0] + 22 * u + (tower[0] - boss[0] - 22 * u) // 2, tower[0] + 14 * u + (worker[0] - tower[0] - 14 * u) // 2):
        w = cv.text_width("FIND PEERS", 3)
        cv.text("FIND PEERS", cx - w // 2, 268, 3, t["dim"])
    # captions
    for (name, sub, cx) in (("BOSS", "YOU DISPATCH", boss[0] + 11 * u),
                            ("RELAY", "DISCOVERY + NAT", tower[0] + 7 * u),
                            ("WORKER", "RUNS YOUR AGENT", worker[0] + 10 * u)):
        w = cv.text_width(name, 5)
        cv.text(name, cx - w // 2, 330, 5, t["text"])
        w = cv.text_width(sub, 3)
        cv.text(sub, cx - w // 2, 378, 3, t["dim"])
    return cv.svg()


def chamfer_box(cv: Canvas, x, y, w, h, border, fill, b=4):
    """A box with notched pixel corners."""
    cv.rect(x + b, y, w - 2 * b, h, border)
    cv.rect(x, y + b, w, h - 2 * b, border)
    cv.rect(x + b, y + b, w - 2 * b, h - 2 * b, fill)


def quest(theme: str):
    t = THEMES[theme]
    levels = [("LVL 1", "DISPATCH", "~1 MIN / APP"),
              ("LVL 2", "CALL THE API", "~1 MIN / CURL"),
              ("LVL 3", "RUN A WORKER", "~3 MIN / CLI")]
    cv = Canvas(1280, 220, "Five-minute quest",
                "Three levels: level 1, dispatch a task from the desktop app, about 1 minute. "
                "Level 2, call the mesh with curl or the OpenAI SDK, about 1 minute. "
                "Level 3, run your own worker from the CLI, about 3 minutes.")
    cv.rect(0, 0, cv.w, cv.h, t["bg"])
    dot_grid(cv, t)
    bw, gap, x = 360, 60, 40
    for i, (lvl, title, sub) in enumerate(levels):
        bx = x + i * (bw + gap)
        chamfer_box(cv, bx, 30, bw, 160, t["edge"], t["panel"])
        cv.rect(bx + 4, 30, bw - 8, 8, t["a2"] if i == 0 else t["a1"] if i == 1 else t["ok"])
        cv.text(lvl, bx + 28, 64, 4, t["a2"])
        # progress pips: how much of the quest you've done after this level
        for p in range(3):
            cv.rect(bx + bw - 28 - (3 - p) * 22, 66, 14, 14, t["a2"] if p <= i else t["edge"])
        cv.text(title, bx + 28, 108, 4, t["text"])
        cv.text(sub, bx + 28, 150, 3, t["dim"])
        if i < 2:  # connector arrow
            ax = bx + bw + 12
            for r in range(5):
                cv.rect(ax + r * 4, 102 + r * 4, 8, 4, t["dim"])
                cv.rect(ax + r * 4, 138 - r * 4, 8, 4, t["dim"])
            cv.rect(ax + 20, 118, 8, 4, t["dim"])
    return cv.svg()


def footer(theme: str):
    t = THEMES[theme]
    cv = Canvas(1280, 200, "Press star to continue",
                "Pixel footer: Press star to continue. Built with Go, libp2p and Podman.")
    cv.rect(0, 0, cv.w, cv.h, t["bg"])
    dot_grid(cv, t)
    msg = "PRESS   TO CONTINUE"
    w = cv.text_width(msg, 7)
    x = (cv.w - w) // 2
    cv.body.append('<g>')
    cv.text(msg, x, 50, 7, t["text"])
    cv.body.append('<animate attributeName="opacity" values="1;1;0;0" keyTimes="0;0.6;0.6;1" dur="1.4s" '
                   'repeatCount="indefinite" calcMode="discrete"/></g>')
    # the star sits in the gap: chars 6-8 of the message
    cv.sprite([r.replace("o", "#").replace("a", "#") for r in ICONS["star"]], x + 237, 46, 4, t["a2"])
    sub = "BUILT WITH GO, LIBP2P AND PODMAN"
    w = cv.text_width(sub, 3)
    cv.text(sub, (cv.w - w) // 2, 138, 3, t["dim"])
    return cv.svg()


# 16x16 heading icons. o = orange, a = amber, d = grey, w = highlight. Same art in both themes.
ICONS = {
    "play": [
        "................", "....oo..........", "....oaa.........", "....oaaa........",
        "....oaaaa.......", "....oaaaaa......", "....oaaaaaa.....", "....oaaaaaaa....",
        "....oaaaaaaa....", "....oaaaaaa.....", "....oaaaaa......", "....oaaaa.......",
        "....oaaa........", "....oaa.........", "....oo..........", "................",
    ],
    "bulb": [
        "................", ".....aaaaaa.....", "....aaaaaaaa....", "...aawaaaaaaa...",
        "...awaaaaaaaa...", "...aaaaaaaaaa...", "...aaaaaaaaaa...", "....aaaaaaaa....",
        ".....oaaaao.....", "......oaao......", ".....dddddd.....", "................",
        ".....dddddd.....", "......dddd......", "................", "................",
    ],
    "blocks": [
        "................", ".oooooo..aaaaaa.", ".owaaao..awaaaa.", ".oaaaao..aaaaaa.",
        ".oaaaao..aaaaaa.", ".oaaaao..aaaaaa.", ".oooooo..aaaaaa.", "................",
        "................", ".aaaaaa..oooooo.", ".awaaaa..owaaao.", ".aaaaaa..oaaaao.",
        ".aaaaaa..oaaaao.", ".aaaaaa..oaaaao.", ".aaaaaa..oooooo.", "................",
    ],
    "code": [
        "................", "................", "..........a.....", "...o......a.o...",
        "..oo.....a..oo..", ".oo......a...oo.", "oo......a.....oo", ".oo.....a....oo.",
        "..oo...a....oo..", "...o...a....o...", ".......a........", "................",
        "................", "................", "................", "................",
    ],
    "dpad": [
        "................", "......oooo......", "......oaao......", "......oaao......",
        "......oaao......", "......oaao......", ".ooooooaaoooooo.", ".oaaaaaaaaaaaao.",
        ".oaaaaaaaaaaaao.", ".ooooooaaoooooo.", "......oaao......", "......oaao......",
        "......oaao......", "......oaao......", "......oooo......", "................",
    ],
    "book": [
        "................", "................", ".oooooo..oooooo.", ".owwwwo..owwwwo.",
        ".owddwo..owddwo.", ".owwwwo..owwwwo.", ".owddwo..owddwo.", ".owwwwo..owwwwo.",
        ".owddwo..owddwo.", ".owwwwo..owwwwo.", ".oooooooooooooo.", "......oooo......",
        "................", "................", "................", "................",
    ],
    "monitor": [
        "................", ".dddddddddddddd.", ".daaaaaaaaaaaad.", ".daooooaaaaaaad.",
        ".daaaaaaaaaaaad.", ".daoooooooaaaad.", ".daaaaaaaaaaaad.", ".daoooaaaaaaaad.",
        ".daaaaaaaaaaaad.", ".dddddddddddddd.", "......dddd......", "....dddddddd....",
        "................", "................", "................", "................",
    ],
    "shield": [
        "................", "..oooooooooooo..", "..oaaaaaaaaaao..", "..oaaaaaaaaaao..",
        "..oaaaaaaaawao..", "..oaaaaaaawaao..", "..oawaaaawaaao..", "..oaawaawaaaao..",
        "...oaawwaaaao...", "...oaaaaaaaao...", "....oaaaaaao....", ".....oaaaao.....",
        "......oaao......", ".......oo.......", "................", "................",
    ],
    "star": [
        "................", ".......oo.......", ".......aa.......", "......oaao......",
        "......aaaa......", "oooooaaaaaaooooo", ".oaaaaaaaaaaaao.", "..oaaaaaaaaaao..",
        "...oaaaaaaaao...", "....aaaaaaaa....", "...oaaaaaaaao...", "...aaaaooaaaa...",
        "..oaaao..oaaao..", "..aao......oaa..", ".oo..........oo.", "................",
    ],
    "mesh": [
        "................", "......oooo......", "......oaao......", "......oooo......",
        ".......dd.......", "......d..d......", ".....d....d.....", "....d......d....",
        "...d........d...", ".oooo......oooo.", ".oaao......oaao.", ".oooo.dddd.oooo.",
        "................", "................", "................", "................",
    ],
}
ICON_COLORS = {"o": "#f57c1f", "a": "#fbbf24", "d": "#8b93a1", "w": "#fff4d6"}


def icon(name: str):
    rows = ICONS[name]
    assert len(rows) == 16 and all(len(r) == 16 for r in rows), name
    u = 2
    cv = Canvas(32, 32, "", "")
    for key, col in ICON_COLORS.items():
        cv.sprite(["".join("#" if c == key else "." for c in r) for r in rows], 0, 0, u, col)
    # decorative: the heading text next to it says what the section is
    return cv.svg().replace(' role="img" aria-labelledby="t d"><title id="t"></title><desc id="d"></desc>', ' aria-hidden="true">')


def main():
    for theme in THEMES:
        for name, fn in (("hero", hero), ("how-it-works", diagram), ("quest", quest), ("footer", footer)):
            with open(os.path.join(OUT, f"{name}-{theme}.svg"), "w") as f:
                f.write(fn(theme))
    os.makedirs(os.path.join(OUT, "icons"), exist_ok=True)
    for name in ICONS:
        with open(os.path.join(OUT, "icons", f"{name}.svg"), "w") as f:
            f.write(icon(name))


if __name__ == "__main__":
    main()
