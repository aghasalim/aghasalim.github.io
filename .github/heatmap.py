"""Rewrite the contribution heatmap in index.html from both GitHub accounts.

Sums the two contribution calendars day by day over the last year (GitHub's
default range, ending today) and replaces the <section class="heat"> block.
Needs GITHUB_TOKEN in the environment. Stdlib only.
"""
import datetime, json, os, re, urllib.request
from collections import Counter

USERS = ["aghasalim", "MustafazadaAghasalim"]
QUERY = 'query($u:String!){user(login:$u){contributionsCollection{contributionCalendar{totalContributions weeks{contributionDays{date contributionCount}}}}}}'


def calendar(user):
    req = urllib.request.Request(
        "https://api.github.com/graphql",
        data=json.dumps({"query": QUERY, "variables": {"u": user}}).encode(),
        headers={"Authorization": "bearer " + os.environ["GITHUB_TOKEN"]},
    )
    body = json.load(urllib.request.urlopen(req))
    if "errors" in body:
        raise SystemExit(f"{user}: {body['errors']}")
    return body["data"]["user"]["contributionsCollection"]["contributionCalendar"]


def level(n):
    return sum(n >= t for t in (1, 4, 10, 25))


def render(weeks, total):
    months, cells = [], []
    for col, week in enumerate(weeks, 1):
        for date, n in week:
            if date.endswith("-01"):
                m = datetime.date.fromisoformat(date).strftime("%b")
                months.append(f'<span style="grid-column:{col}">{m}</span>')
            cells.append(f'<i class="l{level(n)}" title="{n} on {date}"></i>')
    return (
        f'<section class="heat" aria-label="GitHub contributions in the last year" style="--w:{len(weeks)}">\n'
        f'  <div class="months">{"".join(months)}</div>\n'
        f'  <div class="grid">{"".join(cells)}</div>\n'
        f'  <div class="legend"><span>{total:,} contributions in the last year, across my two GitHub accounts, '
        + " and ".join(f'<a href="https://github.com/{u}">{u}</a>' for u in USERS)
        + '.</span><span class="scale">Less <i class="l0"></i><i class="l1"></i><i class="l2"></i><i class="l3"></i><i class="l4"></i> More</span></div>\n'
        "</section>"
    )


def main():
    cals = [calendar(u) for u in USERS]
    counts = Counter()
    for cal in cals:
        for w in cal["weeks"]:
            for d in w["contributionDays"]:
                counts[d["date"]] += d["contributionCount"]
    # both calendars cover the same days; the first one gives the week layout
    weeks = [[(d["date"], counts[d["date"]]) for d in w["contributionDays"]] for w in cals[0]["weeks"]]
    total = sum(n for w in weeks for _, n in w)
    assert total == sum(c["totalContributions"] for c in cals), "merged total differs from the two accounts' totals"

    path = os.path.join(os.path.dirname(__file__), "..", "index.html")
    html = open(path, encoding="utf-8").read()
    new, hits = re.subn(r'<section class="heat".*?</section>', lambda _: render(weeks, total), html, count=1, flags=re.S)
    assert hits == 1, "heatmap section not found in index.html"
    open(path, "w", encoding="utf-8").write(new)
    print(f"{total:,} contributions, {weeks[0][0][0]} to {weeks[-1][-1][0]}")


if __name__ == "__main__":
    main()
