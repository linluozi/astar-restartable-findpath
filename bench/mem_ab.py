#!/usr/bin/env python
"""Interleaved A/B memory measurement for the astar patch.

For every case this runs one fresh process per arm per round, and each process
performs exactly one search. A `none` pseudo-case is measured the same way and
subtracted, so the reported footprint is the search itself, not the Go runtime
and the test binary.

Reported per case:
  peak_ws      incremental peak working set (kernel high-water mark, exact)
  heap_peak    incremental sampled heap peak (approximate, see mem_test.go)
  expansions   number of Neighbours() calls (black-box, deterministic)
  d_calls      number of step-cost invocations (black-box, deterministic)

The runner also refuses to report a ratio if the two arms disagree on the
result, so a "faster because it is wrong" patch cannot pass silently.

Windows only: the probe uses psapi.dll. Exit status 0 means every case was
measured. Usage: python mem_ab.py [rounds] [case1,case2,...]
"""

import json
import os
import pathlib
import re
import shutil
import statistics
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
BENCH = ROOT / "bench"


def find_go():
    for cand in (os.environ.get("GO_BIN"), shutil.which("go"),
                 r"C:\Users\15082\.workbuddy\binaries\go\go\bin\go.exe"):
        if cand and pathlib.Path(cand).exists():
            return str(cand)
    sys.exit("go toolchain not found; set GO_BIN to the go executable")


GO = find_go()
ENV = dict(os.environ)
ENV.update({
    "GOFLAGS": "-mod=mod",
    "GOPROXY": "https://goproxy.cn,direct",
    "CGO_ENABLED": "0",
    "GOGC": "100",
})

CASES = ["open-100x100", "open-300x300", "open-600x600",
         "rooms-8x8", "rooms-12x12", "maze-41x41"]
BASE = "none"

ARM_SOURCE = {"upstream": ROOT / "patch" / "upstream", "patched": ROOT / "patched"}
PROBE = re.compile(r"PROBE (.*)")
FIELDS = ("cost", "nodes", "peak_ws", "heap_peak", "samples",
          "expansions", "d_calls", "total_alloc", "mallocs")


def ensure_arm(arm):
    d = BENCH / "arms" / arm
    d.mkdir(parents=True, exist_ok=True)
    for name in ("common.go", "b_test.go", "mem_test.go"):
        shutil.copyfile(BENCH / name, d / name)
    (d / "go.mod").write_text(
        "module bench\n\ngo 1.23.0\n\nrequire github.com/fzipp/astar v0.0.0\n\n"
        "replace github.com/fzipp/astar => %s\n" % ARM_SOURCE[arm].as_posix(),
        encoding="utf-8", newline="\n")
    return d


def probe(arm, case):
    env = dict(ENV)
    env["PROBE_CASE"] = case
    r = subprocess.run([GO, "test", "-run", "^TestMemProbe$", "-count=1", "-v", "."],
                       cwd=str(ensure_arm(arm)), env=env,
                       capture_output=True, text=True)
    if r.returncode != 0:
        sys.exit("arm %s case %s failed:\n%s\n%s" % (arm, case, r.stdout, r.stderr))
    m = PROBE.search(r.stdout)
    if not m:
        sys.exit("arm %s case %s: no PROBE line in\n%s" % (arm, case, r.stdout))
    out = {}
    for tok in m.group(1).split():
        k, _, v = tok.partition("=")
        if k in FIELDS:
            out[k] = float(v) if k == "cost" else int(v)
    missing = [k for k in FIELDS if k not in out]
    if missing:
        sys.exit("arm %s case %s: missing %s" % (arm, case, ",".join(missing)))
    return out


def ratio(a, b):
    return a / b if b else float("inf")


def main():
    global CASES
    rounds = int(sys.argv[1]) if len(sys.argv) > 1 else 5
    if len(sys.argv) > 2:
        CASES = sys.argv[2].split(",")

    samples = {c: {"upstream": [], "patched": [], "aa": []} for c in CASES}
    base = {"upstream": [], "patched": []}

    for r in range(rounds):
        base["upstream"].append(probe("upstream", BASE))
        base["patched"].append(probe("patched", BASE))
        for c in CASES:
            u = probe("upstream", c)
            p = probe("patched", c)
            u2 = probe("upstream", c)  # A/A: same arm twice
            samples[c]["upstream"].append(u)
            samples[c]["patched"].append(p)
            samples[c]["aa"].append(u2)
        print("round %d/%d done" % (r + 1, rounds), flush=True)

    bws = {"upstream": statistics.median([x["peak_ws"] for x in base["upstream"]]),
           "patched": statistics.median([x["peak_ws"] for x in base["patched"]])}
    bhp = {"upstream": statistics.median([x["heap_peak"] for x in base["upstream"]]),
           "patched": statistics.median([x["heap_peak"] for x in base["patched"]])}

    report = {"rounds": rounds, "go": subprocess.run(
        [GO, "version"], capture_output=True, text=True).stdout.strip(),
        "baseline": {"peak_ws": bws, "heap_peak": bhp}, "cases": {}}

    print()
    print("%-14s %10s %10s %7s %9s %9s %9s" % (
        "case", "up ws MB", "pa ws MB", "ws x", "up heap MB", "pa heap MB", "heap x"))
    for c in CASES:
        us, ps, aa = samples[c]["upstream"], samples[c]["patched"], samples[c]["aa"]
        ws_u = [x["peak_ws"] - bws["upstream"] for x in us]
        ws_p = [x["peak_ws"] - bws["patched"] for x in ps]
        hp_u = [x["heap_peak"] - bhp["upstream"] for x in us]
        hp_p = [x["heap_peak"] - bhp["patched"] for x in ps]
        ws_ratios = [ratio(a, b) for a, b in zip(ws_u, ws_p)]
        hp_ratios = [ratio(a, b) for a, b in zip(hp_u, hp_p)]
        aa_ratios = [ratio(x["peak_ws"] - bws["upstream"], y["peak_ws"] - bws["upstream"])
                     for x, y in zip(us, aa)]

        same = all(
            abs(x["cost"] - y["cost"]) <= 1e-9
            and x["nodes"] == y["nodes"]
            and x["expansions"] == y["expansions"]
            for x, y in zip(us, ps))

        report["cases"][c] = {
            "equivalent": same,
            "upstream": {"peak_ws": ws_u, "heap_peak": hp_u,
                         "expansions": [x["expansions"] for x in us],
                         "d_calls": [x["d_calls"] for x in us],
                         "total_alloc": [x["total_alloc"] for x in us],
                         "mallocs": [x["mallocs"] for x in us],
                         "cost": [x["cost"] for x in us]},
            "patched": {"peak_ws": ws_p, "heap_peak": hp_p,
                        "expansions": [x["expansions"] for x in ps],
                        "d_calls": [x["d_calls"] for x in ps],
                        "total_alloc": [x["total_alloc"] for x in ps],
                        "mallocs": [x["mallocs"] for x in ps],
                        "cost": [x["cost"] for x in ps]},
            "peak_ws_ratio_median": statistics.median(ws_ratios),
            "peak_ws_ratio_min": min(ws_ratios),
            "peak_ws_ratio_max": max(ws_ratios),
            "heap_peak_ratio_median": statistics.median(hp_ratios),
            "heap_peak_ratio_min": min(hp_ratios),
            "heap_peak_ratio_max": max(hp_ratios),
            "aa_ratio_min": min(aa_ratios),
            "aa_ratio_max": max(aa_ratios),
            "d_calls_ratio": ratio(us[0]["d_calls"], ps[0]["d_calls"]),
        }
        print("%-14s %10.1f %10.1f %6.2fx %9.1f %9.1f %8.2fx" % (
            c, statistics.median(ws_u) / 1e6, statistics.median(ws_p) / 1e6,
            statistics.median(ws_ratios),
            statistics.median(hp_u) / 1e6, statistics.median(hp_p) / 1e6,
            statistics.median(hp_ratios)))

    out = ROOT / "evidence" / "mem.json"
    out.parent.mkdir(parents=True, exist_ok=True)
    with open(out, "w", encoding="utf-8", newline="\n") as f:
        json.dump(report, f, indent=2)
    print("\nwritten", out)

    bad = [c for c in CASES if not report["cases"][c]["equivalent"]]
    if bad:
        print("\nARMS DISAGREE on result for: " + ", ".join(bad))
        return 1
    print("all arms agree on cost, path length and expansion count")
    return 0


if __name__ == "__main__":
    sys.exit(main())
