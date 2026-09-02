# aghasalim.github.io

My personal site. One hand written page, no build step, no framework, served by
GitHub Pages straight from this branch. Every figure quoted on the page is
recomputed from the source it was taken from by the checks in `verify/`, and CI
fails the build if any of them disagrees with the page.

```
index.html    the page, with its CSS and JSON-LD inline
sitemap.xml   one URL
robots.txt    allow everything, point at the sitemap
.nojekyll     skip the Jekyll pass
```

Open `index.html` in a browser to see it. There is nothing to install.
