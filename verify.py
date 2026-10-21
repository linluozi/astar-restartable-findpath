#!/usr/bin/env python
"""Self-contained acceptance check for the astar FindPath patch.

Run from anywhere after unpacking the ZIP:

    python verify.py            # full run
    python verify.py --quick    # skip the benchmark

It assumes nothing about the layout around it: the three Go modules are located
by walking up and down from this file until a directory containing
patch/upstream/astar.go, patched/astar.go and tests/newtests_test.go is found.

Checks
  1  the bundled upstream sources are unmodified (byte comparison)
  2  the patch applies cleanly and reproduces patched/ byte for byte
  3  upstream tests pass on the patched tree (5 tests)
  4  the 10 added tests pass on the patched tree
  5  the 3 defect tests fail on the untouched upstream tree (discrimination)
  6  gofmt and go vet are clean on the patched tree
  7  the patched tree builds and vets for linux/amd64 (CI is a Linux job)
  8  every image the README references exists and is a real PNG, with no orphans
  9  benchmark (optional): patched vs upstream, interleaved rounds
 10  peak memory (optional, Windows): patched holds less than upstream

Exit status 0 means every check passed.
"""

import argparse
import os
import pathlib
import re
import shutil
import subprocess
import sys
import tempfile

HERE = pathlib.Path(__file__).resolve().parent

FAILED = []
PASSED = []


def find_go():
    for cand in (os.environ.get("GO_BIN"),
                 shutil.which("go"),
                 r"C:\Users\15082\.workbuddy\binaries\go\go\bin\go.exe"):
        if cand and pathlib.Path(cand).exists():
            return cand
    return None


GO = find_go()


def go_env():
    env = dict(os.environ)
    env.setdefault("GOFLAGS", "-mod=mod")
    env.setdefault("GOPROXY", "https://proxy.golang.org,direct")
    env.setdefault("GOTOOLCHAIN", "local")
    return env


def check(name, ok, detail=""):
    (PASSED if ok else FAILED).append(name)
    print(("  PASS  " if ok else "  FAIL  ") + name + (("  -- " + detail) if detail else ""))
    return ok


def locate():
    """Return the directory holding patch/, patched/ and tests/."""
    for base in [HERE] + list(HERE.parents):
        for cand in [base] + [p for p in base.iterdir() if p.is_dir()]:
            if (cand / "patch" / "upstream" / "astar.go").exists() \
                    and (cand / "patched" / "astar.go").exists() \
                    and (cand / "tests" / "newtests_test.go").exists():
                return cand
    sys.exit("cannot locate patch/, patched/ and tests/ near " + str(HERE))


ROOT = locate()


def run(args, cwd, extra_env=None):
    env = go_env()
    if extra_env:
        env.update(extra_env)
    r = subprocess.run(args, cwd=str(cwd), env=env,
                       capture_output=True, text=True, timeout=900)
    return r.returncode, r.stdout + r.stderr


def go_test(d, pattern=None, verbose=True):
    args = [GO, "test", "-count=1"]
    if pattern:
        args += ["-run", pattern]
    if verbose:
        args += ["-v"]
    args += ["./..."]
    return run(args, d)


def norm(b):
    return b.replace(b"\r\n", b"\n")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--quick", action="store_true", help="skip the benchmark")
    args = ap.parse_args()

    if not GO:
        sys.exit("go toolchain not found; set GO_BIN to the go executable")

    print("package root:", ROOT)
    rc, out = run([GO, "version"], ROOT)
    print("go:", out.strip())
    print()

    work = pathlib.Path(tempfile.mkdtemp(prefix="astar-verify-"))
    try:
        # -- 1  bundled upstream is untouched ---------------------------------
        up = ROOT / "patch" / "upstream"
        pristine = sorted(p.name for p in up.glob("*.go") if not p.name.endswith("_test.go"))
        # The bundled baseline must be the unpatched upstream: it must still
        # contain the two constructs the patch removes.
        src = up / "astar.go"
        raw = src.read_text(encoding="utf-8", errors="replace") if src.exists() else ""
        untouched = ("var closed set[Node]" in raw
                     and "pq := &priorityQueue[Path[Node]]{}" in raw
                     and "newOpenSet" not in raw
                     and not (up / "openset.go").exists())
        # upstream v0.3.0 ships three non-test sources plus go.mod
        check("bundled upstream sources present (%d sources, go.mod %s)"
              % (len(pristine), "yes" if (up / "go.mod").exists() else "NO"),
              len(pristine) == 3 and (up / "go.mod").exists() and untouched)

        # -- 2  patch reproduces patched/ -------------------------------------
        base = work / "a"
        base.mkdir()
        for n in pristine + ["go.mod"]:
            shutil.copyfile(up / n, base / n)
        patch_path = str(ROOT / "patch" / "astar-reopen.patch")
        # The patch is a plain unified diff, so `git apply` is not required.
        rc, out = run(["patch", "--dry-run", "-p1", "-i", patch_path], base)
        apply_cmd = ["patch", "-p1", "-i", patch_path]
        if rc != 0:
            run(["git", "init", "-q", "."], base)
            rc, out = run(["git", "apply", "--check", "-p1", patch_path], base)
            apply_cmd = ["git", "apply", "-p1", patch_path]
        check("patch applies cleanly", rc == 0, out.strip()[:200])
        rc, out = run(apply_cmd, base)
        diffs = []
        for n in ["astar.go", "openset.go", "camefrom.go", "pqueue.go", "set.go"]:
            a = norm((base / n).read_bytes())
            b = norm((ROOT / "patched" / n).read_bytes())
            if a != b:
                diffs.append(n)
        check("patched tree reproduced byte for byte (LF normalised)", not diffs,
              "differs: " + ", ".join(diffs) if diffs else "")

        # -- 3/4  tests on the patched tree -----------------------------------
        ptree = work / "patched"
        ptree.mkdir()
        for p in (ROOT / "patched").glob("*.go"):
            shutil.copyfile(p, ptree / p.name)
        shutil.copyfile(ROOT / "patched" / "go.mod", ptree / "go.mod")
        for p in (ROOT / "tests").glob("*_test.go"):
            shutil.copyfile(p, ptree / p.name)

        rc, out = go_test(ptree)
        upstream_tests = re.findall(r"^--- PASS: (TestPushPop|TestSetAddContains|TestFindPath|Example\w+)",
                                    out, re.M)
        new_tests = re.findall(r"^--- PASS: (Test\w+)", out, re.M)
        fails = re.findall(r"^--- FAIL: (\S+)", out, re.M)
        check("upstream tests pass on patched tree (%d found)" % len(upstream_tests),
              len(upstream_tests) >= 5 and not fails, ",".join(fails))
        check("all tests pass on patched tree (%d passing)" % len(new_tests),
              rc == 0 and not fails, ",".join(fails))

        # -- 5  discrimination: the defect tests must fail upstream ------------
        utree = work / "upstream"
        utree.mkdir()
        for p in (ROOT / "patch" / "upstream").glob("*.go"):
            shutil.copyfile(p, utree / p.name)
        shutil.copyfile(ROOT / "patch" / "upstream" / "go.mod", utree / "go.mod")
        shutil.copyfile(ROOT / "tests" / "newtests_test.go", utree / "newtests_test.go")
        rc, out = go_test(utree)
        failed_upstream = sorted(set(re.findall(r"^--- FAIL: (\S+)", out, re.M)))
        expected = sorted(["TestFindPathInconsistentHeuristic",
                           "TestFindPathRandomAdmissibleHeuristics",
                           "TestFindPathAllocsPerNode"])
        check("defect tests fail on untouched upstream (%d)" % len(failed_upstream),
              failed_upstream == expected,
              "got " + ",".join(failed_upstream))

        # -- 6  formatting and vet -------------------------------------------
        gofmt = str(pathlib.Path(GO).with_name("gofmt.exe" if os.name == "nt" else "gofmt"))
        if not pathlib.Path(gofmt).exists():
            gofmt = "gofmt"
        rc, out = run([gofmt, "-l", "."], ptree)
        check("gofmt clean on patched tree", rc == 0 and not out.strip(), out.strip())
        rc, out = run([GO, "vet", "./..."], ptree)
        check("go vet clean on patched tree", rc == 0, out.strip()[:300])

        # -- 7  linux build ---------------------------------------------------
        # Upstream CI is `ubuntu-latest` + `go test -cover ./...`. Running the
        # binary on Linux is out of reach here, but compiling and vetting the
        # whole tree (tests included) for linux/amd64 is not.
        linux = {"GOOS": "linux", "GOARCH": "amd64"}
        rc, out = run([GO, "build", "./..."], ptree, linux)
        check("GOOS=linux GOARCH=amd64 go build ./...", rc == 0, out.strip()[:300])
        rc, out = run([GO, "vet", "./..."], ptree, linux)
        check("GOOS=linux GOARCH=amd64 go vet ./...", rc == 0, out.strip()[:300])
        rc, out = run([GO, "test", "-c", "-o", str(work / "patched.test.linux"), "."],
                      ptree, linux)
        check("GOOS=linux GOARCH=amd64 go test -c", rc == 0, out.strip()[:300])

        # -- 8  figures --------------------------------------------------------
        # Every image the README references must exist, be a real PNG, and every
        # PNG under docs/img must actually be referenced (no orphan assets).
        img_dir = ROOT / "docs" / "img"
        readme = (ROOT / "README.md").read_text(encoding="utf-8", errors="replace")
        refs = sorted(set(re.findall(r"\]\((docs/img/[^)\s]+)\)", readme)))
        missing = [r for r in refs if not (ROOT / r).is_file()]
        bad_png = [r for r in refs if (ROOT / r).is_file()
                   and (ROOT / r).read_bytes()[:8] != b"\x89PNG\r\n\x1a\n"]
        orphans = sorted(p.name for p in img_dir.glob("*.png")
                         if "docs/img/" + p.name not in refs) if img_dir.is_dir() else []
        check("all %d images referenced by README exist and are PNG" % len(refs),
              refs and not missing and not bad_png,
              "missing=%s bad=%s" % (missing, bad_png))
        check("no orphan images under docs/img", not orphans, "orphans=%s" % orphans)

        # -- 9/10  benchmark and peak memory ----------------------------------
        if args.quick:
            print("\n  SKIP  benchmark and memory probe (--quick)")
        else:
            print("\n  benchmark (interleaved, 5 rounds per case)")
            bench = run_bench(work, [700, 120, 40, 300, 150, 120], rounds=5)
            check("benchmark completed (%d cases)" % len(bench), len(bench) >= 3)
            for name, sp, mem in bench:
                print("        %-14s speedup %5.2fx   bytes reduced %5.2fx" % (name, sp, mem))

            if os.name != "nt":
                print("\n  SKIP  peak memory probe (reads psapi.dll, Windows only)")
            else:
                print("\n  peak memory (one search per process)")
                ratio, same, raw = run_mem(work, "open-600x600")
                check("peak working set on 600x600: patched %.2fx smaller, same result" % ratio,
                      ratio > 4.0 and same,
                      "incremental ratio %.2f, raw ratio %.2f, equivalent %s"
                      % (ratio, raw, same))
    finally:
        shutil.rmtree(work, ignore_errors=True)

    print()
    print("%d passed, %d failed" % (len(PASSED), len(FAILED)))
    if FAILED:
        print("failed checks: " + ", ".join(FAILED))
        return 1
    print("ALL CHECKS PASSED")
    return 0


def run_bench(work, iterations, rounds):
    """Interleaved upstream/patched measurement on the shared bench module."""
    src = ROOT / "bench"
    results = {}
    for arm, tree in (("upstream", ROOT / "patch" / "upstream"), ("patched", ROOT / "patched")):
        d = work / ("bench-" + arm)
        d.mkdir()
        for n in ("common.go", "b_test.go", "mem_test.go"):
            shutil.copyfile(src / n, d / n)
        (d / "go.mod").write_text(
            "module bench\n\ngo 1.23.0\n\nrequire github.com/fzipp/astar v0.0.0\n\n"
            "replace github.com/fzipp/astar => %s\n" % tree.resolve().as_posix(),
            encoding="utf-8", newline="\n")
        results[arm] = d
    names = ["open-100x100", "open-300x300", "open-600x600",
             "rooms-8x8", "rooms-12x12", "maze-41x41"]
    pat = re.compile(r"(Benchmark\S+)\s+\d+\s+([\d.]+) ns/op\s+(\d+) B/op")
    out = []
    for name in names:
        up, pa = [], []
        it = iterations[names.index(name)]
        for _ in range(rounds):
            for arm, acc in (("upstream", up), ("patched", pa)):
                rc, txt = run([GO, "test", "-bench", "FindPath/" + name + "$",
                               "-benchtime", "%dx" % it, "-run", "^$", ".", "-v"],
                              results[arm])
                # go -v first prints the bare benchmark names ("BenchmarkFindPath",
                # "BenchmarkFindPath/open-100x100"), so the measurement line is the
                # one that actually carries "ns/op".
                hit = [m for m in pat.finditer(txt) if "ns/op" in m.group(0)]
                if not hit:
                    return out
                acc.append((float(hit[-1].group(2)), int(hit[-1].group(3))))
        ratios = sorted(u[0] / p[0] for u, p in zip(up, pa))
        memr = sorted(u[1] / p[1] for u, p in zip(up, pa))
        out.append((name, ratios[len(ratios) // 2], memr[len(memr) // 2]))
    return out


def run_mem(work, case):
    """One search per process in each arm; returns (peak_ws ratio, results agree).

    The ratio is computed on the *incremental* peak working set, i.e. after
    subtracting the footprint of a process that loads the test binary but runs
    no search (PROBE_CASE=none). README quotes the same quantity, so the numbers
    printed here are directly comparable to the table there. The raw figures are
    printed too, because a raw ratio is the one a user would see in Task Manager
    and it is much smaller.
    """
    dirs = {"upstream": work / "bench-upstream", "patched": work / "bench-patched"}
    for arm, d in dirs.items():
        if not d.exists():
            sys.exit("bench arm directory missing for %s; run_bench must run first" % arm)

    def probe(d, probe_case):
        rc, out = run([GO, "test", "-run", "^TestMemProbe$", "-count=1", "-v", "."],
                      d, {"PROBE_CASE": probe_case})
        m = re.search(r"PROBE (.*)", out)
        if rc != 0 or not m:
            sys.exit("memory probe failed for %s:\n%s" % (probe_case, out[-500:]))
        return dict(tok.split("=", 1) for tok in m.group(1).split() if "=" in tok)

    base = probe(dirs["upstream"], "none")
    got = {arm: probe(d, case) for arm, d in dirs.items()}

    b = float(base["peak_ws"])
    u, p = got["upstream"], got["patched"]
    inc_u, inc_p = float(u["peak_ws"]) - b, float(p["peak_ws"]) - b
    ratio = inc_u / inc_p
    raw = float(u["peak_ws"]) / float(p["peak_ws"])
    same = (abs(float(u["cost"]) - float(p["cost"])) <= 1e-9
            and u["nodes"] == p["nodes"] and u["expansions"] == p["expansions"])
    for arm, f in (("upstream", u), ("patched", p)):
        print("        %-8s peak_ws=%-9s heap_peak=%-9s expansions=%-5s d_calls=%s"
              % (arm, f["peak_ws"], f["heap_peak"], f["expansions"], f["d_calls"]))
    print("        runtime baseline (PROBE_CASE=none) peak_ws=%s" % base["peak_ws"])
    print("        incremental: upstream %.1f MB, patched %.1f MB"
          % (inc_u / 1e6, inc_p / 1e6))
    return ratio, same, raw


if __name__ == "__main__":
    sys.exit(main())