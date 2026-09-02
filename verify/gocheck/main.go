// Structural validation of the site and of the claims ledger.
//
// The page is hand written HTML with no build step, so a mistyped tag, a link
// to a file that is not in the repository or a sitemap that disagrees with the
// canonical URL would ship without anything complaining. This walks every
// tracked file and refuses all of that. It also recomputes the published values
// a second time, independently of the C, SQL and JavaScript versions, and
// checks that the claim count quoted in the README is the real row count.
package main

import (
	"bufio"
	"encoding/xml"
	"flag"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var problems []string

func bad(format string, a ...interface{}) {
	problems = append(problems, fmt.Sprintf(format, a...))
}

var voidTags = map[string]bool{
	"meta": true, "link": true, "br": true, "img": true, "hr": true,
	"input": true, "source": true, "area": true, "base": true, "col": true,
	"embed": true, "param": true, "track": true, "wbr": true,
}

var (
	tagRe      = regexp.MustCompile(`(?s)<(/?)([a-zA-Z][a-zA-Z0-9]*)([^>]*)>`)
	commentRe  = regexp.MustCompile(`(?s)<!--.*?-->`)
	scriptRe   = regexp.MustCompile(`(?s)<script[^>]*>.*?</script>`)
	styleRe    = regexp.MustCompile(`(?s)<style[^>]*>.*?</style>`)
	attrRe     = regexp.MustCompile(`(href|src|id)="([^"]*)"`)
	locRe      = regexp.MustCompile(`<loc>([^<]+)</loc>`)
	canonicalRe = regexp.MustCompile(`<link rel="canonical" href="([^"]+)"`)
	countRe    = regexp.MustCompile("claims\\.tsv`? records (\\d+)\\s+claims")
)

// checkStructure walks the tag stream of index.html and requires every non void
// element to be closed in order.
func checkStructure(html string) {
	body := commentRe.ReplaceAllString(html, " ")
	body = scriptRe.ReplaceAllString(body, " ")
	body = styleRe.ReplaceAllString(body, " ")

	var stack []string
	for _, m := range tagRe.FindAllStringSubmatch(body, -1) {
		closing, name, attrs := m[1] == "/", strings.ToLower(m[2]), m[3]
		if voidTags[name] || strings.HasSuffix(strings.TrimSpace(attrs), "/") {
			continue
		}
		if !closing {
			stack = append(stack, name)
			continue
		}
		if len(stack) == 0 {
			bad("index.html: </%s> with nothing open", name)
			continue
		}
		top := stack[len(stack)-1]
		if top != name {
			bad("index.html: </%s> closes <%s>", name, top)
		}
		stack = stack[:len(stack)-1]
	}
	if len(stack) != 0 {
		bad("index.html: unclosed tags %v", stack)
	}
}

// checkRefs requires every id to be unique, every fragment link to point at an
// id that exists, every relative href or src to name a file in the repository,
// and every absolute one to be a parseable https URL.
func checkRefs(root, html string) {
	ids := map[string]bool{}
	var frags []string
	for _, m := range attrRe.FindAllStringSubmatch(html, -1) {
		kind, val := m[1], m[2]
		if kind == "id" {
			if ids[val] {
				bad("index.html: duplicate id %q", val)
			}
			ids[val] = true
			continue
		}
		switch {
		case strings.HasPrefix(val, "#"):
			frags = append(frags, strings.TrimPrefix(val, "#"))
		case strings.Contains(val, "://") || strings.HasPrefix(val, "//"):
			u, err := url.Parse(val)
			if err != nil {
				bad("index.html: %s=%q does not parse: %v", kind, val, err)
			} else if u.Scheme != "https" {
				bad("index.html: %s=%q is not https", kind, val)
			}
		case strings.HasPrefix(val, "mailto:"):
		default:
			p := filepath.Join(root, strings.TrimPrefix(strings.SplitN(val, "#", 2)[0], "/"))
			if _, err := os.Stat(p); err != nil {
				bad("index.html: %s=%q does not resolve to a file in the repository", kind, val)
			}
		}
	}
	for _, f := range frags {
		if !ids[f] {
			bad("index.html: link to #%s but no element has that id", f)
		}
	}
}

func checkSiteMeta(root, html string) {
	sitemap, err := os.ReadFile(filepath.Join(root, "sitemap.xml"))
	if err != nil {
		bad("cannot read sitemap.xml: %v", err)
		return
	}
	var doc interface{}
	if err := xml.Unmarshal(sitemap, &doc); err != nil {
		bad("sitemap.xml is not well formed XML: %v", err)
	}
	loc := locRe.FindStringSubmatch(string(sitemap))
	canon := canonicalRe.FindStringSubmatch(html)
	if loc == nil {
		bad("sitemap.xml has no <loc>")
		return
	}
	if canon == nil {
		bad("index.html has no canonical link")
		return
	}
	if loc[1] != canon[1] {
		bad("sitemap loc %q and canonical %q disagree", loc[1], canon[1])
	}
	robots, err := os.ReadFile(filepath.Join(root, "robots.txt"))
	if err != nil {
		bad("cannot read robots.txt: %v", err)
		return
	}
	want := strings.TrimSuffix(loc[1], "/") + "/sitemap.xml"
	if !strings.Contains(string(robots), "Sitemap: "+want) {
		bad("robots.txt does not point at %s", want)
	}
}

type claim struct {
	id, rule  string
	a, b, pub float64
	hasB      bool
	phrase    string
}

func readClaims(root string) []claim {
	f, err := os.Open(filepath.Join(root, "verify", "claims.tsv"))
	if err != nil {
		bad("cannot read claims.tsv: %v", err)
		return nil
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		bad("claims.tsv is empty")
		return nil
	}
	header := strings.Split(sc.Text(), "\t")
	col := map[string]int{}
	for i, h := range header {
		if _, dup := col[h]; dup {
			bad("claims.tsv: duplicate column %q", h)
		}
		col[h] = i
	}
	for _, need := range []string{"id", "repo", "source", "rule", "a", "b", "published", "phrase"} {
		if _, ok := col[need]; !ok {
			bad("claims.tsv: missing column %q", need)
			return nil
		}
	}

	num := func(id, name, s string) (float64, bool) {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			bad("claims.tsv %s: column %s is not a number: %q", id, name, s)
			return 0, false
		}
		if math.IsNaN(v) || math.IsInf(v, 0) {
			bad("claims.tsv %s: column %s is NaN or Inf", id, name)
			return 0, false
		}
		return v, true
	}

	seen := map[string]bool{}
	var out []claim
	line := 1
	for sc.Scan() {
		line++
		text := sc.Text()
		if strings.TrimSpace(text) == "" {
			continue
		}
		fields := strings.Split(text, "\t")
		if len(fields) != len(header) {
			bad("claims.tsv line %d: %d fields, header has %d", line, len(fields), len(header))
			continue
		}
		for i, v := range fields {
			if strings.TrimSpace(v) == "" {
				bad("claims.tsv line %d: column %s is empty", line, header[i])
			}
		}
		c := claim{id: fields[col["id"]], rule: fields[col["rule"]], phrase: fields[col["phrase"]]}
		if seen[c.id] {
			bad("claims.tsv: duplicate id %q", c.id)
		}
		seen[c.id] = true
		ok1, ok2 := false, false
		c.a, ok1 = num(c.id, "a", fields[col["a"]])
		c.pub, ok2 = num(c.id, "published", fields[col["published"]])
		if b := fields[col["b"]]; b != "-" {
			c.b, c.hasB = num(c.id, "b", b)
		}
		if ok1 && ok2 {
			out = append(out, c)
		}
	}
	return out
}

func roundTo(x float64, digits int) float64 {
	f := math.Pow(10, float64(digits))
	return math.Round(x*f) / f
}

// checkClaims recomputes each published value and requires its phrase to appear
// exactly once in the visible text of the page.
func checkClaims(html string, claims []claim) (int, float64, string) {
	text := scriptRe.ReplaceAllString(html, " ")
	text = styleRe.ReplaceAllString(text, " ")
	text = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(text, " ")
	text = strings.Join(strings.Fields(text), " ")

	worst, worstID, okCount := 0.0, "none", 0
	for _, c := range claims {
		if n := strings.Count(text, c.phrase); n != 1 {
			bad("%s: phrase appears %d times in index.html, want 1: %s", c.id, n, c.phrase)
		}
		var got float64
		switch {
		case c.rule == "copy":
			got = c.a
		case strings.HasPrefix(c.rule, "round"):
			d, _ := strconv.Atoi(strings.TrimPrefix(c.rule, "round"))
			got = roundTo(c.a, d)
		case strings.HasPrefix(c.rule, "ratio"):
			if !c.hasB || c.b == 0 {
				bad("%s: rule %s needs a non zero b", c.id, c.rule)
				continue
			}
			d, _ := strconv.Atoi(strings.TrimPrefix(c.rule, "ratio"))
			got = roundTo(c.a/c.b, d)
		case strings.HasPrefix(c.rule, "diff"):
			if !c.hasB {
				bad("%s: rule %s needs a b", c.id, c.rule)
				continue
			}
			d, _ := strconv.Atoi(strings.TrimPrefix(c.rule, "diff"))
			got = roundTo(c.a-c.b, d)
		default:
			bad("%s: unknown rule %s", c.id, c.rule)
			continue
		}
		err := math.Abs(got - c.pub)
		if err > math.Abs(c.pub)*1e-12+1e-12 {
			bad("%s: rule %s gives %g, page publishes %g", c.id, c.rule, got, c.pub)
			continue
		}
		okCount++
		if err > worst {
			worst, worstID = err, c.id
		}
	}
	return okCount, worst, worstID
}

// checkREADMECount requires the claim count quoted in the README to be the real
// number of rows in the ledger.
func checkREADMECount(root string, rows int) {
	b, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		bad("cannot read README.md: %v", err)
		return
	}
	m := countRe.FindStringSubmatch(string(b))
	if m == nil {
		bad("README.md does not state how many claims the ledger records")
		return
	}
	n, _ := strconv.Atoi(m[1])
	if n != rows {
		bad("README.md says the ledger records %d claims, it has %d rows", n, rows)
	}
}

func main() {
	root := flag.String("root", "../..", "repository root")
	flag.Parse()

	htmlBytes, err := os.ReadFile(filepath.Join(*root, "index.html"))
	if err != nil {
		fmt.Println("FAIL cannot read index.html:", err)
		os.Exit(1)
	}
	html := string(htmlBytes)

	checkStructure(html)
	checkRefs(*root, html)
	checkSiteMeta(*root, html)
	claims := readClaims(*root)
	okCount, worst, worstID := checkClaims(html, claims)
	checkREADMECount(*root, len(claims))

	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Println("FAIL", p)
		}
		fmt.Printf("Go: %d problems\n", len(problems))
		os.Exit(1)
	}
	fmt.Printf("Go: index.html well formed, every link and asset resolves, "+
		"sitemap and robots agree with the canonical URL, %d claims reproduced, "+
		"largest error %.1e on %s\n", okCount, worst, worstID)
}
