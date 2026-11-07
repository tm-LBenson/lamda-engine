package augmentation

import (
	"strconv"
	"time"
)

const templeKeyScope = "key:1877"

func templeBoss(id string) bool {
	return id == "2124" || id == "2125" || id == "2126" || id == "2127"
}

// Midnight Season 2: reviewed against the journal, installed DBM/MDT, and
// the original 2026-10-27 Temple key. BfA's old boss rotations are not used.
var templeBossAwards = map[string][]mechanicAward{
	"2124": {
		{label: "Thunder Buddies", unit: "first Thunder and Lightning soaks", joke: "Actually came back after the knockback.", keys: []string{"soak/thunder"}},
		{label: "Unruffled", unit: "deathless duo kills without tracked thunder blast or tempest-zone contact", joke: "Left with both eyebrows intact.", keys: []string{"finish/clean_duo"}},
		{label: "Gust Customer", unit: "Gust assignments", joke: "Aspix has your number on speed dial.", keys: []string{"target/gust"}, minimum: 3, banter: true},
		{label: "Encore Enthusiast", unit: "second Thunder and Lightning blast damage", joke: "Stayed for the encore. The encore was thunder.", keys: []string{"avoid/1314051"}, damage: true, banter: true},
		{label: "Wind Tunnel Tenant", unit: "tempest-zone silence applications", joke: "Your new apartment has excellent ventilation.", keys: []string{"aura/1288885"}, banter: true},
	},
	"2125": {
		{label: "Knot Unraveler", unit: "damage to Knots of Snakes", joke: "Untied a teammate with extreme prejudice.", keys: []string{"work/134388"}, damage: true},
		{label: "Snake Charmer", unit: "distinct Knots hit with hard crowd control", joke: "Customer support has released your teammate.", keys: []string{"cc/knot"}},
		{label: "Storm Serpent Specialist", unit: "damage to Storm Serpents", joke: "The burrow intermission ends when you say so.", keys: []string{"work/134390"}, damage: true},
		{label: "Viper Wiper", unit: "damage to Toxic Vipers", joke: "Cleaned up the nursery before it got ideas.", keys: []string{"work/134389"}, damage: true},
		{label: "Spit Stopper", unit: "Poison Spit interrupts", joke: "Please keep your venom inside the vehicle.", keys: []string{"interrupt/267027"}},
		{label: "Venom Hotline", unit: "Poison Spit dispels", joke: "Your call to poison control was answered.", keys: []string{"dispel/267027"}},
		{label: "No Tunnel Vision", unit: "deathless Merektha kills without tracked Burrow or storm-zone contact", joke: "Saw the snake coming. Moved accordingly.", keys: []string{"finish/clean_merektha"}},
		{label: "Speed Bump", unit: "Burrow damage", joke: "Merektha would like to speak to the roadworks department.", keys: []string{"avoid/264206"}, damage: true, banter: true},
		{label: "Storm Drain", unit: "Lingering Storm damage", joke: "Standing water. Live electricity. Bold combination.", keys: []string{"avoid/1289589"}, damage: true, banter: true},
		{label: "Knot Again", unit: "Knot of Snakes assignments", joke: "The snakes have made this personal.", keys: []string{"target/knot"}, minimum: 3, banter: true},
		{label: "Spit Happens", unit: "Thunder Spit assignments", joke: "The weather forecast is specifically about you.", keys: []string{"target/spit"}, minimum: 3, banter: true},
	},
	"2126": {
		{label: "Human Ground Wire", unit: "logged Galvanized beam ticks", joke: "Prevented the boss from charging his phone.", keys: []string{"beam/ticks"}},
		{label: "Circuit Breaker", unit: "deathless kills with beam duty and no tracked field contact", joke: "Carried the current without becoming floor decor.", keys: []string{"finish/clean_beams"}},
		{label: "Static-Free Finish", unit: "deathless Galvazzt kills without tracked Induction Field contact", joke: "Read the very large warning label on the floor.", keys: []string{"finish/clean_galvazzt"}},
		{label: "Induction Into the Floor", unit: "Induction Field damage", joke: "Accepted into the wrong electrical apprenticeship.", keys: []string{"avoid/1291815"}, damage: true, banter: true},
	},
	"2127": {
		{label: "Loa Life Support", unit: "effective healing to the Avatar", joke: "Treated the actual win condition.", keys: []string{"heal/avatar"}, damage: true},
		{label: "Lifeforce Delivery", unit: "distinct Corrupted Lifeforce soaks", joke: "Delivered the purple parcels personally.", keys: []string{"soak/lifeforce"}},
		{label: "Defiler Eviction", unit: "damage to Essence Defilers", joke: "Removed the reason the health bar would not move.", keys: []string{"work/268427"}, damage: true},
		{label: "Healer Bodyguard", unit: "damage to Faithless Tormentors", joke: "The healer's fan club has been asked to leave.", keys: []string{"work/268317", "work/268729"}, damage: true},
		{label: "Hexxer Exit Interview", unit: "damage to Twisted Hexxers", joke: "Your hex services are no longer required.", keys: []string{"work/268491", "work/136250"}, damage: true},
		{label: "Guardian Dismantler", unit: "damage to Corrupted Guardians", joke: "Converted one large problem into several small purple ones.", keys: []string{"work/268344"}, damage: true},
		{label: "Flame Moderator", unit: "Flame Shock interrupts", joke: "Kept the healer's inbox from catching fire.", keys: []string{"interrupt/1302158", "interrupt/268013"}},
		{label: "Shock Therapy", unit: "Flame Shock dispels", joke: "Deleted a very hostile damage subscription.", keys: []string{"dispel/1302158", "dispel/268013"}},
		{label: "Dry Land Enthusiast", unit: "deathless Avatar kills without tracked Hex Muck or Agony hits", joke: "Declined the complimentary amphibian upgrade.", keys: []string{"finish/clean_avatar"}},
		{label: "Frog Frequent Flyer", unit: "Hex Muck transformations", joke: "The loyalty programme now includes complimentary ribbits.", keys: []string{"aura/1300684"}, banter: true},
		{label: "Agony Tourist", unit: "Agony of Sethraliss damage", joke: "Visited every attraction, including the floor swirls.", keys: []string{"avoid/1301253"}, damage: true, banter: true},
		{label: "Tormentor Fan Club", unit: "distinct Tormentor fixates", joke: "Popular, talented, and being chased around the room.", keys: []string{"target/tormentor"}, minimum: 5, banter: true},
		{label: "Hex Appeal", unit: "Latent Hex assignments", joke: "The Hexxers keep asking for a second date.", keys: []string{"target/hex"}, minimum: 3, banter: true},
		{label: "Vile Pen Pal", unit: "Vile Charge assignments", joke: "Another strongly worded charge addressed to you.", keys: []string{"target/vile"}, minimum: 3, banter: true},
	},
}

var templeTrashAwards = []mechanicAward{
	{label: "Lightning Librarian", unit: "Lightning Bolt interrupts", joke: "Electricity belongs in the quiet section.", keys: []string{"interrupt/1291262"}},
	{label: "Cheap Shot Refund", unit: "Poisoned Cheap Shot interrupts", joke: "Ambush cancelled due to customer dissatisfaction.", keys: []string{"interrupt/1308100"}},
	{label: "Ambush Aftercare", unit: "Poisoned Cheap Shot dispels", joke: "Returned movement privileges to the victim.", keys: []string{"dispel/1308100"}},
	{label: "Mind Your Business", unit: "Addle Mind interrupts", joke: "No unsolicited thoughts today.", keys: []string{"interrupt/1293307"}},
	{label: "Clear Head Department", unit: "Addle Mind dispels", joke: "Rebooted a teammate without losing their keybinds.", keys: []string{"dispel/1293307"}},
	{label: "Cytotoxin Custodian", unit: "Cytotoxin dispels", joke: "Another snake's complaint has been resolved.", keys: []string{"dispel/1308148"}},
	{label: "Conduction Correction", unit: "Imbued Conduction dispels", joke: "Prevented an unscheduled standing-still break.", keys: []string{"dispel/1296052"}},
	{label: "Battery Removal Service", unit: "Accumulate Charge purges", joke: "Batteries sold separately. Removed immediately.", keys: []string{"purge/1310739"}},
	{label: "Eye Tech Support", unit: "Essence Disruption interrupts", joke: "Have you tried stopping the guy breaking the eye?", keys: []string{"interrupt/1303535"}},
	{label: "Venom Veto", unit: "Venom Bolt interrupts", joke: "Your application to poison the party has been denied.", keys: []string{"interrupt/1310683"}},
	{label: "Spark Plug Pulled", unit: "damage to Spark Channelers", joke: "Switched off the corridor's worst light show.", keys: []string{"work/265057", "work/139110"}, damage: true},
	{label: "Temple Pest Control", unit: "damage to Temple Disruptors", joke: "Protected the temple's very expensive eyeballs.", keys: []string{"work/269227"}, damage: true},
	{label: "Eye Contact", unit: "logged Siphon Energy channel ticks", joke: "Held eye contact long enough to open a door.", keys: []string{"eye/ticks"}},
	{label: "Sand in Everything", unit: "Scouring Sand damage", joke: "The complimentary exfoliation was a little aggressive.", keys: []string{"avoid/272655"}, damage: true, banter: true},
	{label: "Spark of Bad Judgment", unit: "Loose Spark damage", joke: "Tested the electric fence. Repeatedly.", keys: []string{"avoid/267483"}, damage: true, banter: true},
	{label: "Bleed Magnet", unit: "Serrated Charge applications", joke: "The krolusks have voted you most approachable.", keys: []string{"aura/1291399"}, minimum: 3, banter: true},
	{label: "Storm-Puddle Season Ticket", unit: "trash Lingering Storm damage", joke: "Renewed your membership in the wrong puddle.", keys: []string{"avoid/1293133"}, damage: true, banter: true},
}

func init() {
	for _, id := range []string{"2124", "2125", "2126", "2127", templeKeyScope} {
		bossMechanics[id] = bossMechanicRules{good: names(), bad: names()}
		encounterObservations[id] = map[string]string{}
	}
	for id, spells := range map[string][]string{
		"2124": {"1314051"}, "2125": {"264206", "1289589"},
		"2126": {"1291815"}, "2127": {"1301253", "1300684"},
	} {
		for _, spell := range spells {
			bossMechanics[id].bad[spell] = true
			bossMechanics[templeKeyScope].bad[spell] = true
		}
	}
	for _, spell := range []string{"272655", "1293133", "267483"} {
		bossMechanics[templeKeyScope].bad[spell] = true
	}
	for id, observations := range map[string]map[string]string{
		"2124": {"1288457": "target/gust"},
		"2125": {"1290030": "target/knot", "1289109": "target/spit"},
		"2127": {"1311979": "target/hex", "1302618": "target/vile"},
	} {
		for spell, key := range observations {
			encounterObservations[id][spell] = key
			encounterObservations[templeKeyScope][spell] = key
		}
	}
	groups := [][]mechanicAward{}
	for _, id := range []string{"2124", "2125", "2126", "2127"} {
		defs := templeBossAwards[id]
		for i := range defs {
			defs[i].minimumVersion = 2
		}
		encounterAwards[id] = defs
		groups = append(groups, defs)
	}
	for i := range templeTrashAwards {
		templeTrashAwards[i].minimumVersion = 2
	}
	groups = append(groups, templeTrashAwards)
	// Round-robin across all four bosses and trash, with core award slots
	// reserved by detailedBossAwards, just like the Voidscar completed key.
	for i := 0; ; i++ {
		added := false
		for _, defs := range groups {
			if i < len(defs) {
				encounterAwards[templeKeyScope] = append(encounterAwards[templeKeyScope], defs[i])
				added = true
			}
		}
		if !added {
			break
		}
	}
}

// Only hard controls that can break a Knot are eligible. Roots/slows, generic
// debuffs and aura removal/despawn are not evidence of a rescue.
var knotControls = names("3355", "119381", "132168", "179057", "105421", "207685", "5246", "8122", "31661", "99", "19577", "853", "408", "1776", "2094", "118", "51514", "115078", "217832", "20066", "6789", "64044", "107570", "30283", "157981")

func (s *Store) trackTempleAwards(seg *segment, f []string) {
	scope := mechanicScope(seg)
	if (!templeBoss(scope) && scope != templeKeyScope) || len(f) < 12 {
		return
	}
	d := seg.damage
	if friendlyGroup(f[3]) {
		if (f[0] == "SPELL_DISPEL" || f[0] == "SPELL_STOLEN") && len(f) > 12 && !friendlyGroup(f[7]) && npcID(f[5]) != "" {
			key := "purge/" + f[12]
			if wantedMechanic(scope, key) {
				d.addMechanic(key, f[1], 1)
			}
		}
		if !friendlyGroup(f[7]) {
			key := "work/" + npcID(f[5])
			if wantedMechanic(scope, key) {
				if base, support, ok := damagePayload(f); ok && !support {
					if n, reason := recordedDamageAmount(f, base); reason == "" {
						d.addMechanic(key, f[1], n)
					} else if reason != "not_damage" {
						d.unknownMechanic(key)
					}
				}
			}
		}
		if npcID(f[5]) == "134388" && f[0] == "SPELL_AURA_APPLIED" && knotControls[f[9]] && wantedMechanic(scope, "cc/knot") {
			st := d.encounter()
			stamp := "cc/knot/" + f[5]
			if st.observed[stamp].IsZero() {
				d.addMechanic("cc/knot", f[1], 1)
				st.observed[stamp] = s.last
			}
		}
		if npcID(f[5]) == "133392" && (f[0] == "SPELL_HEAL" || f[0] == "SPELL_PERIODIC_HEAL") && wantedMechanic(scope, "heal/avatar") {
			// Objective healing is separate from party healing and never credits
			// the Avatar's self-heals, lifeforce consumption or NPC siphons.
			if len(f) < 36 {
				d.unknownMechanic("heal/avatar")
				return
			}
			amount, e1 := strconv.ParseInt(f[31], 10, 64)
			over, e2 := strconv.ParseInt(f[33], 10, 64)
			if e1 != nil || e2 != nil || amount < 0 || over < 0 || over > amount || amount > 1e12 {
				d.unknownMechanic("heal/avatar")
				return
			}
			d.addMechanic("heal/avatar", f[1], amount-over)
		}
	}
	if !player(f[5]) || !friendlyGroup(f[7]) || friendlyGroup(f[3]) {
		return
	}
	st := d.encounter()
	if f[0] == "SPELL_AURA_APPLIED" && (f[9] == "1288885" || f[9] == "1289589" || f[9] == "1291815" || f[9] == "1300684") {
		d.addMechanic("contact/"+f[9], f[5], 1)
	}
	if (f[0] == "SPELL_AURA_APPLIED" || f[0] == "SPELL_AURA_APPLIED_DOSE") && f[9] == "1300877" && npcID(f[1]) == "268364" && wantedMechanic(scope, "soak/lifeforce") {
		stamp := "lifeforce/" + f[1]
		if st.observed[stamp].IsZero() {
			d.addMechanic("soak/lifeforce", f[5], 1)
			st.observed[stamp] = s.last
		}
	}
	if f[0] == "SPELL_AURA_APPLIED" && f[9] == "1300704" && (npcID(f[1]) == "268317" || npcID(f[1]) == "268729") && wantedMechanic(scope, "target/tormentor") {
		stamp := "tormentor/" + f[1]
		if st.observed[stamp].IsZero() {
			d.addMechanic("target/tormentor", f[5], 1)
			st.observed[stamp] = s.last
		}
	}
	key := ""
	if f[9] == "266923" && npcID(f[1]) == "135445" {
		key = "beam/ticks"
	} else if f[9] == "1288092" && npcID(f[1]) == "262530" {
		key = "soak/thunder"
	} else if f[9] == "1303596" && npcID(f[1]) == "240681" {
		key = "eye/ticks"
	}
	if key != "" && wantedMechanic(scope, key) {
		if base, support, ok := damagePayload(f); ok && !support {
			if _, reason := recordedDamageAmount(f, base); reason == "" {
				stamp := key + "/" + f[1] + "/" + f[5]
				last := st.observed[stamp]
				gap := 100 * time.Millisecond
				if key == "soak/thunder" {
					gap = time.Second
				}
				if last.IsZero() || s.last.Sub(last) >= gap {
					d.addMechanic(key, f[5], 1)
					st.observed[stamp] = s.last
				}
			} else if reason != "not_damage" {
				d.unknownMechanic(key)
			}
		}
	}
}

func (s *Store) finishTempleAwards(seg *segment, at time.Time, success bool) {
	if seg.kind != "boss" || !templeBoss(seg.id) || !success || at.Sub(seg.start) < 20*time.Second {
		return
	}
	d := seg.damage
	st := d.encounterState
	if st == nil || d.mechanicsInvalid || len(d.unknownMechanics) > 0 {
		return
	}
	key := map[string]string{"2124": "finish/clean_duo", "2125": "finish/clean_merektha", "2126": "finish/clean_galvazzt", "2127": "finish/clean_avatar"}[seg.id]
	for id, first := range st.firstSeen {
		if st.dead[id] || st.died[id] || first.Sub(seg.start) > 30*time.Second || at.Sub(st.lastSeen[id]) > 10*time.Second || d.avoidableHits[id] > 0 {
			continue
		}
		if seg.id == "2124" && d.mechanicTotals["contact/1288885"][id] > 0 || seg.id == "2125" && d.mechanicTotals["contact/1289589"][id] > 0 || seg.id == "2126" && d.mechanicTotals["contact/1291815"][id] > 0 || seg.id == "2127" && d.mechanicTotals["contact/1300684"][id] > 0 {
			continue
		}
		d.addMechanic(key, id, 1)
		beamDuty := seg.id == "2126" && d.mechanicTotals["beam/ticks"][id] >= 20
		if beamDuty {
			d.addMechanic("finish/clean_beams", id, 1)
		}
		if run := s.active["key"]; run != nil && run.id == "1877" && run.start.Before(seg.start) {
			run.damage.addMechanic(key, id, 1)
			if beamDuty {
				run.damage.addMechanic("finish/clean_beams", id, 1)
			}
		}
	}
}
