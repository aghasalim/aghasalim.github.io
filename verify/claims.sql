-- Recompute every published value in verify/claims.tsv with SQLite's own
-- arithmetic and rounding, independently of the C, Go and JavaScript versions.
-- Prints one row per disagreement, then a summary line per rule. verify.sh
-- requires the disagreement count to be zero.
.mode tabs
.import verify/claims.tsv claims_raw
.headers off

CREATE TEMP VIEW claims AS
SELECT
  "id" AS id,
  "repo" AS repo,
  "rule" AS rule,
  CAST("a" AS REAL) AS a,
  CASE WHEN "b" = '-' THEN NULL ELSE CAST("b" AS REAL) END AS b,
  CAST("published" AS REAL) AS published
FROM claims_raw;

CREATE TEMP VIEW recomputed AS
SELECT id, repo, rule, published,
  CASE
    WHEN rule = 'copy'   THEN a
    WHEN rule = 'round0' THEN round(a, 0)
    WHEN rule = 'round1' THEN round(a, 1)
    WHEN rule = 'round2' THEN round(a, 2)
    WHEN rule = 'ratio0' THEN round(a / b, 0)
    WHEN rule = 'ratio1' THEN round(a / b, 1)
    WHEN rule = 'diff4'  THEN round(a - b, 4)
  END AS got
FROM claims;

.mode csv
SELECT 'DISAGREE', id, rule, published, got
FROM recomputed
WHERE got IS NULL OR abs(got - published) > 1e-12;

SELECT 'ROWS', count(*) FROM recomputed;
SELECT 'BADROWS', count(*) FROM recomputed
WHERE got IS NULL OR abs(got - published) > 1e-12;
SELECT 'BYRULE', rule, count(*) FROM recomputed GROUP BY rule ORDER BY rule;
SELECT 'BYREPO', repo, count(*) FROM recomputed GROUP BY repo ORDER BY repo;
SELECT 'MAXERR', max(abs(got - published)) FROM recomputed;
