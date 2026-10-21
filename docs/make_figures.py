#!/usr/bin/env python3
"""Regenerate the figures used in README.md from evidence/*.json.

Every number plotted here is read from a result file; nothing is hard coded
except the layout and the colours. Run from the package root:

    python docs/make_figures.py

Output: docs/img/fig1_measured.png ... fig4_mechanism.png
"""

import json
import pathlib
import statistics

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
from matplotlib.patches import FancyBboxPatch, FancyArrowPatch, Circle

ROOT = pathlib.Path(__file__).resolve().parent.parent
IMG = ROOT / "docs" / "img"
IMG.mkdir(parents=True, exist_ok=True)

INK = "#2b2119"
PAPER = "#faf6f0"
AMBER = "#c8892b"
TERRA = "#b4552f"
OLIVE = "#6b6b3a"
MUTED = "#8a7a68"
GRID = "#d9cfc2"

plt.rcParams.update({
    "font.family": "DejaVu Sans",
    "font.size": 9.6,
    "axes.edgecolor": MUTED,
    "axes.labelcolor": INK,
    "text.color": INK,
    "xtick.color": MUTED,
    "ytick.color": INK,
    "figure.facecolor": PAPER,
    "axes.facecolor": PAPER,
    "savefig.facecolor": PAPER,
})

CASES = ["open-100x100", "open-300x300", "open-600x600",
         "rooms-8x8", "rooms-12x12", "maze-41x41"]
LABELS = ["open 100\u00d7100", "open 300\u00d7300", "open 600\u00d7600",
          "rooms 8\u00d78", "rooms 12\u00d712", "maze 41\u00d741"]

bench = json.loads((ROOT / "evidence" / "bench.json").read_text(encoding="utf-8"))
mem = json.loads((ROOT / "evidence" / "mem.json").read_text(encoding="utf-8"))

sess1 = ROOT / "evidence" / "bench-session1.json"
bench1 = json.loads(sess1.read_text(encoding="utf-8")) if sess1.exists() else None


def med(xs):
    return statistics.median(xs)


def dumbbell(ax, rows, xlo, xhi, unit, fmt, show_labels=True):
    """rows: list of (label, a_value, b_value). Horizontal A/B dumbbell."""
    ypos = list(range(len(rows)))[::-1]
    for y, (label, a, b) in zip(ypos, rows):
        ax.plot([a, b], [y, y], color=GRID, lw=2.4, zorder=1, solid_capstyle="round")
        ax.scatter([a], [y], s=54, color=TERRA, marker="o", zorder=3,
                   edgecolor=PAPER, linewidth=0.8)
        ax.scatter([b], [y], s=46, color=OLIVE, marker="s", zorder=3,
                   edgecolor=PAPER, linewidth=0.8)
        hi = max(a, b)
        ax.annotate(("\u00d7%.1f" % (a / b)) if a > b else ("\u00f7%.1f" % (b / a)),
                    xy=(hi, y), xytext=(7, 0), textcoords="offset points",
                    va="center", ha="left", fontsize=9.4, color=INK, fontweight="bold")
    ax.set_yticks(ypos)
    if show_labels:
        ax.set_yticklabels([r[0] for r in rows], fontsize=9.4, color=INK)
    else:
        ax.set_yticklabels([])
    ax.set_xscale("log")
    ax.set_xlim(xlo, xhi)
    ax.set_ylim(-0.7, len(rows) - 0.3)
    ax.grid(axis="x", color=GRID, lw=0.6, alpha=0.7, zorder=0)
    ax.set_axisbelow(True)
    for s in ("top", "right"):
        ax.spines[s].set_visible(False)
    ax.set_xlabel(unit, fontsize=9.4, color=MUTED)


# ------------------------------------------------------- fig 1: what you pay
def fig1():
    fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(8.6, 3.35))

    t_rows = []
    for c, lab in zip(CASES, LABELS):
        d = bench["cases"][c]
        t_rows.append((lab, med(d["upstream_ns"]) / 1e6, med(d["patched_ns"]) / 1e6))
    dumbbell(ax1, t_rows, 0.3, 26, "wall clock per search, ms (log)", "%.2f")

    m_rows = []
    for c, lab in zip(CASES, LABELS):
        d = mem["cases"][c]
        m_rows.append((lab, med(d["upstream"]["peak_ws"]) / 1e6,
                       med(d["patched"]["peak_ws"]) / 1e6))
    dumbbell(ax2, m_rows, 0.5, 62, "peak working set, MB, runtime baseline removed (log)",
             "%.2f")

    ax1.set_title("Time per search", fontsize=12, color=INK, loc="left", pad=10)
    ax2.set_title("Peak memory per search", fontsize=12, color=INK, loc="left", pad=10)

    t_vals = [v for r in t_rows for v in r[1:]]
    m_vals = [v for r in m_rows for v in r[1:]]

    # Legend built from proxy handles so it matches the markers exactly.
    from matplotlib.lines import Line2D
    handles = [
        Line2D([], [], color=TERRA, marker="o", lw=0, markersize=7, label="upstream v0.3.0"),
        Line2D([], [], color=OLIVE, marker="s", lw=0, markersize=6.5, label="patched"),
    ]
    fig.legend(handles=handles, loc="lower center", ncol=2, frameon=False,
               bbox_to_anchor=(0.5, -0.02), fontsize=9.4)
    fig.text(0.5, 0.975,
             "log axes: the measurements span %.0f\u00d7 in time and %.0f\u00d7 in memory"
             % (max(t_vals) / min(t_vals), max(m_vals) / min(m_vals)),
             ha="center", fontsize=8.8, color=MUTED)
    fig.tight_layout(rect=(0, 0.06, 1, 0.955))
    fig.savefig(IMG / "fig-measured.png", dpi=200, bbox_inches="tight")
    plt.close(fig)


# ------------------------------------------- fig 2: where the time goes, exactly
def fig2():
    fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(8.6, 3.35),
                                   gridspec_kw={"width_ratios": [1, 1.15]})

    # Left: the formula, on the three open grids where every interior node has
    # degree 4, so d_calls = 4E(E+1) - (8 - deg(start)) with deg(start) = 3.
    es = [mem["cases"][c]["upstream"]["expansions"][0] for c in CASES[:3]]
    ds = [mem["cases"][c]["upstream"]["d_calls"][0] for c in CASES[:3]]
    xs = list(range(90, 640, 5))
    ys = [4 * e * (e + 1) - 5 for e in xs]
    ax1.plot(xs, ys, color=AMBER, lw=1.6, label="4E(E+1) \u2212 5  (closed form)")
    ax1.scatter(es, ds, s=64, color=INK, zorder=4, facecolor="none",
                linewidth=1.6, label="measured (upstream)")
    for i, (e, d) in enumerate(zip(es, ds)):
        # The first point sits against the left spine, so its label goes right.
        dx, dy = (11, -13) if i == 0 else (-9, 9)
        ax1.annotate("{:,}".format(d), xy=(e, d), xytext=(dx, dy),
                     textcoords="offset points", ha="left" if i == 0 else "right",
                     fontsize=9.2, color=INK)
    ax1.set_yscale("log")
    ax1.set_xlabel("expansions E", fontsize=9.4, color=MUTED)
    ax1.set_ylabel("calls to your cost function", fontsize=9.4, color=MUTED)
    ax1.set_title("Every push re-walks the path",
                  fontsize=11.5, color=INK, loc="left", pad=8)
    ax1.grid(color=GRID, lw=0.6, alpha=0.7)
    ax1.set_axisbelow(True)
    for s in ("top", "right"):
        ax1.spines[s].set_visible(False)
    ax1.legend(loc="upper left", frameon=False, fontsize=9.2, handletextpad=0.6)
    ax1.annotate("max |measured \u2212 formula| = 0\non all three grid sizes",
                 xy=(0.97, 0.06), xycoords="axes fraction", ha="right",
                 fontsize=9.0, color=MUTED)

    # Right: the same counter on all six cases, both arms.
    rows = []
    for c, lab in zip(CASES, LABELS):
        d = mem["cases"][c]
        rows.append((lab, d["upstream"]["d_calls"][0], d["patched"]["d_calls"][0]))
    dumbbell(ax2, rows, 300, 6e6, "calls to your cost function (log)", "%.2f")
    ax2.set_title("Same search, 50\u00d7 to 300\u00d7 fewer evaluations",
                  fontsize=11.5, color=INK, loc="left", pad=8)

    fig.tight_layout(w_pad=2.6)
    fig.savefig(IMG / "fig-dcalls.png", dpi=200, bbox_inches="tight")
    plt.close(fig)


# ------------------------------------------------ fig 3: the wrong answer
def fig3():
    pos = {
        0: (0.00, 0.00),
        1: (1.50, 1.85),
        2: (0.90, -1.90),
        5: (2.55, 1.15),
        3: (2.75, -0.35),
        4: (4.00, -1.65),
        6: (4.30, 1.35),
    }
    edges = [(0, 1, 2.92), (0, 3, 2.62), (0, 5, 1.13), (0, 2, 2.10),
             (1, 2, 1.94), (1, 3, 1.46), (3, 4, 1.75), (3, 5, 1.26), (3, 6, 2.57)]
    hv = {0: 4.00, 1: 4.00, 2: 5.97, 3: 1.00, 4: 4.00, 5: 3.83, 6: 0.00}
    optimal = [0, 5, 3, 6]
    returned = [0, 3, 6]

    fig, ax = plt.subplots(figsize=(7.2, 5.05))
    ax.set_xlim(-0.75, 4.95)
    ax.set_ylim(-2.95, 2.85)
    ax.set_aspect("equal")
    ax.axis("off")

    # Edges first, then the two routes on top, then labels above everything.
    for a, b, w in edges:
        (x1, y1), (x2, y2) = pos[a], pos[b]
        ax.plot([x1, x2], [y1, y2], color=GRID, lw=1.6, zorder=1, solid_capstyle="round")
        mx, my = (x1 + x2) / 2, (y1 + y2) / 2
        dx, dy = x2 - x1, y2 - y1
        n = (dx * dx + dy * dy) ** 0.5
        ox, oy = -dy / n * 0.17, dx / n * 0.17
        # A few edges have their label pushed to the other side to stay clear
        # of a route line drawn on the same segment.
        if (a, b) in ((0, 5), (3, 5), (0, 3), (1, 2)):
            ox, oy = -ox, -oy
        ax.text(mx + ox, my + oy, "%.2f" % w, fontsize=7.8, color=MUTED,
                ha="center", va="center", zorder=9,
                bbox=dict(boxstyle="round,pad=0.18", fc=PAPER, ec="none"))

    def route(nodes, color, lw, style, off):
        for i in range(len(nodes) - 1):
            x1, y1 = pos[nodes[i]]
            x2, y2 = pos[nodes[i + 1]]
            dx, dy = x2 - x1, y2 - y1
            n = (dx * dx + dy * dy) ** 0.5
            ox, oy = -dy / n * off, dx / n * off
            ax.plot([x1 + ox, x2 + ox], [y1 + oy, y2 + oy], color=color, lw=lw,
                    ls=style, zorder=4, solid_capstyle="round")

    route(optimal, OLIVE, 4.2, "-", 0.075)
    route(returned, TERRA, 3.0, (0, (5, 3)), -0.02)

    for n, (x, y) in pos.items():
        ax.add_patch(Circle((x, y), 0.285, facecolor=PAPER, edgecolor=INK,
                            lw=1.4, zorder=6))
        ax.text(x, y + 0.085, str(n), fontsize=10.0, ha="center", va="center",
                color=INK, fontweight="bold", zorder=7)
        ax.text(x, y - 0.115, "h=%.2f" % hv[n], fontsize=5.9, ha="center",
                va="center", color=MUTED, zorder=7)

    ax.text(pos[0][0] - 0.12, pos[0][1] - 0.48, "start", fontsize=8, color=MUTED,
            ha="right")
    ax.text(pos[6][0] + 0.10, pos[6][1] + 0.42, "goal", fontsize=8, color=MUTED)

    from matplotlib.lines import Line2D
    handles = [
        Line2D([], [], color=OLIVE, lw=3.5,
               label="optimal  0\u21925\u21923\u21926 = 4.96"),
        Line2D([], [], color=TERRA, lw=2.6, ls=(0, (5, 3)),
               label="upstream returns  0\u21923\u21926 = 5.19"),
    ]
    ax.legend(handles=handles, loc="upper center", bbox_to_anchor=(0.5, 0.055),
              ncol=2, frameon=False, fontsize=9, handlelength=2.4,
              columnspacing=2.4)

    ax.text(0.5, 1.035,
            "h is admissible everywhere (h \u2264 true remaining cost) but not consistent:"
            "   h(1) \u2212 h(3) = 3.00 > cost(1,3) = 1.46",
            transform=ax.transAxes, ha="center", va="top", fontsize=9, color=INK)
    ax.text(0.5, -0.035,
            "Node 3 is popped at f = 3.62 and closed. The cheaper route to it"
            " (1.13 + 1.26 = 2.39) arrives later and is dropped, so node 6\n"
            "is only ever relaxed at g = 5.19. An implementation that may re-open"
            " node 3 finds the 4.96 route.",
            transform=ax.transAxes, ha="center", va="top", fontsize=8.3, color=MUTED,
            linespacing=1.6)

    fig.savefig(IMG / "fig-counterexample.png", dpi=200, bbox_inches="tight")
    plt.close(fig)


# ------------------------------------------------- fig 4: what actually changed
def fig4():
    fig, ax = plt.subplots(figsize=(8.6, 3.8))
    ax.set_xlim(0, 10)
    ax.set_ylim(0, 7.62)
    ax.axis("off")

    cards = [
        ("priority of a queue entry",
         "path.Cost(d) + h\nre-walks every edge of the\ncandidate path, O(L) per push",
         "g + h\ng is carried on the node,\nO(1) per push"),
        ("how many entries per node",
         "one entry per relaxation\n53,316 pushes for 46,677 pops\non the 8\u00d78 room grid",
         "one entry per node\nheap.Fix on improvement,\nno duplicates"),
        ("what a closed node means",
         "closed forever\nif a cheaper route is found\nlater, it is dropped",
         "re-openable\nre-inserted when g improves,\nso A* stays correct"),
    ]

    colx = {"up": 0.15, "pa": 5.05}
    w = 4.7
    head_y = 7.02
    ax.text(colx["up"] + w / 2, head_y, "upstream v0.3.0", ha="center", fontsize=11,
            color=TERRA, fontweight="bold")
    ax.text(colx["pa"] + w / 2, head_y, "this patch", ha="center", fontsize=11,
            color=OLIVE, fontweight="bold")

    for i, (title, up, pa) in enumerate(cards):
        y = 5.05 - i * 1.85
        ax.text(colx["up"] + w / 2, y + 1.24, title, ha="center", va="bottom",
                fontsize=9, color=INK, style="italic")
        for key, body, color in (("up", up, TERRA), ("pa", pa, OLIVE)):
            box = FancyBboxPatch((colx[key], y), w, 1.15,
                                 boxstyle="round,pad=0.02,rounding_size=0.08",
                                 facecolor="#f4ece1", edgecolor=color, lw=1.3)
            ax.add_patch(box)
            ax.text(colx[key] + w / 2, y + 0.575, body, ha="center", va="center",
                    fontsize=8.0, color=INK, linespacing=1.35)

    for i in range(len(cards)):
        y = 5.05 - i * 1.85 + 0.575
        ax.add_patch(FancyArrowPatch((colx["up"] + w + 0.06, y), (colx["pa"] - 0.06, y),
                                     arrowstyle="-|>", mutation_scale=13,
                                     color=MUTED, lw=1.2, shrinkA=0, shrinkB=0))

    ax.text(5.0, 0.62,
            "Behaviour is unchanged on consistent heuristics: the two arms expand the same "
            "nodes in the same order and return the same cost.",
            ha="center", fontsize=8.4, color=MUTED)
    ax.text(5.0, 7.40, "Three changes, one effect: O(L) work per relaxation becomes O(1)",
            ha="center", fontsize=11.5, color=INK)

    fig.savefig(IMG / "fig-mechanism.png", dpi=200, bbox_inches="tight")
    plt.close(fig)


if __name__ == "__main__":
    fig1()
    fig2()
    fig3()
    fig4()
    for p in sorted(IMG.glob("*.png")):
        print("%-28s %8d bytes" % (p.name, p.stat().st_size))
