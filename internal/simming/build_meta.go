package simming

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/healsim"
	"github.com/tm-LBenson/lamda-engine/internal/wowspec"
)

type BuildPlayer struct {
	Rank    int    `json:"rank"`
	Name    string `json:"name"`
	Realm   string `json:"realm"`
	Details string `json:"details"`
	Rating  int    `json:"rating"`
	URL     string `json:"url"`
}
type BuildChoice struct {
	Name    string  `json:"name"`
	Count   int     `json:"count"`
	Percent float64 `json:"percent"`
	URL     string  `json:"url,omitempty"`
}
type BuildStat struct {
	Key     string  `json:"key"`
	Name    string  `json:"name"`
	Rank    int     `json:"rank"`
	Percent float64 `json:"percent"`
	Rating  int     `json:"rating"`
}
type BuildGear struct {
	ID   int    `json:"id"`
	Slot string `json:"slot"`
	Name string `json:"name"`
	Rank int    `json:"rank"`
	BIS  bool   `json:"bis"`
}
type BuildReport struct {
	SpecName  string                   `json:"specName"`
	Class     string                   `json:"class"`
	Source    string                   `json:"source"`
	URL       string                   `json:"url"`
	Mode      string                   `json:"mode"`
	Spec      int                      `json:"spec"`
	FetchedAt time.Time                `json:"fetchedAt"`
	UpdatedAt time.Time                `json:"updatedAt"`
	Season    string                   `json:"season"`
	Patch     string                   `json:"patch"`
	Sample    int                      `json:"sample"`
	Cached    bool                     `json:"cached"`
	Stale     bool                     `json:"stale"`
	Players   []BuildPlayer            `json:"players"`
	Heroes    []BuildChoice            `json:"heroes"`
	Talents   []BuildChoice            `json:"talents"`
	Stats     []BuildStat              `json:"stats"`
	Gear      []BuildGear              `json:"gear"`
	Sources   []healsim.Source         `json:"sources"`
	Consensus []healsim.Recommendation `json:"consensus"`
	Warnings  []string                 `json:"warnings"`
}

var buildModes = map[string]bool{"m+": true, "solo": true, "2v2": true, "3v3": true, "blitz": true, "rbg": true}
var anchorRE = regexp.MustCompile(`(?s)<a\s+([^>]+)>(.*?)</a>`)
var divRE = regexp.MustCompile(`(?s)<div\b[^>]*>([^<]*)</div>`)
var tagRE = regexp.MustCompile(`<[^>]*>`)
var spaceRE = regexp.MustCompile(`\s+`)
var timeRE = regexp.MustCompile(`<time\b[^>]*datetime="([^"]+)"`)
var countRE = regexp.MustCompile(`class="guide-talent-count"[^>]*>\s*(\d+)`)
var seasonRE = regexp.MustCompile(`Midnight Season (\d+)`)
var patchRE = regexp.MustCompile(`Patch (\d+\.\d+(?:\.\d+)?)`)
var heroRE = regexp.MustCompile(`>\s*(\d+) [^<>]+\s*<`)
var statSectionRE = regexp.MustCompile(`(?s)<section\b[^>]*id="stat-priority"[^>]*>(.*?)</section>`)
var gearSectionRE = regexp.MustCompile(`(?s)<section\b[^>]*id="gear"[^>]*>(.*?)</section>`)
var statChartRE = regexp.MustCompile(`(?s)<li\b[^>]*class="[^"]*guide-stats-chart-item[^"]*"[^>]*>(.*?)</li>`)
var statPriorityRE = regexp.MustCompile(`(?s)<h4[^>]*>\s*Stat priority\s*</h4>\s*<ol\b[^>]*>(.*?)</ol>`)
var listItemRE = regexp.MustCompile(`(?s)<li\b[^>]*>(.*?)</li>`)
var gearSlotRE = regexp.MustCompile(`(?s)<h3[^>]*>(.*?)</h3>\s*<ol\b[^>]*>(.*?)</ol>`)
var heading4RE = regexp.MustCompile(`(?s)<h4\b[^>]*>(.*?)</h4>`)
var wowheadItemRE = regexp.MustCompile(`^https://www\.wowhead\.com/item=(\d+)$`)

func textHTML(s string) string {
	return strings.TrimSpace(spaceRE.ReplaceAllString(html.UnescapeString(tagRE.ReplaceAllString(s, " ")), " "))
}
func attribute(s, key string) string {
	r := regexp.MustCompile(`(?:^|\s)` + regexp.QuoteMeta(key) + `="([^"]*)"`)
	m := r.FindStringSubmatch(s)
	if len(m) > 1 {
		return html.UnescapeString(m[1])
	}
	return ""
}

func buildStatKey(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "critical strike", "crit":
		return "crit"
	case "haste":
		return "haste"
	case "mastery":
		return "mastery"
	case "versatility", "vers":
		return "vers"
	}
	return ""
}

func parseBuildStats(page string) []BuildStat {
	section := statSectionRE.FindStringSubmatch(page)
	if len(section) < 2 {
		return nil
	}
	values := map[string]BuildStat{}
	for _, match := range statChartRE.FindAllStringSubmatch(section[1], -1) {
		text := textHTML(match[1])
		for _, name := range []string{"Critical Strike", "Haste", "Mastery", "Versatility"} {
			marker := "% " + name + " +"
			before, after, ok := strings.Cut(text, marker)
			if !ok {
				continue
			}
			fields := strings.Fields(after)
			if len(fields) == 0 {
				continue
			}
			percent, percentErr := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(before), ",", ""), 64)
			rating, ratingErr := strconv.Atoi(strings.ReplaceAll(fields[0], ",", ""))
			key := buildStatKey(name)
			if percentErr == nil && ratingErr == nil && percent >= 0 && percent <= 100 && rating >= 0 && rating <= 100000 && key != "" {
				values[key] = BuildStat{Key: key, Name: name, Percent: percent, Rating: rating}
			}
		}
	}
	priority := statPriorityRE.FindStringSubmatch(section[1])
	if len(priority) < 2 {
		return nil
	}
	var out []BuildStat
	seen := map[string]bool{}
	for _, match := range listItemRE.FindAllStringSubmatch(priority[1], -1) {
		name := textHTML(match[1])
		key := buildStatKey(name)
		value, ok := values[key]
		if key == "" || !ok || seen[key] {
			continue
		}
		seen[key] = true
		value.Rank = len(out) + 1
		out = append(out, value)
	}
	if len(out) != 4 {
		return nil
	}
	return out
}

func parseBuildGear(page string) []BuildGear {
	section := gearSectionRE.FindStringSubmatch(page)
	if len(section) < 2 {
		return nil
	}
	var out []BuildGear
	for _, slotMatch := range gearSlotRE.FindAllStringSubmatch(section[1], -1) {
		slot := textHTML(slotMatch[1])
		if slot == "" || len(slot) > 40 {
			continue
		}
		bestCount := 1
		if slot == "Rings" || slot == "Trinkets" {
			bestCount = 2
		}
		rank := 0
		for _, match := range anchorRE.FindAllStringSubmatch(slotMatch[2], -1) {
			if !strings.Contains(attribute(match[1], "class"), "list-item") {
				continue
			}
			itemMatch := wowheadItemRE.FindStringSubmatch(attribute(match[1], "href"))
			nameMatch := heading4RE.FindStringSubmatch(match[2])
			if len(itemMatch) < 2 || len(nameMatch) < 2 {
				continue
			}
			id, _ := strconv.Atoi(itemMatch[1])
			name := textHTML(nameMatch[1])
			if id <= 0 || name == "" || len(name) > 160 {
				continue
			}
			rank++
			out = append(out, BuildGear{ID: id, Slot: slot, Name: name, Rank: rank, BIS: rank <= bestCount})
		}
	}
	return out
}

// Only the public summary page is read. No hidden APIs, profile crawling or
// login is needed. A changed page must fail parsing, not silently emit guesses.
func parseBuildPage(body []byte, specID int, mode string, now time.Time) (*BuildReport, error) {
	spec, ok := wowspec.Lookup(specID)
	if !ok || !buildModes[mode] {
		return nil, fmt.Errorf("unsupported build specialization or mode")
	}
	r := &BuildReport{SpecName: spec.Label(), Class: spec.Class, Source: "Murlok.io", URL: "https://murlok.io" + spec.Path() + "/" + mode, Spec: specID, Mode: mode, FetchedAt: now, Sources: healsim.SourcesFor(specID), Consensus: healsim.ConsensusFor(specID)}
	for i := range r.Sources {
		if r.Sources[i].ID == "murlok" {
			r.Sources[i].URL = r.URL
		}
	}
	s := string(body)
	r.Stats = parseBuildStats(s)
	r.Gear = parseBuildGear(s)
	tm := timeRE.FindStringSubmatch(s)
	if len(tm) > 1 {
		r.UpdatedAt, _ = time.Parse(time.RFC3339, tm[1])
	}
	if m := seasonRE.FindStringSubmatch(s); len(m) > 1 {
		r.Season = "Midnight Season " + m[1]
	}
	if m := patchRE.FindStringSubmatch(s); len(m) > 1 {
		r.Patch = m[1]
	}
	// Hero names come from this spec's intro links, including irregular plurals
	// such as "Keepers of the Grove"; the icon label provides the singular name.
	for _, m := range anchorRE.FindAllStringSubmatch(s, -1) {
		class, href := attribute(m[1], "class"), attribute(m[1], "href")
		if !strings.Contains(class, "vi-intro-icon-button") || !strings.HasPrefix(href, spec.Path()+"/") {
			continue
		}
		hit := heroRE.FindStringSubmatch(m[2])
		name := strings.TrimSuffix(attribute(m[2], "alt"), " icon")
		if len(hit) < 2 || name == "" || len(name) > 100 {
			continue
		}
		count, _ := strconv.Atoi(hit[1])
		if count < 0 || count > 100 {
			return nil, fmt.Errorf("invalid hero sample")
		}
		r.Heroes = append(r.Heroes, BuildChoice{Name: name, Count: count})
		r.Sample += count
	}
	seen := map[string]bool{}
	for _, m := range anchorRE.FindAllStringSubmatch(s, -1) {
		class := attribute(m[1], "class")
		href := attribute(m[1], "href")
		if strings.Contains(class, "top-ratings-item") && strings.HasPrefix(href, "/character/") {
			if len(r.Players) >= 10 {
				continue
			}
			parts := []string{}
			for _, v := range divRE.FindAllStringSubmatch(m[2], -1) {
				t := textHTML(v[1])
				if t != "" {
					parts = append(parts, t)
				}
			}
			if len(parts) != 5 {
				continue
			}
			rank, _ := strconv.Atoi(parts[0])
			rating, _ := strconv.Atoi(parts[4])
			if rank < 1 || rating <= 0 || len(parts[1]) > 128 || len(parts[2]) > 128 || len(href) > 512 {
				continue
			}
			r.Players = append(r.Players, BuildPlayer{rank, parts[1], parts[2], parts[3], rating, "https://murlok.io" + href})
		}
		if strings.Contains(class, "guide-talent ") && strings.HasPrefix(href, spec.Path()+"/talents#") && !seen[href] {
			match := countRE.FindStringSubmatch(m[2])
			name := attribute(m[2], "alt")
			if len(match) < 2 || name == "" || len(name) > 100 {
				continue
			}
			count, _ := strconv.Atoi(match[1])
			if count < 0 || count > r.Sample {
				continue
			}
			seen[href] = true
			r.Talents = append(r.Talents, BuildChoice{Name: name, Count: count, URL: "https://murlok.io" + href})
		}
	}
	if r.Sample < 1 || r.Sample > 100 || len(r.Players) < 1 || r.Players[0].Rank != 1 || r.UpdatedAt.IsZero() || r.UpdatedAt.After(now.Add(24*time.Hour)) || r.Season == "" || len(r.Talents) < 10 || len(r.Stats) != 4 || len(r.Gear) < 10 {
		return nil, fmt.Errorf("Murlok's page did not contain a complete recognizable specialization snapshot; previous data was kept")
	}
	for i := range r.Heroes {
		r.Heroes[i].Percent = 100 * float64(r.Heroes[i].Count) / float64(r.Sample)
	}
	for i := range r.Talents {
		r.Talents[i].Percent = 100 * float64(r.Talents[i].Count) / float64(r.Sample)
	}
	r.Stale = now.Sub(r.UpdatedAt) > 48*time.Hour
	r.Warnings = []string{"Popularity describes this ranked sample, not a simulated performance improvement. Talent pick rates do not constitute a legal complete build; open a player's source page for their import code.", "BiS labels mean the most frequently equipped item in each slot among this top-player sample (the top two for rings and trinkets). Your completed character simulation remains the personal upgrade result.", "Guide recommendations are PvE only. PvP samples are kept separate and do not change healing simulation priorities."}
	if r.Patch != "12.1" && r.Patch != "12.1.0" {
		r.Warnings = append(r.Warnings, "Leaderboard patch differs from the 12.1 guide review. Review strategies before comparing.")
	}
	return r, nil
}

func fetchBuilds(ctx context.Context, specID int, mode, cacheDir string) (*BuildReport, error) {
	spec, validSpec := wowspec.Lookup(specID)
	if !validSpec || !buildModes[mode] {
		return nil, fmt.Errorf("unsupported build mode")
	}
	path := filepath.Join(cacheDir, strconv.Itoa(specID)+"-"+strings.ReplaceAll(mode, "+", "plus")+".json")
	now := time.Now().UTC()
	var cached BuildReport
	if data, err := readBounded(path, 256*1024); err == nil {
		if json.Unmarshal(data, &cached) != nil || cached.Spec != specID || cached.Mode != mode || cached.URL != "https://murlok.io"+spec.Path()+"/"+mode || len(cached.Stats) != 4 || len(cached.Gear) < 10 {
			cached = BuildReport{}
		}
	}
	if !cached.FetchedAt.IsZero() && !cached.FetchedAt.After(now) && now.Sub(cached.FetchedAt) < 8*time.Hour {
		cached.Cached = true
		cached.Stale = now.Sub(cached.UpdatedAt) > 48*time.Hour
		return &cached, nil
	}
	r, err := downloadBuilds(ctx, specID, mode, now)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !cached.FetchedAt.IsZero() {
			cached.Cached = true
			cached.Stale = true
			cached.Warnings = append(cached.Warnings, "Refresh failed; showing the dated cached snapshot. "+err.Error())
			return &cached, nil
		}
		return nil, err
	}
	if err = writeJSON(path, r); err != nil {
		r.Warnings = append(r.Warnings, "Could not save the local build cache.")
	}
	return r, nil
}

func applyBuildGear(result *Result, report *BuildReport) {
	if result == nil || report == nil {
		return
	}
	result.Builds = report
	best := map[int]BuildGear{}
	for _, item := range report.Gear {
		if !item.BIS {
			continue
		}
		old, exists := best[item.ID]
		if !exists || item.Rank < old.Rank {
			best[item.ID] = item
		}
	}
	for i := range result.Upgrades {
		if item, ok := best[result.Upgrades[i].ID]; ok {
			result.Upgrades[i].BIS = true
			result.Upgrades[i].BISRank = item.Rank
			result.Upgrades[i].BISSlot = item.Slot
		}
	}
}

func attachUpgradeBuilds(ctx context.Context, result *Result, specID int, cacheDir string) {
	report, err := fetchBuilds(ctx, specID, "m+", cacheDir)
	if err != nil {
		result.Warnings = append(result.Warnings, "Current Mythic+ BiS reference could not be refreshed; personal upgrade results are still complete. "+err.Error())
		return
	}
	applyBuildGear(result, report)
}

func downloadBuilds(ctx context.Context, specID int, mode string, now time.Time) (*BuildReport, error) {
	spec, ok := wowspec.Lookup(specID)
	if !ok || !buildModes[mode] {
		return nil, fmt.Errorf("unsupported build specialization or mode")
	}
	client := &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 3 || req.URL.Scheme != "https" || req.URL.Host != "murlok.io" {
			return fmt.Errorf("unexpected build-source redirect")
		}
		return nil
	}}
	request, err := http.NewRequestWithContext(ctx, "GET", "https://murlok.io"+spec.Path()+"/"+mode, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "LamdaUI/1.0 (personal build reference)")
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("Murlok refresh unavailable: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("Murlok returned HTTP %d; no access restrictions were bypassed", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 2*1024*1024 {
		return nil, fmt.Errorf("build-source page is too large")
	}
	return parseBuildPage(data, specID, mode, now)
}
