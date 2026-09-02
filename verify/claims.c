/* Recompute every published value in verify/claims.tsv in C.
 *
 * Same arithmetic as the SQL, Go and JavaScript versions, written against a
 * different rounding implementation and a different TSV reader. Columns are
 * resolved by name from the header row, so reordering the file cannot silently
 * change what is compared.
 *
 * Build: cc -std=c99 -O2 -o claimc verify/claims.c -lm
 * Run:   ./claimc <repo root>
 */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <math.h>

#define MAXCOL 32
#define MAXLINE 4096

static int find_col(char *header[], int n, const char *name) {
    for (int i = 0; i < n; i++) {
        if (strcmp(header[i], name) == 0) return i;
    }
    fprintf(stderr, "claims.tsv has no column named %s\n", name);
    exit(1);
}

/* Split a tab separated line in place. Returns the field count. */
static int split(char *line, char *out[], int max) {
    int n = 0;
    char *p = line;
    out[n++] = p;
    while (*p) {
        if (*p == '\t') {
            *p = '\0';
            if (n >= max) { fprintf(stderr, "too many columns\n"); exit(1); }
            out[n++] = p + 1;
        }
        p++;
    }
    return n;
}

static double round_to(double x, int digits) {
    double f = pow(10.0, (double)digits);
    return round(x * f) / f;
}

int main(int argc, char **argv) {
    const char *root = argc > 1 ? argv[1] : ".";
    char path[1024];
    snprintf(path, sizeof path, "%s/verify/claims.tsv", root);
    FILE *f = fopen(path, "r");
    if (!f) { fprintf(stderr, "cannot open %s\n", path); return 1; }

    char line[MAXLINE];
    if (!fgets(line, sizeof line, f)) { fprintf(stderr, "empty claims.tsv\n"); return 1; }
    line[strcspn(line, "\r\n")] = '\0';
    char *header[MAXCOL];
    int ncol = split(line, header, MAXCOL);

    int c_id = find_col(header, ncol, "id");
    int c_rule = find_col(header, ncol, "rule");
    int c_a = find_col(header, ncol, "a");
    int c_b = find_col(header, ncol, "b");
    int c_pub = find_col(header, ncol, "published");

    int rows = 0, bad = 0;
    double worst = 0.0;
    char worst_id[128] = "none";

    while (fgets(line, sizeof line, f)) {
        line[strcspn(line, "\r\n")] = '\0';
        if (line[0] == '\0') continue;
        char *fld[MAXCOL];
        int n = split(line, fld, MAXCOL);
        if (n != ncol) {
            printf("FAIL ragged row %d: %d fields, header has %d\n", rows + 1, n, ncol);
            bad++; rows++; continue;
        }
        rows++;

        char *end;
        double a = strtod(fld[c_a], &end);
        if (end == fld[c_a] || *end != '\0') {
            printf("FAIL %s: column a is not a number: %s\n", fld[c_id], fld[c_a]); bad++; continue;
        }
        double pub = strtod(fld[c_pub], &end);
        if (end == fld[c_pub] || *end != '\0') {
            printf("FAIL %s: column published is not a number: %s\n", fld[c_id], fld[c_pub]); bad++; continue;
        }
        int has_b = strcmp(fld[c_b], "-") != 0;
        double b = 0.0;
        if (has_b) {
            b = strtod(fld[c_b], &end);
            if (end == fld[c_b] || *end != '\0') {
                printf("FAIL %s: column b is not a number: %s\n", fld[c_id], fld[c_b]); bad++; continue;
            }
        }
        if (!isfinite(a) || !isfinite(pub) || (has_b && !isfinite(b))) {
            printf("FAIL %s: non finite value in the row\n", fld[c_id]); bad++; continue;
        }

        const char *rule = fld[c_rule];
        double got;
        if (strcmp(rule, "copy") == 0) {
            got = a;
        } else if (strncmp(rule, "round", 5) == 0) {
            got = round_to(a, atoi(rule + 5));
        } else if (strncmp(rule, "ratio", 5) == 0) {
            if (!has_b || b == 0.0) {
                printf("FAIL %s: rule %s needs a non zero b\n", fld[c_id], rule); bad++; continue;
            }
            got = round_to(a / b, atoi(rule + 5));
        } else if (strncmp(rule, "diff", 4) == 0) {
            if (!has_b) {
                printf("FAIL %s: rule %s needs a b\n", fld[c_id], rule); bad++; continue;
            }
            got = round_to(a - b, atoi(rule + 4));
        } else {
            printf("FAIL %s: unknown rule %s\n", fld[c_id], rule); bad++; continue;
        }

        double err = fabs(got - pub);
        double tol = fabs(pub) * 1e-12 + 1e-12;
        if (err > tol) {
            printf("FAIL %s: rule %s gives %.10g, page publishes %.10g\n", fld[c_id], rule, got, pub);
            bad++;
        } else if (err > worst) {
            worst = err;
            snprintf(worst_id, sizeof worst_id, "%s", fld[c_id]);
        }
    }
    fclose(f);

    if (rows == 0) { printf("FAIL claims.tsv has no rows\n"); return 1; }
    if (bad) { printf("C: %d of %d rows disagree\n", bad, rows); return 1; }
    printf("C: %d published values reproduced, largest error %.1e on %s\n", rows, worst, worst_id);
    return 0;
}
