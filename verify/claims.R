# Recompute every published value in verify/claims.tsv using R's own rounding.
#
# Same arithmetic as the SQL, C, Go and JS versions. R's round() uses IEEE 754
# banker's rounding (round half to even), which differs from C's round() on
# half values -- if they still agree, the published values are not on a
# rounding boundary.
#
# Run: Rscript verify/claims.R <repo root>

args <- commandArgs(trailingOnly = TRUE)
root <- if (length(args) > 0) args[1] else "."

tsv <- read.delim(file.path(root, "verify", "claims.tsv"),
                  stringsAsFactors = FALSE, colClasses = "character")
bad <- 0
worst <- 0

for (i in seq_len(nrow(tsv))) {
  r    <- tsv[i, ]
  rule <- r$rule
  a    <- as.numeric(r$a)
  b    <- if (r$b == "-") NA_real_ else as.numeric(r$b)
  pub  <- as.numeric(r$published)

  got <- switch(rule,
    copy   = a,
    round0 = round(a, 0),
    round1 = round(a, 1),
    round2 = round(a, 2),
    ratio0 = round(a / b, 0),
    ratio1 = round(a / b, 1),
    diff4  = round(a - b, 4),
    { cat("unknown rule:", rule, "\n"); NA_real_ }
  )

  if (is.na(got) || abs(got - pub) > 1e-12) {
    cat(sprintf("DISAGREE %s: rule=%s published=%s got=%s\n",
                r$id, rule, r$published, as.character(got)))
    bad <- bad + 1
  } else {
    err <- abs(got - pub)
    if (err > worst) worst <- err
  }
}

if (bad > 0) {
  cat("R:", bad, "disagreement(s)\n")
  quit(status = 1)
}
cat(sprintf("R: %d values reproduced, largest error %.1e\n", nrow(tsv), worst))
