#!/usr/bin/env python
"""Reproduce upstream's CI as far as this machine allows, and record the rest.

Upstream .github/workflows/build.yml is three lines of substance:

    runs-on: ubuntu-latest
    go-version: '1.x'
    go test -cover ./...

This script runs every step that does not require a Linux kernel, plus the
cross-platform checks that close most of the remaining gap:

  1  go test -cover ./...            the CI command verbatim, on the upstream tree
  2  go test -cover ./...            the CI command verbatim, on the patched tree
  3  GOOS=linux go build ./...       the patched tree compiles for linux/amd64
  4  GOOS=linux go vet ./...         it typechecks and vets for linux/amd64
  5  GOOS=linux go test -c           the test binary compiles for linux/amd64
  6  gofmt -l                        on LF-normalised copies of the patched tree
  7  go vet ./...                    on this platform

What is NOT reproduced, and is recorded as such in evidence/ci.json:
  - executing the test binary on a Linux kernel (no container, no WSL distro,
    no gcc on this machine), so anything that would only fail at runtime on
    Linux is out of scope. The package is pure computation: no files, no
    network, no time, no goroutines, no build tags, no cgo.

Usage: python ci_equiv.py        exit status 0 means every runnable step passed
"""

import json
import os
import pathlib
import shutil
import subprocess
import sys
import tempfile

ROOT = pathlib.Path(__file__).resolve().parent.parent


def find_go():
    for cand in (os.environ.get("GO_BIN"), shutil.which("go"),
                 r"C:\Users\15082\.workbuddy\binaries\go\go\bin\go.exe"):
        if cand and pathlib.Path(cand).exists():
            return str(cand)
    sys.exit("go toolchain not found; set GO_BIN to the go executable")


GO = find_go()
ENV = dict(os.environ)
ENV.update({"GOFLAGS": "-mod=mod", "GOPROXY": "https://goproxy.cn,direct",
            "GOTOOLCHAIN": "local"})


def run(args, cwd, extra_env=None):
    env = dict(ENV)
    if extra_env:
        env.update(extra_env)
    r = subprocess.run(args, cwd=str(cwd), env=env, capture_output=True, text=True)
    return r.returncode, (r.stdout + r.stderr).strip()


def assemble(dst, src_dir, test_dir, test_glob):
    dst.mkdir(parents=True, exist_ok=True)
    for p in src_dir.glob("*.go"):
        if not p.name.endswith("_test.go"):
            shutil.copyfile(p, dst / p.name)
    shutil.copyfile(src_dir / "go.mod", dst / "go.mod")
    for p in test_dir.glob(test_glob):
        shutil.copyfile(p, dst / p.name)
    return dst


def main():
    work = pathlib.Path(tempfile.mkdtemp(prefix="astar-ci-"))
    steps = []

    def record(name, ok, detail):
        steps.append({"step": name, "ok": bool(ok), "detail": detail[-600:]})
        print(("  PASS  " if ok else "  FAIL  ") + name)
        if not ok:
            print("        " + detail.strip().replace("\n", "\n        ")[:600])

    try:
        up = assemble(work / "upstream-plain", ROOT / "patch" / "upstream",
                      ROOT / "patch" / "upstream", "*_test.go")
        pa = assemble(work / "patched", ROOT / "patched", ROOT / "tests", "*.go")

        rc, out = run([GO, "test", "-cover", "./..."], up)
        record("go test -cover ./... (upstream tree, baseline)", rc == 0, out)

        rc, out = run([GO, "test", "-cover", "./..."], pa)
        record("go test -cover ./... (patched tree, CI command verbatim)", rc == 0, out)

        rc, out = run([GO, "build", "./..."], pa, {"GOOS": "linux", "GOARCH": "amd64"})
        record("GOOS=linux GOARCH=amd64 go build ./...", rc == 0, out)

        rc, out = run([GO, "vet", "./..."], pa, {"GOOS": "linux", "GOARCH": "amd64"})
        record("GOOS=linux GOARCH=amd64 go vet ./...", rc == 0, out)

        binp = work / "patched.test.linux"
        rc, out = run([GO, "test", "-c", "-o", str(binp), "."], pa,
                      {"GOOS": "linux", "GOARCH": "amd64"})
        record("GOOS=linux GOARCH=amd64 go test -c (test binary compiles)",
               rc == 0 and binp.exists(), out)

        gofmt = str(pathlib.Path(GO).with_name("gofmt.exe" if os.name == "nt" else "gofmt"))
        if not pathlib.Path(gofmt).exists():
            gofmt = "gofmt"
        lf = work / "lf"
        lf.mkdir()
        for p in pa.glob("*.go"):
            (lf / p.name).write_bytes(p.read_bytes().replace(b"\r\n", b"\n"))
        rc, out = run([gofmt, "-l", "."], lf)
        record("gofmt -l clean (LF-normalised copies)", rc == 0 and not out.strip(), out)

        rc, out = run([GO, "vet", "./..."], pa)
        record("go vet ./... (host platform)", rc == 0, out)

        ver, _ = run([GO, "version"], work)
        report = {
            "go": ver,
            "host_os": os.name,
            "steps": steps,
            "not_reproduced": [
                "executing the test binary on a Linux kernel: no container runtime, "
                "no WSL distribution and no gcc on this machine",
            ],
            "ci_scope_note": (
                "upstream build.yml runs `go test -cover ./...` on ubuntu-latest with "
                "go-version '1.x'; it does not use -race, cgo, build tags, files, "
                "network or goroutines, so a Linux-only runtime difference is not "
                "expected, but it was not executed on Linux and is not claimed."
            ),
        }
        out_path = ROOT / "evidence" / "ci.json"
        out_path.parent.mkdir(parents=True, exist_ok=True)
        with open(out_path, "w", encoding="utf-8", newline="\n") as f:
            json.dump(report, f, indent=2)
        print("\nwritten", out_path)
        failed = [s["step"] for s in steps if not s["ok"]]
        if failed:
            print("failed steps: " + ", ".join(failed))
            return 1
        print("%d/%d steps passed" % (len(steps), len(steps)))
        return 0
    finally:
        shutil.rmtree(work, ignore_errors=True)


if __name__ == "__main__":
    sys.exit(main())
