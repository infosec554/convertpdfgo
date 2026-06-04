#!/usr/bin/env python3
"""Sandboxed Python runner for AI-generated XLSX-builder code.

Why: when the LLM writes Excel formulas with cell references, it
constantly mis-indexes rows ('=B1*B2' when the data is in B4*B5).
The Code-Interpreter pattern sidesteps the problem — the LLM writes
Python that computes the values directly, we run it in a strict
sandbox, and the renderer gets fully-resolved numbers. No formulas,
no cell-ref bugs, ever.

The runner enforces three hard guards before exec:

  1. Restricted __builtins__       — no open / exec / eval / compile / __import__
  2. Whitelist module set          — math, statistics, datetime, json, re only
  3. multiprocessing.Process       — hard wall-clock timeout, separate memory

The script must define a top-level variable named `result` which is
the workbook spec the renderer consumes.

Usage:
  python3 sandbox_xlsx.py <code.py> <result.json>

Exits with code 0 on success, non-zero on any error (timeout,
syntax error, missing `result`, non-serializable `result`).
"""
import json
import math
import multiprocessing
import statistics
import sys
import datetime as _datetime_mod
import re as _re_mod


SAFE_BUILTINS = {
    # core types + constants
    "True": True, "False": False, "None": None,
    "bool": bool, "int": int, "float": float, "str": str,
    "list": list, "dict": dict, "tuple": tuple, "set": set, "frozenset": frozenset,
    # arithmetic / comparison helpers
    "abs": abs, "round": round, "min": min, "max": max, "sum": sum,
    "divmod": divmod, "pow": pow,
    # iteration / collection
    "len": len, "range": range, "enumerate": enumerate, "zip": zip,
    "sorted": sorted, "reversed": reversed, "map": map, "filter": filter,
    "any": any, "all": all,
    # printing / introspection (for debugging — stdout is captured)
    "print": print, "isinstance": isinstance, "type": type,
    # string / number conversion
    "repr": repr, "format": format, "hex": hex, "bin": bin, "oct": oct, "chr": chr, "ord": ord,
    # exceptions the AI might raise/catch deliberately
    "Exception": Exception, "ValueError": ValueError, "TypeError": TypeError,
    "ZeroDivisionError": ZeroDivisionError, "KeyError": KeyError, "IndexError": IndexError,
}

# These get exposed as already-imported modules in the exec namespace
# so the AI can use them WITHOUT calling __import__ (which is blocked).
SAFE_MODULES = {
    "math":       math,
    "statistics": statistics,
    "datetime":   _datetime_mod,
    "json":       json,
    "re":         _re_mod,
}


def _runner(code: str, out_queue):
    """Run inside a fresh process. Puts ('ok', result) on the queue,
    or ('err', message) on any failure."""
    try:
        ns = {"__builtins__": SAFE_BUILTINS}
        ns.update(SAFE_MODULES)
        exec(code, ns)  # noqa: S102 — sandbox is the point
        if "result" not in ns:
            out_queue.put(("err", "script did not define a top-level `result` variable"))
            return
        result = ns["result"]
        try:
            # Reject anything not JSON-serializable so the parent process
            # doesn't crash later.
            payload = json.dumps(result, default=str)
        except (TypeError, ValueError) as e:
            out_queue.put(("err", f"`result` is not JSON-serializable: {e}"))
            return
        out_queue.put(("ok", payload))
    except SyntaxError as e:
        out_queue.put(("err", f"syntax error: {e.msg} (line {e.lineno})"))
    except Exception as e:  # noqa: BLE001 — sandbox failure routing
        out_queue.put(("err", f"runtime error: {type(e).__name__}: {e}"))


def run_sandboxed(code: str, timeout_s: float = 8.0):
    """Run `code` in a child process with a wall-clock timeout."""
    ctx = multiprocessing.get_context("spawn")
    q = ctx.Queue()
    p = ctx.Process(target=_runner, args=(code, q))
    p.start()
    p.join(timeout_s)
    if p.is_alive():
        p.terminate()
        p.join(1.0)
        if p.is_alive():
            p.kill()
        return ("err", f"timeout after {timeout_s}s")
    if q.empty():
        return ("err", "sandbox returned no result (process likely killed by OS — out of memory?)")
    return q.get()


def main():
    if len(sys.argv) != 3:
        print("usage: sandbox_xlsx.py <code.py> <result.json>", file=sys.stderr)
        sys.exit(2)
    code_path = sys.argv[1]
    out_path  = sys.argv[2]

    with open(code_path) as f:
        code = f.read()

    # Hard size cap — defends against a runaway prompt that returns
    # 50 MB of code we'd then have to parse.
    if len(code) > 64_000:
        print("code too large (>64KB)", file=sys.stderr)
        sys.exit(3)

    status, payload = run_sandboxed(code)
    if status == "err":
        print(f"sandbox error: {payload}", file=sys.stderr)
        sys.exit(4)

    with open(out_path, "w") as f:
        f.write(payload)
    print(f"OK {out_path}")


if __name__ == "__main__":
    main()
