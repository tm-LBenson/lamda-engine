package augmentation

// Reviewed 2026-10-29 against DBM's installed 20261026 encounter definitions,
// the matching original raid events, and the Method Heroic/Mythic guides.
// Targeting awards are deliberately neutral banter: assignment RNG is not a
// mechanic failure. Orb/egg awards credit pickups, never unobserved placement.
var additionalEncounterAwards = map[string][]mechanicAward{
	"3470": {
		{label: "Flame Bearer", unit: "Slithering Flame assignments", joke: "The guild's portable fireplace.", keys: []string{"target/flame"}},
		{label: "Soul Commuter", unit: "Immortal Coil trips", joke: "The other realm knows your name.", keys: []string{"use/coil"}},
		{label: "Soul Rent Due", unit: "Essence Rend assignments", joke: "Nek'zali keeps renewing your lease.", keys: []string{"target/rend"}, minimum: 3, banter: true},
		{label: "Pyre Priority", unit: "Hungering Pyre assignments", joke: "Somehow always on the invitation list.", keys: []string{"target/pyre"}, minimum: 3, banter: true},
		{label: "Echo Chamber", unit: "Anguished Echoes damage", joke: "The circles got an enthusiastic reply.", keys: []string{"avoid/1294846"}, damage: true, banter: true},
	},
	"3445": {
		{label: "Summa Cum Laude", unit: "Helical Toxins puzzles resolved at four", joke: "Calculator optional. Counting mandatory.", keys: []string{"puzzle/four"}},
		{label: "Droplet Dispatcher", unit: "Toxic Droplets soaks", joke: "Popping problems before they become everyone's problem.", keys: []string{"soak/Toxic Droplets"}},
		{label: "Walking Blood Bank", unit: "Blighted Blood assignments", joke: "The boss keeps requesting another donation.", keys: []string{"target/blood"}, minimum: 3, banter: true},
		{label: "Miasma Magnet", unit: "Unstable Miasma assignments", joke: "Personal space denied, repeatedly.", keys: []string{"target/miasma"}, minimum: 3, banter: true},
		{label: "Return to Sender", unit: "Living Venom damage", joke: "The return trip caught you too.", keys: []string{"avoid/1284209"}, damage: true, banter: true},
	},
	"3455": {
		{label: "Blood Donor", unit: "Siphoning Infection support applications", joke: "A donation with unusually sharp paperwork.", keys: []string{"aura/1295380"}},
		{label: "Fire Marshal", unit: "Exploding Infection dispels", joke: "Another explosive patient discharged.", keys: []string{"dispel/1295173"}},
		{label: "Patient Zero", unit: "Adaptive Infection assignments", joke: "Somehow on every clinical trial list.", keys: []string{"target/infection"}, minimum: 3, banter: true},
		{label: "Froth Subscription", unit: "Plague Froth assignments", joke: "The free trial keeps renewing.", keys: []string{"target/froth"}, minimum: 3, banter: true},
		{label: "Plus Sign Collector", unit: "Plague Wave damage", joke: "Found the intersection the hard way.", keys: []string{"avoid/1295798"}, damage: true, banter: true},
	},
	"3497": {
		{label: "Crate Expectations", unit: "crate-opening Splinters applications", joke: "Somebody has to unpack this raid.", keys: []string{"use/crate"}},
		{label: "Mushroom Pilot", unit: "mushroom bounces", joke: "Economy class, excellent legroom.", keys: []string{"use/bounce"}},
		{label: "Gebbo's Favorite", unit: "Explosive Surprise assignments", joke: "Another parcel from your biggest fan.", keys: []string{"target/surprise"}, minimum: 3, banter: true},
		{label: "Landing Pad", unit: "Mighty Thud assignments", joke: "Nama keeps booking the same landing spot.", keys: []string{"target/thud"}, minimum: 3, banter: true},
		{label: "Hot and Cold Relationship", unit: "Frostfire Volley assignments", joke: "Iku cannot decide how he feels about you.", keys: []string{"target/volley"}, minimum: 3, banter: true},
		{label: "Human Popsicle", unit: "Piercing Frost stuns", joke: "An extremely committed freeze frame.", keys: []string{"aura/1291918"}, banter: true},
		{label: "Wave Goodbye", unit: "Blast Wave damage", joke: "The mushroom departure gate was right there.", keys: []string{"avoid/1305844"}, damage: true, banter: true},
	},
	"3420": {
		{label: "Air Traffic Survivor", unit: "Crosswinds assignments survived on the kill", joke: "All those flights, still no early release.", keys: []string{"finish/crosswinds_survived"}},
		{label: "Bomb Squad Graduate", unit: "bomb drops completed on a deathless kill", joke: "Every assigned parcel delivered. Still employed.", keys: []string{"finish/bombs_delivered"}},
		{label: "Bombs Know My Address", unit: "Venomous Surge assignments", joke: "Sszorak upgraded you to scheduled delivery.", keys: []string{"target/surge"}, minimum: 3, banter: true},
		{label: "Frequent Flyer", unit: "Raging Crosswinds assignments", joke: "Boarding group one. Again.", keys: []string{"target/crosswinds"}, minimum: 3, banter: true},
		{label: "Residue Residency", unit: "Caustic Residue damage", joke: "Signed a lease in the wrong puddle.", keys: []string{"avoid/1296667"}, damage: true, banter: true},
		{label: "Fury Fan Club", unit: "Serpent's Fury assignments", joke: "The autograph queue is getting dangerous.", keys: []string{"target/fury"}, minimum: 3, banter: true},
	},
	"3421": {
		{label: "Globule Goblin", unit: "Caustic Globule soaks", joke: "Collecting the raid's least desirable currency.", keys: []string{"soak/Caustic Globule"}},
		{label: "Nine Lives", unit: "kills survived after reaching nine Eternal Venom stacks", joke: "The tenth life was not in the budget.", keys: []string{"finish/venom_nine_survived"}},
		{label: "Clean Finish", unit: "kills finished alive at zero Eternal Venom stacks", joke: "Left with an empty venom tab.", keys: []string{"finish/venom_zero"}},
		{label: "One Stack From Disaster", unit: "kills finished alive at nine Eternal Venom stacks", joke: "Living within one stack of your means.", keys: []string{"finish/venom_nine"}, banter: true},
		{label: "Ichor Loyalty Card", unit: "Coiling Ichor assignments", joke: "The next puddle is apparently on the house.", keys: []string{"target/ichor"}, minimum: 3, banter: true},
		{label: "Laser Tourist", unit: "Vile Flood damage", joke: "The sightseeing route crossed the beam.", keys: []string{"avoid/1294605"}, damage: true, banter: true},
		{label: "Seconds, Please", unit: "Feast soaks while already Feasted", joke: "The buffet explicitly said one serving.", keys: []string{"fail/repeat_feast"}, banter: true},
	},
	"3429": {
		{label: "Orb Hauler", unit: "Volatile Venom orb pickups", joke: "Unpaid logistics, premium service.", keys: []string{"use/venom_orb"}},
		{label: "Purple Parcel Service", unit: "Mutagenic Venom orb pickups", joke: "Hazard pay remains under review.", keys: []string{"use/mutagen_orb"}},
		{label: "No Laughing Matter", unit: "Wail of Terror interrupts", joke: "Cancelled the Spiritcackle comedy set.", keys: []string{"interrupt/1286399"}},
		{label: "Guillotine Loyalty Scheme", unit: "Guillotine assignments", joke: "Zul'jan keeps asking for your headcount.", keys: []string{"target/guillotine"}, minimum: 3, banter: true},
		{label: "Ghosted, Repeatedly", unit: "Unnerving Fixation assignments", joke: "Even the ghosts will not leave you on read.", keys: []string{"target/fixation"}, minimum: 3, banter: true},
		{label: "Dread Delivery", unit: "Dreadmarch assignments", joke: "The boss keeps walking you through onboarding.", keys: []string{"target/dreadmarch"}, minimum: 3, banter: true},
		{label: "Gloom Service", unit: "Gloombomb assignments", joke: "Complimentary room service. Explosive minibar.", keys: []string{"target/gloombomb"}, minimum: 3, banter: true},
		{label: "Bad Company", unit: "Malevolent Resonance applications", joke: "Those ghosts were not meant to mingle.", keys: []string{"aura/1310744"}, banter: true},
	},
	"3492": {
		{label: "Egg Custodian", unit: "Blightscale egg pickups", joke: "Fragile cargo, enthusiastic mother.", keys: []string{"use/egg"}},
		{label: "Heavy Lifter", unit: "Doomscale egg pickups", joke: "The large parcel is your problem now.", keys: []string{"use/doomscale_egg"}},
		{label: "Chain Reaction", unit: "Grasping Fangs breaks", joke: "Successfully ended another clingy relationship.", keys: []string{"aura/1311602"}},
		{label: "Bite Club Regular", unit: "Serpent's Bite assignments", joke: "The first rule is apparently bite this person again.", keys: []string{"target/bite"}, minimum: 3, banter: true},
		{label: "Sting Operation", unit: "Petrifying Sting assignments", joke: "The vipers keep choosing the same suspect.", keys: []string{"target/sting"}, minimum: 3, banter: true},
		{label: "Stage Five Clinger", unit: "Grasping Fangs assignments", joke: "Ula'tek is not taking the hint.", keys: []string{"target/fangs"}, minimum: 3, banter: true},
		{label: "Wave Rider", unit: "Caustic Wave debuff applications", joke: "Surf lessons remain a work in progress.", keys: []string{"aura/1292403"}, banter: true},
	},
}

func init() {
	for id, defs := range additionalEncounterAwards {
		for i := range defs {
			defs[i].extended = true
		}
		encounterAwards[id] = append(encounterAwards[id], defs...)
		encounterAwards[id] = append(encounterAwards[id], mechanicAward{label: "Still Standing", unit: "mechanic assignments survived on a deathless kill", joke: "Repeatedly selected. Refused to clock out.", keys: []string{"finish/assignment_survivor"}, extended: true})
		encounterAwards[id] = append(encounterAwards[id], mechanicAward{label: "Boss's Favorite", unit: "recorded mechanic assignments", joke: "Random targeting has a very personal sense of humor.", keys: []string{"target/all"}, minimum: 6, banter: true, extended: true})
	}
	// Split combined categories so one mechanic does not win twice for the
	// same events. Existing names remain available with the narrower counter.
	a := encounterAwards["3445"]
	a[1].keys = []string{"soak/Unstable Miasma"}
	a[1].unit = "Unstable Miasma soaks"
	a = encounterAwards["3455"]
	a[1].keys = []string{"dispel/1294994", "dispel/1295224", "dispel/1305833"}
	for id, spells := range map[string][]string{"3470": {"1294846"}, "3445": {"1284209"}, "3455": {"1295798"}, "3497": {"1305844"}} {
		r := bossMechanics[id]
		for _, spell := range spells {
			r.bad[spell] = true
		}
	}
	r := bossMechanics["3421"]
	r.good["Caustic Globule"] = true
}
