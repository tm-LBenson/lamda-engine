package market

import "sort"

func (m model) quoteCost(item string, q int64) (Plan, bool) {
	key := lotKey{item, q}
	if cached, ok := m.lotCache[key]; ok {
		return cached.Plan, cached.OK
	}
	p, ok := lotCost(m.quotes[item].Lots, q)
	if m.lotCache != nil {
		m.lotCache[key] = pricedLot{p, ok}
	}
	return p, ok
}

// Whole stacks are indivisible. Exact bounded selection minimizes total copper,
// including overbuy, for ordinary craft batches. Very large problems use a
// conservative listed-cost plan and disclose that its selection is approximate.
func lotCost(lots []Lot, q int64) (Plan, bool) {
	if q <= 0 {
		return Plan{}, q == 0
	}
	type bundle struct{ quantity, cost int64 }
	groups := []bundle{}
	exact := q <= 10000
	divisibleOnly := len(lots) > 0
	for _, l := range lots {
		if l.Quantity <= 0 || l.Count <= 0 || l.Price <= 0 {
			return Plan{}, false
		}
		if !l.Divisible {
			divisibleOnly = false
		}
		if !exact {
			continue
		}
		if l.Divisible {
			exact = false
			continue
		}
		count := min(l.Count, (q+l.Quantity-1)/l.Quantity)
		for n := int64(1); count > 0; n *= 2 {
			take := min(n, count)
			count -= take
			if l.Price <= maxCost/take {
				groups = append(groups, bundle{l.Quantity * take, l.Price * take})
			}
			if int64(len(groups))*q > 2_000_000 {
				exact = false
				break
			}
		}
	}
	if !exact {
		p, ok := greedyLotCost(lots, q)
		if ok && !divisibleOnly {
			p.Assumptions = append(p.Assumptions, "Auction stack selection is approximate for this batch size.")
		}
		return p, ok
	}
	type entry struct{ cost, acquired int64 }
	dp := make([]entry, int(q)+1)
	for i := 1; i < len(dp); i++ {
		dp[i].cost = maxCost
	}
	for _, g := range groups {
		for have := int(q) - 1; have >= 0; have-- {
			if dp[have].cost == maxCost {
				continue
			}
			next := int(min(q, int64(have)+g.quantity))
			cost := safeAdd(dp[have].cost, g.cost)
			acquired := dp[have].acquired + g.quantity
			if cost < dp[next].cost || (cost == dp[next].cost && acquired < dp[next].acquired) {
				dp[next] = entry{cost, acquired}
			}
		}
	}
	if dp[int(q)].cost >= maxCost {
		return Plan{}, false
	}
	return Plan{Cost: dp[int(q)].cost, Acquired: dp[int(q)].acquired}, true
}

// Costs use complete stacks and vendor bundles. Unused leftovers have no assumed resale value.
func greedyLotCost(lots []Lot, quantity int64) (Plan, bool) {
	lots = append([]Lot(nil), lots...)
	sort.Slice(lots, func(i, j int) bool {
		return float64(lots[i].Price)/float64(lots[i].Quantity) < float64(lots[j].Price)/float64(lots[j].Quantity)
	})
	p := Plan{Used: map[string]int64{}}
	need := quantity
	for _, l := range lots {
		if need <= 0 {
			break
		}
		if l.Divisible {
			take := min(need, l.Quantity*l.Count)
			if l.Price > maxCost/max(1, take) {
				return Plan{}, false
			}
			p.Cost = safeAdd(p.Cost, (l.Price*take+l.Quantity-1)/l.Quantity)
			p.Acquired += take
			need -= take
		} else {
			n := min(l.Count, (need+l.Quantity-1)/l.Quantity)
			if l.Price > maxCost/max(1, n) {
				return Plan{}, false
			}
			p.Cost = safeAdd(p.Cost, l.Price*n)
			p.Acquired += n * l.Quantity
			need -= n * l.Quantity
		}
	}
	return p, need <= 0 && p.Cost < maxCost
}
