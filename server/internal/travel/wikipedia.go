package travel

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Wiki is the free, keyless "is this place worth seeing?" signal.
//
//   - Wikivoyage's city article carries curated See / Do / Eat listings with
//     coordinates, a sentence of description and usually the admission price.
//   - Wikipedia page views say how well known a place is, so Red Rocks
//     outranks a pocket park by two orders of magnitude. Map providers can
//     tell us neither.
type Wiki struct {
	http *http.Client
}

func NewWiki() *Wiki {
	return &Wiki{http: &http.Client{Timeout: 15 * time.Second}}
}

const (
	wikipediaAPI  = "https://en.wikipedia.org/w/api.php"
	wikivoyageAPI = "https://en.wikivoyage.org/w/api.php"
)

type wikiCacheEntry struct {
	at     time.Time
	places []Place
}

var (
	wikiMu    sync.Mutex
	wikiCache = map[string]wikiCacheEntry{}
	viewCache = map[string]float64{} // Wikipedia title → views/day
)

// Listings returns the Wikivoyage See+Do listings (kind "attraction") or Eat
// listings (kind "food") for a city, within radiusMiles of center, ranked by
// Wikipedia readership. It never errors; on any failure it returns nil.
func (w *Wiki) Listings(ctx context.Context, city string, center LatLng, radiusMiles float64, kind string, limit int) []Place {
	if w == nil || Offline || strings.TrimSpace(city) == "" || !validGeoPoint(center) || limit < 1 {
		return nil
	}
	key := fmt.Sprintf("%s|%s|%.0f|%d", strings.ToLower(city), kind, radiusMiles, limit)
	wikiMu.Lock()
	if e, ok := wikiCache[key]; ok && time.Since(e.at) < 24*time.Hour {
		wikiMu.Unlock()
		return e.places
	}
	wikiMu.Unlock()

	q := url.Values{"action": {"parse"}, "page": {city}, "prop": {"wikitext"}, "redirects": {"1"}, "format": {"json"}}
	var out struct {
		Parse struct {
			Wikitext map[string]string `json:"wikitext"`
		} `json:"parse"`
	}
	if err := w.get(ctx, wikivoyageAPI, q, &out); err != nil {
		return nil
	}
	templates := []string{"see", "do"}
	if kind == "food" {
		templates = []string{"eat"}
	}
	places := ParseWikivoyage(out.Parse.Wikitext["*"], templates, kind)
	kept := places[:0]
	for _, p := range places {
		if haversineMiles(center, p.Location) <= radiusMiles {
			kept = append(kept, p)
		}
	}
	places = kept

	names := make([]string, 0, len(places))
	for _, p := range places {
		names = append(names, p.Name)
	}
	views := w.Popularity(ctx, names)
	for i := range places {
		// A curated listing counts for something even without an article.
		places[i].Popularity = math.Max(views[places[i].Name], 10)
		// "Do" listings are often sports teams whose readership measures
		// fandom, not the outing; keep them in the mix without letting them
		// crowd out the sights.
		if len(places[i].Categories) > 0 && places[i].Categories[0] == "wikivoyage.do" {
			places[i].Popularity = math.Min(places[i].Popularity, 100)
		}
	}
	sort.SliceStable(places, func(i, j int) bool { return places[i].Popularity > places[j].Popularity })
	if len(places) > limit {
		places = places[:limit]
	}
	wikiMu.Lock()
	wikiCache[key] = wikiCacheEntry{at: time.Now(), places: places}
	wikiMu.Unlock()
	return places
}

// Popularity returns average daily Wikipedia page views (last 30 days) for
// each name that has an article, following redirects ("Denver Zoo" works
// even when the article is titled differently). Missing names are absent.
func (w *Wiki) Popularity(ctx context.Context, names []string) map[string]float64 {
	res := map[string]float64{}
	if w == nil || Offline {
		return res
	}
	var ask []string
	wikiMu.Lock()
	for _, n := range names {
		if v, ok := viewCache[n]; ok {
			if v > 0 {
				res[n] = v
			}
		} else {
			ask = append(ask, n)
		}
	}
	wikiMu.Unlock()
	if len(ask) > 100 {
		ask = ask[:100]
	}
	// Small batches: one invalid title fails the whole request, and titles
	// may not contain # < > [ ] | { }.
	for i := 0; i < len(ask); i += 20 {
		batch := ask[i:min(i+20, len(ask))]
		titles := make([]string, 0, len(batch))
		for _, n := range batch {
			if t := safeTitle(n); t != "" {
				titles = append(titles, t)
			}
		}
		if len(titles) == 0 {
			continue
		}
		q := url.Values{"action": {"query"}, "prop": {"pageviews"}, "pvipdays": {"30"}, "redirects": {"1"}, "format": {"json"}, "titles": {strings.Join(titles, "|")}}
		var out struct {
			Error *struct {
				Code string `json:"code"`
			} `json:"error"`
			Query struct {
				Normalized []struct{ From, To string } `json:"normalized"`
				Redirects  []struct{ From, To string } `json:"redirects"`
				Pages      map[string]struct {
					Title     string          `json:"title"`
					Pageviews map[string]*int `json:"pageviews"`
				} `json:"pages"`
			} `json:"query"`
		}
		if w.get(ctx, wikipediaAPI, q, &out) != nil || out.Error != nil {
			continue // not cached, so a transient failure is retried next time
		}
		// Map each asked name back through normalization and redirects.
		to := map[string]string{}
		for _, n := range out.Query.Normalized {
			to[n.From] = n.To
		}
		for _, r := range out.Query.Redirects {
			to[r.From] = r.To
		}
		final := func(n string) string {
			for i := 0; i < 3; i++ {
				next, ok := to[n]
				if !ok {
					break
				}
				n = next
			}
			return n
		}
		byTitle := map[string]float64{}
		for _, p := range out.Query.Pages {
			sum, n := 0, 0
			for _, v := range p.Pageviews {
				if v != nil {
					sum += *v
					n++
				}
			}
			if n > 0 {
				byTitle[p.Title] = float64(sum) / float64(n)
			}
		}
		wikiMu.Lock()
		for _, name := range batch {
			v := byTitle[final(safeTitle(name))]
			viewCache[name] = v
			if v > 0 {
				res[name] = v
			}
		}
		wikiMu.Unlock()
	}
	return res
}

// safeTitle strips the characters MediaWiki forbids in titles.
func safeTitle(s string) string {
	s = strings.Map(func(r rune) rune {
		if strings.ContainsRune("#<>[]|{}", r) {
			return -1
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

func (w *Wiki) get(ctx context.Context, api string, q url.Values, v any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", api+"?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "TrailMix-TPN/1.0 (HackMidwest 2026; trip planner)")
	res, err := w.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("wiki: status %d", res.StatusCode)
	}
	return json.NewDecoder(res.Body).Decode(v)
}

// ---------------------------------------------------------------- wikitext

var (
	wikiLink   = regexp.MustCompile(`\[\[(?:[^\]|]*\|)?([^\]]*)\]\]`)
	extLink    = regexp.MustCompile(`\[https?://\S+\s+([^\]]*)\]`)
	wikiTmpl   = regexp.MustCompile(`\{\{[^{}]*\}\}`)
	htmlTag    = regexp.MustCompile(`<[^>]+>`)
	dollarsRe  = regexp.MustCompile(`\$\s?(\d+(?:\.\d+)?)`)
	freeRe     = regexp.MustCompile(`(?i)\bfree\b`)
	listingTag = regexp.MustCompile(`(?i)\{\{\s*(see|do|eat|listing)\s*(\||\n)`)
)

// ParseWikivoyage extracts the given listing templates ({{see|...}} etc.)
// from a Wikivoyage article into Places. Listings without coordinates are
// skipped; a "price" field like "Adult $27, child $5" becomes PriceUSD 27.
func ParseWikivoyage(wikitext string, templates []string, kind string) []Place {
	want := map[string]bool{}
	for _, t := range templates {
		want[strings.ToLower(t)] = true
	}
	var places []Place
	for _, m := range listingTag.FindAllStringSubmatchIndex(wikitext, -1) {
		name := strings.ToLower(wikitext[m[2]:m[3]])
		start := m[0]
		end := matchBraces(wikitext, start)
		if end < 0 {
			continue
		}
		body := wikitext[start+2 : end-2]
		fields := splitFields(body)
		if name == "listing" {
			name = strings.ToLower(fields["type"])
		}
		if !want[name] {
			continue
		}
		lat, err1 := strconv.ParseFloat(strings.TrimSpace(fields["lat"]), 64)
		lng, err2 := strconv.ParseFloat(strings.TrimSpace(fields["long"]), 64)
		title := cleanWiki(fields["name"])
		if err1 != nil || err2 != nil || title == "" || !validGeoPoint(LatLng{Lat: lat, Lng: lng}) {
			continue
		}
		p := Place{
			Name: title, Category: kind, Location: LatLng{Lat: lat, Lng: lng}, Source: "wikivoyage",
			Categories: []string{"wikivoyage." + name},
			Note:       clip(cleanWiki(fields["content"]), 160),
		}
		if h := cleanWiki(fields["hours"]); h != "" && len(h) <= 60 {
			p.Note = strings.TrimSpace(strings.Join(strings.Fields(p.Note+" · hours "+h), " "))
		}
		price := strings.TrimSpace(fields["price"])
		switch {
		case price == "":
			p.PriceUnknown = true
		case dollarsRe.MatchString(price):
			if v, ok := adultPrice(price); ok {
				p.PriceUSD = v
			} else {
				p.PriceUnknown = true
			}
		case freeRe.MatchString(price):
			p.PriceUSD = 0
		default:
			p.PriceUnknown = true
		}
		places = append(places, p)
	}
	return places
}

// adultPrice picks the per-person figure out of "Adult $27, child $12,
// parking $5": the amount next to "adult" when there is one, else the first.
func adultPrice(price string) (float64, bool) {
	ms := dollarsRe.FindAllStringSubmatchIndex(price, -1)
	if len(ms) == 0 {
		return 0, false
	}
	pick := ms[0]
	lower := strings.ToLower(price)
	for _, m := range ms {
		lo, hi := max(0, m[0]-14), min(len(lower), m[1]+14)
		if strings.Contains(lower[lo:hi], "adult") {
			pick = m
			break
		}
	}
	v, err := strconv.ParseFloat(price[pick[2]:pick[3]], 64)
	if err != nil || v < 0 || v > 500 {
		return 0, false
	}
	return v, true
}

// matchBraces returns the index just past the "}}" that closes the "{{" at i.
func matchBraces(s string, i int) int {
	depth := 0
	for j := i; j+1 < len(s); j++ {
		switch {
		case s[j] == '{' && s[j+1] == '{':
			depth++
			j++
		case s[j] == '}' && s[j+1] == '}':
			depth--
			j++
			if depth == 0 {
				return j + 1
			}
		}
	}
	return -1
}

// splitFields splits "see | name=X | content=[[a|b]] ..." on top-level pipes.
func splitFields(body string) map[string]string {
	out := map[string]string{}
	depthB, depthT := 0, 0
	fieldStart := 0
	flush := func(seg string) {
		if k, v, ok := strings.Cut(seg, "="); ok {
			out[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
		}
	}
	for j := 0; j < len(body); j++ {
		switch {
		case strings.HasPrefix(body[j:], "[["):
			depthB++
			j++
		case strings.HasPrefix(body[j:], "]]"):
			depthB--
			j++
		case strings.HasPrefix(body[j:], "{{"):
			depthT++
			j++
		case strings.HasPrefix(body[j:], "}}"):
			depthT--
			j++
		case body[j] == '|' && depthB == 0 && depthT == 0:
			flush(body[fieldStart:j])
			fieldStart = j + 1
		}
	}
	flush(body[fieldStart:])
	return out
}

func cleanWiki(s string) string {
	s = wikiTmpl.ReplaceAllString(s, "")
	s = wikiLink.ReplaceAllString(s, "$1")
	s = extLink.ReplaceAllString(s, "$1")
	s = htmlTag.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "'''", "")
	s = strings.ReplaceAll(s, "''", "")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	return strings.Join(strings.Fields(s), " ")
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if i := strings.LastIndex(s[:n], ". "); i > n/2 {
		return s[:i+1]
	}
	return s[:n-3] + "..."
}

// MergeNotable adds curated places the map provider missed and lends their
// popularity (and description, price) to the provider's own entries with the
// same name. The result is sorted most popular first; unmatched provider
// places keep their relative order after the known ones.
func MergeNotable(cands, notable []Place) []Place {
	out := append([]Place(nil), cands...)
	for _, n := range notable {
		k := normName(n.Name)
		matched := false
		for i := range out {
			ok := normName(out[i].Name)
			if ok == k || (len(k) >= 8 && len(ok) >= 8 && (strings.Contains(ok, k) || strings.Contains(k, ok))) {
				out[i].Popularity = math.Max(out[i].Popularity, n.Popularity)
				if out[i].Note == "" || len(n.Note) > len(out[i].Note) {
					out[i].Note = n.Note
				}
				if (out[i].PriceUnknown || out[i].PriceEstimated) && !n.PriceUnknown {
					out[i].PriceUSD, out[i].PriceUnknown, out[i].PriceEstimated = n.PriceUSD, false, false
				}
				matched = true
				break
			}
		}
		if !matched {
			out = append(out, n)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Popularity > out[j].Popularity })
	return out
}

func normName(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == ' ' {
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(strings.ReplaceAll(" "+b.String()+" ", " the ", " ")), " ")
}
