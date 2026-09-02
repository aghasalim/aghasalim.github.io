"""Recompute every published value in verify/claims.tsv using Python's rounding.

Python's round() uses banker's rounding (same as R, different from C/Ruby).
Shares no code with the other verifiers.

Run: python3 verify/claims.py <repo root>
"""
import csv
import sys
from pathlib import Path

root = Path(sys.argv[1]) if len(sys.argv) > 1 else Path(".")
bad = 0
worst = 0.0

with open(root / "verify" / "claims.tsv", encoding="utf-8") as f:
    reader = csv.DictReader(f, delimiter="\t")
    rows = list(reader)

for r in rows:
    rule = r["rule"]
    a = float(r["a"])
    b = None if r["b"] == "-" else float(r["b"])
    pub = float(r["published"])

    if rule == "copy":
        got = a
    elif rule == "round0":
        got = round(a, 0)
    elif rule == "round1":
        got = round(a, 1)
    elif rule == "round2":
        got = round(a, 2)
    elif rule == "ratio0":
        got = round(a / b, 0)
    elif rule == "ratio1":
        got = round(a / b, 1)
    elif rule == "diff4":
        got = round(a - b, 4)
    else:
        print(f"unknown rule: {rule}")
        got = None

    if got is None or abs(got - pub) > 1e-12:
        print(f"DISAGREE {r['id']}: rule={rule} published={pub} got={got}")
        bad += 1
    else:
        err = abs(got - pub)
        if err > worst:
            worst = err

if bad:
    print(f"Python: {bad} disagreement(s)")
    sys.exit(1)
print(f"Python: {len(rows)} values reproduced, largest error {worst:.1e}")
