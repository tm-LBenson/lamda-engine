package market

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type Plan struct {
	MakerID     string           `json:"-"`
	ActorIDs    []string         `json:"-"`
	Assumptions []string         `json:"assumptions,omitempty"`
	CraftSteps  []string         `json:"-"`
	Maker       string           `json:"maker,omitempty"`
	Cost        int64            `json:"cost"`
	Acquired    int64            `json:"acquired"`
	Steps       []string         `json:"steps"`
	Actors      []string         `json:"actors"`
	Used        map[string]int64 `json:"-"`
}
type Opportunity struct {
	Item     string  `json:"item"`
	Name     string  `json:"name"`
	Kind     string  `json:"kind"`
	Quantity int64   `json:"quantity"`
	Cost     int64   `json:"cost"`
	Revenue  int64   `json:"revenue"`
	Profit   int64   `json:"profit"`
	ROI      float64 `json:"roi"`
	Target   float64 `json:"target"`
	Plan     Plan    `json:"plan"`
	Note     string  `json:"note"`
}
type quote struct {
	Lots     []Lot
	At       int64
	Coverage string
}
type lotKey struct {
	Item     string
	Quantity int64
}
type pricedLot struct {
	Plan Plan
	OK   bool
}
type model struct {
	lotCache map[lotKey]pricedLot
	quotes   map[string]quote
	vendors  []Vendor
	recipes  []Recipe
	byOutput map[string][]Recipe
	chars    []Character
	items    map[string]Item
	settings Settings
	now      int64
}

const maxCost = int64(9_000_000_000_000_000)

func safeAdd(a, b int64) int64 {
	if a < 0 || b < 0 || a > maxCost-b {
		return maxCost
	}
	return a + b
}

func (m model) actors(r Recipe) []string {
	if m.settings.Mode == "theory" {
		if rank, ok := m.settings.Theory[r.Profession]; ok && rank >= r.Rank && (r.RankKnown || m.settings.AllowUnknownRanks) {
			return []string{"Theorycraft: " + r.Profession}
		}
		return nil
	}
	out := []string{}
	for _, c := range m.chars {
		if m.settings.Mode == "character" && c.ID != m.settings.Character {
			continue
		}
		known := false
		for _, id := range c.Known {
			if id == r.ID {
				known = true
				break
			}
		}
		if !known {
			continue
		}
		for _, p := range c.Professions {
			if p.ID == r.Profession && p.Rank >= r.Rank {
				out = append(out, c.ID)
				break
			}
		}
	}
	return out
}
func (m model) actorName(id string) string {
	for _, c := range m.chars {
		if c.ID == id {
			name := c.DisplayName()
			for _, other := range m.chars {
				if other.ID != id && other.DisplayName() == name {
					return name + " (" + id + ")"
				}
			}
			return name
		}
	}
	return id
}
func unique(v []string) []string {
	out := []string{}
	for _, s := range v {
		found := false
		for _, x := range out {
			if s == x {
				found = true
				break
			}
		}
		if !found {
			out = append(out, s)
		}
	}
	return out
}

// Reprice shared purchased inputs jointly, so two recipe branches cannot
// both consume the same cheap stack or each charge the same bundle twice.
func (m model) reprice(used map[string]int64) (int64, bool) {
	var cost int64
	for source, quantity := range used {
		if strings.HasPrefix(source, "ah/") {
			p, ok := m.quoteCost(strings.TrimPrefix(source, "ah/"), quantity)
			if !ok {
				return 0, false
			}
			cost = safeAdd(cost, p.Cost)
		} else {
			found := false
			for _, v := range m.vendors {
				if source != "vendor/"+v.ID {
					continue
				}
				bundles := (quantity + v.Bundle - 1) / v.Bundle
				if v.Stock >= 0 && bundles*v.Bundle > v.Stock {
					return 0, false
				}
				if v.Buy > maxCost/max(1, bundles) {
					return 0, false
				}
				cost = safeAdd(cost, v.Buy*bundles)
				found = true
				break
			}
			if !found {
				return 0, false
			}
		}
	}
	return cost, cost < maxCost
}
func (m model) purchaseSteps(used map[string]int64) []string {
	keys := make([]string, 0, len(used))
	for k := range used {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := []string{}
	for _, k := range keys {
		if strings.HasPrefix(k, "ah/") {
			id := strings.TrimPrefix(k, "ah/")
			p, ok := m.quoteCost(id, used[k])
			if ok {
				out = append(out, fmt.Sprintf("Buy %d × %s at auction for %d copper (listing prices)", p.Acquired, m.name(id), p.Cost))
			}
		} else {
			for _, v := range m.vendors {
				if k == "vendor/"+v.ID {
					n := (used[k] + v.Bundle - 1) / v.Bundle
					out = append(out, fmt.Sprintf("Buy %d × %s from %s (%s) for %d copper", n*v.Bundle, m.name(v.Item), v.Name, v.Location, n*v.Buy))
					break
				}
			}
		}
	}
	return out
}
func (m model) recipesFor(item string) []Recipe {
	if m.byOutput != nil {
		return m.byOutput[item]
	}
	out := []Recipe{}
	for _, r := range m.recipes {
		if r.Output.Item == item {
			out = append(out, r)
		}
	}
	return out
}
func (m model) acquire(item string, q int64, path map[string]bool, depth int) (Plan, bool) {
	budget := 20000
	return m.acquireFor(item, q, path, depth, "", &budget)
}
func (m model) acquireFor(item string, q int64, path map[string]bool, depth int, preferred string, budget *int) (Plan, bool) {
	*budget--
	if *budget < 0 || depth > 6 || path[item] || q <= 0 || q > 10_000_000 {
		return Plan{}, false
	}
	next := map[string]bool{}
	for k, v := range path {
		next[k] = v
	}
	next[item] = true
	best := Plan{Cost: maxCost}
	ok := false
	if a, yes := m.quotes[item]; yes && m.now-a.At <= int64(m.settings.MaxAgeHours)*3600 && a.Coverage != "summary" {
		if p, yes := m.quoteCost(item, q); yes {
			p.Steps = []string{fmt.Sprintf("Buy %d × %s at auction (listing prices)", p.Acquired, m.name(item))}
			p.Used = map[string]int64{"ah/" + item: q}
			best, ok = p, true
		}
	}
	for _, v := range m.vendors {
		if v.Item != item || v.Buy <= 0 || v.Requirement != "" {
			continue
		}
		bundles := (q + v.Bundle - 1) / v.Bundle
		if v.Stock >= 0 && bundles*v.Bundle > v.Stock {
			continue
		}
		if v.Buy > maxCost/max(1, bundles) {
			continue
		}
		cost := v.Buy * bundles
		if cost < best.Cost {
			best = Plan{Cost: cost, Acquired: bundles * v.Bundle, Steps: []string{fmt.Sprintf("Buy %d × %s from %s (%s)", bundles*v.Bundle, m.name(item), v.Name, v.Location)}, Used: map[string]int64{"vendor/" + v.ID: q}}
			ok = true
		}
	}
	for _, r := range m.recipesFor(item) {
		if r.Cooldown != "" || r.Requirement != "" {
			continue
		}
		actors := m.actors(r)
		sort.SliceStable(actors, func(i, j int) bool { return actors[i] == preferred && actors[j] != preferred })
		for _, actor := range actors {
			crafts := (q + r.Output.Quantity - 1) / r.Output.Quantity
			label := m.actorName(actor)
			p := Plan{Acquired: crafts * r.Output.Quantity, Maker: label, MakerID: actor, ActorIDs: []string{actor}, Actors: []string{label}, Used: map[string]int64{}}
			good := true
			for _, in := range r.Inputs {
				child, yes := m.acquireFor(in.Item, in.Quantity*crafts, next, depth+1, actor, budget)
				if !yes {
					good = false
					break
				}
				if child.MakerID != "" && child.MakerID != actor && !m.items[in.Item].Transferable {
					good = false
					break
				}
				p.CraftSteps = append(p.CraftSteps, child.CraftSteps...)
				p.Assumptions = append(p.Assumptions, child.Assumptions...)
				p.Actors = append(p.Actors, child.Actors...)
				p.ActorIDs = append(p.ActorIDs, child.ActorIDs...)
				for k, n := range child.Used {
					p.Used[k] += n
				}
			}
			if !good {
				continue
			}
			cost, yes := m.reprice(p.Used)
			if !yes || cost >= best.Cost {
				continue
			}
			p.Cost = cost
			p.Actors = unique(p.Actors)
			p.ActorIDs = unique(p.ActorIDs)
			p.CraftSteps = append(p.CraftSteps, fmt.Sprintf("%s: %s × %d", label, r.Name, crafts))
			p.Steps = append(m.purchaseSteps(p.Used), p.CraftSteps...)
			for source, n := range p.Used {
				if strings.HasPrefix(source, "ah/") {
					raw, _ := m.quoteCost(strings.TrimPrefix(source, "ah/"), n)
					p.Assumptions = append(p.Assumptions, raw.Assumptions...)
				}
			}
			if r.Custom {
				p.Assumptions = append(p.Assumptions, "Custom recipe model: "+r.Name)
			}
			if !r.RankKnown && m.settings.Mode == "theory" {
				p.Assumptions = append(p.Assumptions, "Required skill rank unverified: "+r.Name)
			}
			p.Assumptions = unique(p.Assumptions)
			best, ok = p, true
		}
	}
	return best, ok
}
func (m model) name(id string) string {
	if i, ok := m.items[id]; ok && i.Name != "" {
		return i.Name
	}
	return id
}
func (s *Store) Opportunities(key string, now time.Time) ([]Opportunity, error) {
	return s.opportunitiesFor(s, key, now)
}
func (s *Store) opportunitiesFor(profile *Store, key string, now time.Time) ([]Opportunity, error) {
	settings := profile.Settings(key)
	m := model{lotCache: map[lotKey]pricedLot{}, quotes: map[string]quote{}, items: map[string]Item{}, settings: settings, now: now.Unix()}
	if recipes, err := s.recipesFor(profile, key); err != nil {
		return nil, err
	} else {
		m.recipes = recipes
	}

	if err := profile.documents(key, "character", &m.chars); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT body FROM documents WHERE market=? AND kind='vendor' AND at>=?`, key, now.Add(-time.Duration(settings.MaxAgeHours)*time.Hour).Unix())
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var b []byte
		rows.Scan(&b)
		var v Vendor
		json.Unmarshal(b, &v)
		m.vendors = append(m.vendors, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = s.db.Query(`SELECT q.item,q.at,q.coverage,q.lots FROM quotes q WHERE market=? AND q.scan=(SELECT z.scan FROM quotes z WHERE z.market=q.market AND z.item=q.item ORDER BY z.at DESC,z.scan DESC LIMIT 1)`, key)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id string
		var v quote
		var b []byte
		if err = rows.Scan(&id, &v.At, &v.Coverage, &b); err != nil {
			rows.Close()
			return nil, err
		}
		json.Unmarshal(b, &v.Lots)
		m.quotes[id] = v
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	var fullAt int64
	_ = s.db.QueryRow(`SELECT COALESCE(MAX(at),0) FROM scans WHERE market=? AND complete=1 AND coverage IN ('full','summary')`, key).Scan(&fullAt)
	for id, v := range m.quotes {
		if v.At < fullAt {
			delete(m.quotes, id)
		}
	}
	m.byOutput = map[string][]Recipe{}
	for _, r := range m.recipes {
		m.byOutput[r.Output.Item] = append(m.byOutput[r.Output.Item], r)
	}
	cards, err := s.cardsFor(profile, key, now.Add(-30*24*time.Hour).Unix())
	if err != nil {
		return nil, err
	}
	// Include transfer metadata for intermediate items that have no auction history.
	itemRows, err := s.db.Query(`SELECT body FROM items WHERE market=?`, key)
	if err != nil {
		return nil, err
	}
	for itemRows.Next() {
		var b []byte
		if err = itemRows.Scan(&b); err != nil {
			itemRows.Close()
			return nil, err
		}
		var item Item
		if err = json.Unmarshal(b, &item); err != nil {
			itemRows.Close()
			return nil, err
		}
		m.items[item.ID] = item
	}
	err = itemRows.Err()
	itemRows.Close()
	if err != nil {
		return nil, err
	}
	out := []Opportunity{}
	for _, c := range cards {
		m.items[c.ID] = c.Item
	}
	add := func(id, kind string, qty, cost, revenue int64, target float64, p Plan, note string) {
		extra := int64(max(0, len(p.ActorIDs)-1))
		if settings.TransferCost > maxCost/max(1, extra) {
			return
		}
		fee := extra * settings.TransferCost
		cost = safeAdd(cost, fee)
		p.Cost = cost
		if fee > 0 {
			p.Steps = append(p.Steps, fmt.Sprintf("Additional character overhead: %d copper", fee))
		}
		if settings.Mode == "theory" && settings.AllowUnknownRanks {
			note += " Unverified skill requirements are allowed by your theorycraft settings."
		}
		if len(p.Assumptions) > 0 {
			note += " " + strings.Join(p.Assumptions, ". ") + "."
		}
		if cost <= 0 || revenue <= cost || cost >= maxCost {
			return
		}
		out = append(out, Opportunity{Item: id, Name: m.name(id), Kind: kind, Quantity: qty, Cost: cost, Revenue: revenue, Profit: revenue - cost, ROI: float64(revenue-cost) / float64(cost) * 100, Target: target, Plan: p, Note: note})
	}
	net := func(price float64, qty int64) int64 {
		gross := price * float64(qty)
		if gross > float64(maxCost) {
			return 0
		}
		return int64(math.Floor(gross*float64(10000-settings.CutBPS)/10000)) - settings.LostDeposit
	}
	seenRecipes := map[string]bool{}
	for _, r := range m.recipes {
		if seenRecipes[r.Output.Item] {
			continue
		}
		seenRecipes[r.Output.Item] = true
		q := r.Output.Quantity * max(1, settings.Batch)
		if a, ok := m.quotes[r.Output.Item]; !ok || m.now-a.At > int64(settings.MaxAgeHours)*3600 || len(a.Lots) == 0 {
			continue
		}
		p, ok := m.acquire(r.Output.Item, q, map[string]bool{}, 0)
		if !ok || len(p.Actors) == 0 {
			continue
		}
		q = p.Acquired
		a := m.quotes[r.Output.Item]
		price := float64(a.Lots[0].Price) / float64(a.Lots[0].Quantity)
		for _, l := range a.Lots {
			price = math.Min(price, float64(l.Price)/float64(l.Quantity))
		}
		add(r.Output.Item, "craft", q, p.Cost, net(price, q), price, p, "Estimated sale at the lowest observed listing. Demand and sale time are unknown.")
	}
	for _, v := range m.vendors {
		if v.Sell <= 0 {
			continue
		}
		p, ok := m.acquire(v.Item, 1, map[string]bool{}, 0)
		if ok && v.Sell <= maxCost/max(1, p.Acquired) {
			add(v.Item, "vendor sale", p.Acquired, p.Cost, v.Sell*p.Acquired, float64(v.Sell), p, "Vendor resale uses the observed sell price for the entire acquired output. Verify the merchant and item variant.")
		}
	}
	for _, c := range cards {
		if c.BindingKnown && !c.Transferable {
			continue
		}
		a := m.quotes[c.ID]
		if len(a.Lots) > 0 && m.now-a.At <= int64(settings.MaxAgeHours)*3600 && a.Coverage != "summary" {
			sizes := map[int64]bool{1: true}
			for _, v := range m.vendors {
				if v.Item == c.ID {
					sizes[v.Bundle] = true
				}
			}
			best := Plan{}
			bestProfit := int64(0)
			for q := range sizes {
				p, yes := m.acquire(c.ID, q, map[string]bool{}, 0)
				if !yes || len(p.Actors) > 0 {
					continue
				}
				profit := net(c.Price, p.Acquired) - p.Cost
				if profit > bestProfit || profit == bestProfit && profit > 0 && p.Cost < best.Cost {
					best, bestProfit = p, profit
				}
			}
			if bestProfit > 0 {
				add(c.ID, "source resale", best.Acquired, best.Cost, net(c.Price, best.Acquired), c.Price, best, "Buy from the cheaper observed source and list at the observed auction price. Listing supply does not establish demand.")
			}

		}
		if len(c.History) < 3 || m.now-a.At > int64(settings.MaxAgeHours)*3600 || a.Coverage == "summary" {
			continue
		}
		prior := []float64{}
		for _, h := range c.History[:len(c.History)-1] {
			prior = append(prior, h.Price)
		}
		sort.Float64s(prior)
		target := prior[len(prior)/2]
		p, ok := m.quoteCost(c.ID, 1)
		// A resale position can use an entire low-unit-cost stack, even when a
		// smaller purchase is cheaper in total. Keep both objectives distinct.
		bulk, bulkOK := greedyLotCost(a.Lots, 1)
		if bulkOK && (!ok || net(target, bulk.Acquired)-bulk.Cost > net(target, p.Acquired)-p.Cost) {
			p, ok = bulk, true
		}
		if !ok {
			continue
		}
		p.Steps = []string{fmt.Sprintf("Buy %d × %s at the current listing", p.Acquired, c.Name)}
		add(c.ID, "price recovery", p.Acquired, p.Cost, net(target, p.Acquired), target, p, "Speculative target: median of prior observations over 30 days. Recovery is not predicted.")
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Profit > out[j].Profit })
	if len(out) > 300 {
		out = out[:300]
	}
	return out, nil
}
