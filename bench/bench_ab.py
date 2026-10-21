#!/usr/bin/env python
"""Interleaved A/B benchmark for the astar patch.

Each arm runs in its own `go test -bench` process; the arm directories are
generated on first use under `bench/arms/<arm>/` with a replace directive that
points at `patch/upstream` or `patched`. Rounds alternate upstream/patched so
that machine drift affects both arms equally; the reported number is the median
of the per-round ratios, plus the full sample.

Also measures a noise floor: the same binary benchmarked twice in a row (A/A).

Usage:  python bench_ab.py [rounds] [case1,case2,...]
"""

import json
import os
import re
import shutil
import statistics
import subprocess
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


def find_go():
    for cand in (os.environ.get("GO_BIN"), shutil.which("go"),
                 r"C:\Users\15082\.workbuddy\binaries\go\go\bin\go.exe"):
        if cand and os.path.exists(cand):
            return cand
    sys.exit("go toolchain not found; set GO_BIN to the go executable")


GO = find_go()
ENV = dict(os.environ)
ENV.update({
    "GOPATH": os.environ.get("GOPATH", r"C:\Users\15082\.workbuddy\binaries\go\gopath"),
    "GOFLAGS": "-mod=mod",
    "GOPROXY": "https://goproxy.cn,direct",
    "CGO_ENABLED": "0",
})

BENCHES = [
    "open-100x100",
    "open-300x300",
    "open-600x600",
    "rooms-8x8",
    "rooms-12x12",
    "maze-41x41",
]

LINE = re.compile(
    r"^(Benchmark\S+)\s+(\d+)\s+([\d.]+) ns/op\s+(\d+) B/op\s+(\d+) allocs/op"
)

ARM_SOURCE = {"upstream": "patch/upstream", "patched": "patched"}


def ensure_arm(arm):
    """Create bench/arms/<arm>/ on first use, with a replace to the right tree."""
    d = os.path.join(ROOT, "bench", "arms", arm)
    os.makedirs(d, exist_ok=True)
    for name in ("common.go", "b_test.go"):
        src = os.path.join(ROOT, "bench", name)
        dst = os.path.join(d, name)
        with open(src, "rb") as f:
            data = f.read()
        with open(dst, "wb") as f:
            f.write(data)
    target = os.path.join(ROOT, *ARM_SOURCE[arm].split("/"))
    gomod = ("module bench\n\ngo 1.23.0\n\n"
             "require github.com/fzipp/astar v0.0.0\n\n"
             "replace github.com/fzipp/astar => %s\n"
             % target.replace("\\", "/"))
    with open(os.path.join(d, "go.mod"), "w", encoding="utf-8", newline="\n") as f:
        f.write(gomod)
    return d


def run(arm, bench, n, count=1):
    """Run one benchmark in one arm, return [(ns, bytes, allocs), ...]."""
    # Go 1.26 matches -bench against the name *with* the "-<GOMAXPROCS>" suffix,
    # so a "^Name$" pattern fails to match and go still exits 0 with no
    # benchmark line. The working form is the full name anchored only at the end,
    # which also selects exactly one sub-benchmark per process.
    cmd = [GO, "test", "-bench", "FindPath/" + bench + "$",
           "-benchtime", "%dx" % n, "-count", str(count), "-run", "^$", "-v", "."]
    out = subprocess.run(cmd, cwd=ensure_arm(arm),
                         env=ENV, capture_output=True, text=True)
    if out.returncode != 0:
        sys.exit("arm %s failed: %s %s" % (arm, out.stdout, out.stderr))
    res = []
    for line in out.stdout.splitlines():
        line = line.strip()
        if not line.startswith("BenchmarkFindPath/" + bench):
            continue
        m = LINE.match(line)
        if m:
            res.append((float(m.group(3)), int(m.group(4)), int(m.group(5))))
    if len(res) != count:
        sys.exit("arm %s bench %s: expected %d samples, got %d: %s"
                 % (arm, bench, count, len(res), out.stdout))
    return res


def main():
    global BENCHES  # noqa: PLW0603 - allows a subset via argv
    rounds = int(sys.argv[1]) if len(sys.argv) > 1 else 7
    if len(sys.argv) > 2:
        BENCHES = sys.argv[2].split(",")
    sizes = {  # fixed iteration counts keep each round short and comparable
        "open-100x100": 2000,
        "open-300x300": 400,
        "open-600x600": 60,
        "rooms-8x8": 600,
        "rooms-12x12": 300,
        "maze-41x41": 300,
    }

    samples = {b: {"upstream": [], "patched": []} for b in BENCHES}
    noise = {b: [] for b in BENCHES}

    # Noise floor: same arm twice, back to back, within a round.
    for _ in range(rounds):
        for b in BENCHES:
            a1 = run("patched", b, sizes[b])[0][0]
            a2 = run("patched", b, sizes[b])[0][0]
            noise[b].append(a2 / a1)

    for r in range(rounds):
        for b in BENCHES:
            u = run("upstream", b, sizes[b])[0]
            p = run("patched", b, sizes[b])[0]
            samples[b]["upstream"].append(u)
            samples[b]["patched"].append(p)
        print("round %d/%d done" % (r + 1, rounds), flush=True)

    report = {"rounds": rounds, "cases": {}}
    print()
    print("%-22s %12s %12s %9s %10s %10s %8s" % (
        "case", "up median", "patched med", "speedup", "bytes x", "allocs x", "A/A band"))
    for b in BENCHES:
        us = samples[b]["upstream"]
        ps = samples[b]["patched"]
        um = statistics.median([x[0] for x in us])
        pm = statistics.median([x[0] for x in ps])
        ratios = [u[0] / p[0] for u, p in zip(us, ps)]
        mb = statistics.median([u[1] / p[1] for u, p in zip(us, ps)])
        ma = statistics.median([u[2] / p[2] for u, p in zip(us, ps)])
        nz = noise[b]
        report["cases"][b] = {
            "upstream_ns": [x[0] for x in us],
            "patched_ns": [x[0] for x in ps],
            "upstream_bytes": [x[1] for x in us],
            "patched_bytes": [x[1] for x in ps],
            "upstream_allocs": [x[2] for x in us],
            "patched_allocs": [x[2] for x in ps],
            "ratio_median": statistics.median(ratios),
            "ratio_min": min(ratios),
            "ratio_max": max(ratios),
            "bytes_ratio_median": mb,
            "allocs_ratio_median": ma,
            "aa_ratio_min": min(nz),
            "aa_ratio_max": max(nz),
        }
        print("%-22s %12.0f %12.0f %8.2fx %9.2fx %9.2fx %6.2f-%.2f" % (
            b, um, pm, statistics.median(ratios), mb, ma,
            min(nz), max(nz)))

    out = os.path.join(ROOT, "evidence", "bench.json")
    os.makedirs(os.path.dirname(out), exist_ok=True)
    with open(out, "w", encoding="utf-8", newline="\n") as f:
        json.dump(report, f, indent=2)
    print("\nwritten", os.path.normpath(out))


if __name__ == "__main__":
    main()
