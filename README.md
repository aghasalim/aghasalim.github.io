# aghasalim.github.io

My personal site. One hand written page, no build step, no framework, served by
GitHub Pages straight from this branch.

```
index.html    the page, with its CSS and JSON-LD inline
sitemap.xml   one URL
robots.txt    allow everything, point at the sitemap
.nojekyll     skip the Jekyll pass
```

Open `index.html` in a browser to see it. There is nothing to install.

## Every number on the page is checked

The page is a list of results, and each number on it was copied by hand out of
the README of the repository it links to. That is exactly the kind of edit that
goes wrong quietly: a digit drops, a value gets updated in the source repository
and not here, or a ratio gets worked out in my head and lands one decimal off.
The page renders fine either way and nothing complains.

So the numbers now live in `verify/claims.tsv` as well. Each row records where
the value came from, the values it was derived from, the rule used to derive it,
and the phrase that has to appear on the page. `verify/claims.tsv` records 19
claims. Ten are copied straight from a source table, five are rounded, three are
a ratio and one is a difference. Every one of them is recomputed by seven
independent implementations, and CI fails if any of them disagrees with the
page.

Run it all:

```
./verify/verify.sh
```

| implementation | what it recomputes | measured agreement |
|---|---|---|
| [`verify/claims.sql`](verify/claims.sql) | all 19 published values with SQLite's own arithmetic and rounding, plus the row counts per rule and per source repository | 19 rows, 0 disagreements, largest error 0.0 |
| [`verify/claims.c`](verify/claims.c) | the same 19 values with C `round`, columns resolved by name from the header | 19 values reproduced, largest error 0.0e+00 |
| [`verify/gocheck`](verify/gocheck) | tag balance, every link and asset, id uniqueness, sitemap against canonical, robots against sitemap, the structure of `claims.tsv`, the claim count quoted here, and the 19 values again | no problems, 19 claims reproduced, largest error 0.0e+00 |
| [`verify/claims.js`](verify/claims.js) | the 19 values in Node, and separately every claim phrase in the rendered text of the page, the JSON-LD block through a real JSON parser, and its identity URL against the canonical link and the sitemap | 19 phrases found once each, 19 values reproduced, largest error 0.0e+00 |
| [`verify/claims.py`](verify/claims.py) | the 19 values in Python, whose `round()` uses banker's rounding | 19 values reproduced, largest error 0.0e+00 |
| [`verify/claims.R`](verify/claims.R) | the same values in R, also banker's rounding but a different implementation | 19 values reproduced, largest error 0.0e+00 |
| [`verify/claims.rb`](verify/claims.rb) | the same values in Ruby, which uses round-half-up by default | 19 values reproduced, largest error 0.0e+00 |

The rounding disagreement across languages is the point. Python and R use
banker's rounding (round half to even), C and Ruby use round-half-away-from-zero,
and SQLite and Go each have their own. If all seven agree, no published value
sits on a rounding boundary where the rule matters.

The CI job runs the whole thing, then corrupts `verify/claims.tsv` on purpose
and requires the run to fail, then restores it and requires it to pass again. A
check that cannot fail is not checking anything.

### What this caught

The page used to say the Schrodinger bridge work wins "by 2x to 12x". The source
table gives 1.84x, 1.29x and 2.71x on the ODE column and 6.77x, 3.52x and 12.08x
on the SDE column, so no reading of it supports a floor of 2x. The page now says
1.3x to 12x, which is the real range, and both ends are in the ledger.

The rounding checks earn their place too. The KV cache line says 120 GB down to
8.4 GB and a 14.2x reduction. Working that out from the two rounded numbers on
the page gives 14.3x. The ledger keeps the unrounded 8.44 GB from the source
table, and 120 / 8.44 is 14.2x, so the page is right and the shortcut is wrong.
