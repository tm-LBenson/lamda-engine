package simming

import (
	"fmt"
	"github.com/tm-LBenson/lamda-engine/internal/wowspec"
	"regexp"
	"strconv"
	"strings"
)

var word = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
var numericList = regexp.MustCompile(`^[0-9]+(/[0-9]+)*$`)
var talentCode = regexp.MustCompile(`^[A-Za-z0-9+/=]+$`)
var professionCode = regexp.MustCompile(`^[a-z_]+=[0-9]{1,4}(/[a-z_]+=[0-9]{1,4})?$`)
var classes = stringSet("deathknight demonhunter druid evoker hunter mage monk paladin priest rogue shaman warlock warrior")
var slots = stringSet("head neck shoulder back chest shirt tabard wrist hands waist legs feet finger1 finger2 trinket1 trinket2 main_hand off_hand")
var itemFields = stringSet("id enchant_id gem_id bonus_id crafted_stats gem_bonus_id crafting_quality drop_level content_tuning redirected_base_stats titan_disc_id ilevel")

func stringSet(s string) map[string]bool {
	r := map[string]bool{}
	for _, s := range strings.Fields(s) {
		r[s] = true
	}
	return r
}

type Candidate struct {
	Venomstones   int    `json:"venomstones,omitempty"`
	ID            string `json:"id"`
	ComparisonID  string `json:"comparisonID,omitempty"`
	Label         string `json:"label"`
	Source        string `json:"source"`
	Slot          string `json:"slot"`
	Value         string `json:"value"`
	BaseValue     string `json:"baseValue,omitempty"`
	Level         int    `json:"level,omitempty"`
	NextLevel     int    `json:"nextLevel,omitempty"`
	MaxLevel      int    `json:"maxLevel,omitempty"`
	Track         string `json:"track,omitempty"`
	CurrentRank   int    `json:"currentRank,omitempty"`
	MaxRank       int    `json:"maxRank,omitempty"`
	RankLevels    []int  `json:"rankLevels,omitempty"`
	Variant       string `json:"variant,omitempty"`
	Link          string `json:"link,omitempty"`
	InventoryType string `json:"inventoryType,omitempty"`
}
type TalentBuild struct {
	Label string `json:"label"`
	Code  string `json:"code"`
}
type Profile struct {
	Planning   PlanningData  `json:"planning"`
	Base       string        `json:"-"`
	Class      string        `json:"class"`
	Level      int           `json:"level"`
	Spec       string        `json:"spec"`
	Role       string        `json:"role"`
	Candidates []Candidate   `json:"candidates"`
	Talents    []TalentBuild `json:"talents"`
}

func itemValue(value string) (string, error) {
	value = strings.TrimPrefix(value, ",")
	parts := strings.Split(value, ",")
	seen := map[string]bool{}
	for i, part := range parts {
		key, val, ok := strings.Cut(part, "=")
		if !ok && i == 0 && word.MatchString(part) {
			continue
		} // optional item name; never a file directive
		if !ok || !itemFields[key] || seen[key] || !numericList.MatchString(val) || len(val) > 512 {
			return "", fmt.Errorf("unsupported item field %q", key)
		}
		seen[key] = true
	}
	if !seen["id"] {
		return "", fmt.Errorf("item ID is missing")
	}
	return value, nil
}

// Export text is data, never a simulator command file. Build a new allowlisted
// profile: reject includes, actions, output paths, shell/file/network options.
func ParseProfile(text string) (*Profile, error) {
	if len(text) == 0 || len(text) > MaxSnapshot {
		return nil, fmt.Errorf("a complete SimulationCraft character export is required")
	}
	p := &Profile{Candidates: []Candidate{}}
	var lines []string
	seen := map[string]bool{}
	source := "owned"
	label := ""
	for _, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		comment := strings.HasPrefix(line, "#")
		if comment {
			if strings.HasPrefix(line, "###") {
				lower := strings.ToLower(line)
				switch {
				case strings.Contains(lower, "weekly"), strings.Contains(lower, "vault"):
					source = "vault"
				case strings.Contains(lower, "bags"):
					source = "owned"
				default:
					source = "other"
				}
			}
			line = strings.TrimSpace(strings.TrimLeft(line, "#"))
		}
		key, val, ok := strings.Cut(line, "=")
		if ok {
			handled, err := p.Planning.parse(key, val)
			if err != nil {
				return nil, err
			}
			if handled {
				continue
			}
		}
		if comment {
			if ok && key == "talents" && len(val) <= 2048 && talentCode.MatchString(val) {
				p.Talents = append(p.Talents, TalentBuild{Label: strings.TrimPrefix(label, "Saved Loadout: "), Code: val})
			} else if ok && slots[key] && (source == "owned" || source == "vault") {
				value, err := itemValue(val)
				if err != nil {
					return nil, err
				}
				if len(p.Candidates) < 256 {
					p.Candidates = append(p.Candidates, Candidate{ID: fmt.Sprintf("item%d", len(p.Candidates)+1), Label: label, Source: source, Slot: key, Value: value})
				}
			} else if len(line) > 0 && len(line) < 200 && !ok {
				label = line
			}
			continue
		}
		if !ok || seen[key] {
			return nil, fmt.Errorf("invalid or repeated profile field %q", key)
		}
		seen[key] = true
		switch {
		case classes[key]:
			if p.Class != "" {
				return nil, fmt.Errorf("only one character may be imported")
			}
			p.Class = key
			lines = append(lines, key+"=Lamda")
		case slots[key]:
			value, err := itemValue(val)
			if err != nil {
				return nil, err
			}
			lines = append(lines, key+"="+value)
		case key == "level":
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 || n > 200 {
				return nil, fmt.Errorf("invalid level")
			}
			p.Level = n
			lines = append(lines, line)
		case key == "race" || key == "spec" || key == "role" || key == "zandalari_loa":
			if !word.MatchString(val) && !(key == "role" && val == "") {
				return nil, fmt.Errorf("invalid %s", key)
			}
			if key == "spec" {
				p.Spec = val
			}
			if key == "role" {
				p.Role = val
			}
			lines = append(lines, line)
		case key == "talents":
			if len(val) > 2048 || !talentCode.MatchString(val) {
				return nil, fmt.Errorf("invalid talent export")
			}
			lines = append(lines, line)
		case key == "professions":
			if !professionCode.MatchString(val) {
				return nil, fmt.Errorf("invalid professions")
			}
			lines = append(lines, line)
		case key == "omnium_talents":
			if len(val) > 4096 || !regexp.MustCompile(`^[0-9]+:[0-9]+(/[0-9]+:[0-9]+)*$`).MatchString(val) {
				return nil, fmt.Errorf("invalid Omnium talents")
			}
			lines = append(lines, line)
		case key == "bonus_roll_items": // Metadata only; never a worker directive.
			if len(val) > 32768 || !regexp.MustCompile(`^[0-9:/]+$`).MatchString(val) {
				return nil, fmt.Errorf("invalid bonus roll history")
			}
		case key == "server" || key == "region": // personal labels are not needed by the local calculation
		default:
			return nil, fmt.Errorf("profile field %q is not supported; import an unedited addon export", key)
		}
	}
	if p.Class == "" || p.Spec == "" || !seen["level"] || !seen["race"] || !seen["talents"] || !seen["main_hand"] {
		return nil, fmt.Errorf("character export is missing class, spec, level, race, talents or weapon data")
	}
	// Current native Augmentation exports can contain an explicit empty role.
	// Normalize only this known class/spec pair; Capability still checks the
	// snapshot's DAMAGER role, class, specialization and level independently.
	if seen["role"] && p.Role == "" {
		if p.Class != "evoker" || p.Spec != "augmentation" {
			return nil, fmt.Errorf("invalid role")
		}
		p.Role = "spell"
		for i, line := range lines {
			if line == "role=" {
				lines[i] = "role=spell"
			}
		}
	}
	p.Base = strings.Join(lines, "\n") + "\n"
	return p, nil
}

type Scenario struct {
	HealingCohort    string                      `json:"healingCohort,omitempty"`
	HealingScenario  string                      `json:"healingScenario,omitempty"`
	HealingIntensity int                         `json:"healingIntensity,omitempty"`
	HealingRuns      int                         `json:"healingRuns,omitempty"`
	HealingPolicy    string                      `json:"healingPolicy,omitempty"`
	BuildSpec        int                         `json:"buildSpec,omitempty"`
	BuildMode        string                      `json:"buildMode,omitempty"`
	AugmentationMode string                      `json:"augmentationMode,omitempty"`
	AllyItemLevel    int                         `json:"allyItemLevel,omitempty"`
	AllySkill        float64                     `json:"allySkill,omitempty"`
	Task             string                      `json:"task,omitempty"`
	KeyLevel         int                         `json:"keyLevel,omitempty"`
	RaidDifficulty   int                         `json:"raidDifficulty,omitempty"`
	HealingMode      string                      `json:"healingMode,omitempty"`
	FightStyle       string                      `json:"fightStyle,omitempty"`
	RaidBuffs        string                      `json:"raidBuffs,omitempty"`
	Bloodlust        string                      `json:"bloodlust,omitempty"`
	Encounters       []Encounter                 `json:"encounters,omitempty"`
	ItemOptions      map[string][]ItemAdjustment `json:"itemOptions,omitempty"`
	CompareMode      string                      `json:"compareMode,omitempty"`
	LoadoutSlots     map[string]string           `json:"loadoutSlots,omitempty"`
	LoadoutLevels    map[string]int              `json:"loadoutLevels,omitempty"`
	Talents          []string                    `json:"talents,omitempty"`
	Targets          int                         `json:"targets"`
	Seconds          int                         `json:"seconds"`
	Iterations       int                         `json:"iterations"`
	BudgetSeconds    int                         `json:"budgetSeconds,omitempty"`
	quick            bool
}

func DefaultScenario() Scenario {
	return Scenario{Targets: 1, Seconds: 300, Iterations: 1000, BudgetSeconds: 60}
}
func (s Scenario) Validate() error {
	if err := validateWorkflowScenario(s); err != nil {
		return err
	}
	if err := validateAugmentationSettings(s); err != nil {
		return err
	}
	if s.Task != "" && s.Task != "upgrades" && s.Task != "analysis" && !builtinTask(s.Task) {
		return fmt.Errorf("unknown calculation task")
	}
	if s.BuildSpec != 0 {
		if _, ok := wowspec.Lookup(s.BuildSpec); !ok {
			return fmt.Errorf("unsupported build specialization")
		}
	}
	if s.BuildMode != "" && !buildModes[s.BuildMode] {
		return fmt.Errorf("invalid build reference mode")
	}
	if s.HealingScenario != "" && s.HealingScenario != "tank" && s.HealingScenario != "dungeon" && s.HealingScenario != "raid" && s.HealingScenario != "replay" && s.HealingScenario != "learned" {
		return fmt.Errorf("invalid healing encounter")
	}
	if len(s.HealingCohort) > 256 || (s.HealingCohort != "" && s.HealingScenario != "learned") {
		return fmt.Errorf("invalid learned encounter selection")
	}
	if s.HealingPolicy != "" && s.HealingPolicy != "consensus" && s.HealingPolicy != "reactive" && s.HealingPolicy != "compare" {
		return fmt.Errorf("invalid healing policy")
	}
	if s.HealingIntensity < 0 || s.HealingIntensity > 200 || s.HealingRuns < 0 || s.HealingRuns > 100 {
		return fmt.Errorf("invalid healing workload or runs")
	}
	if s.KeyLevel < 0 || s.KeyLevel > 7 || s.RaidDifficulty < 0 || s.RaidDifficulty > 3 {
		return fmt.Errorf("invalid loot difficulty")
	}
	if s.HealingMode != "" && s.HealingMode != "raid" && s.HealingMode != "dungeon" {
		return fmt.Errorf("unknown healing mode")
	}
	if s.FightStyle != "" && s.FightStyle != "patchwerk" && s.FightStyle != "dungeon_slice" {
		return fmt.Errorf("unknown fight style")
	}
	if s.RaidBuffs != "" && s.RaidBuffs != "full" && s.RaidBuffs != "none" {
		return fmt.Errorf("unknown raid buff preset")
	}
	if s.CompareMode != "" && s.CompareMode != "individual" && s.CompareMode != "topgear" && s.CompareMode != "loadout" {
		return fmt.Errorf("unknown comparison mode")
	}
	if len(s.LoadoutSlots) > 0 && s.CompareMode != "loadout" {
		return fmt.Errorf("exact item placement requires a planned loadout")
	}
	if len(s.LoadoutLevels) > 0 && s.CompareMode != "loadout" {
		return fmt.Errorf("planned item levels require a planned loadout")
	}
	for id, slot := range s.LoadoutSlots {
		if id == "" || !slots[slot] {
			return fmt.Errorf("invalid planned item placement")
		}
	}
	for id, level := range s.LoadoutLevels {
		if id == "" || level <= 0 || level > 2000 {
			return fmt.Errorf("invalid planned item level")
		}
	}
	if len(s.Talents) > 16 {
		return fmt.Errorf("choose at most sixteen talent builds")
	}
	for _, v := range s.Talents {
		if len(v) > 2048 || !talentCode.MatchString(v) {
			return fmt.Errorf("invalid talent export")
		}
	}
	minimumIterations := 100
	if s.quick {
		minimumIterations = 20
	}
	if s.Targets < 1 || s.Targets > 20 || s.Seconds < 30 || s.Seconds > 600 || s.Iterations < minimumIterations || s.Iterations > 20000 {
		return fmt.Errorf("scenario must use 1–20 targets, 30–600 seconds and 100–20000 iterations")
	}
	if s.BudgetSeconds != 0 && (s.BudgetSeconds < 30 || s.BudgetSeconds > 60) {
		return fmt.Errorf("analysis time budget must be 30–60 seconds")
	}
	return nil
}

func (p *Profile) Input(s Scenario, selected []string) (string, error) {
	if err := s.Validate(); err != nil {
		return "", err
	}
	limit := 8
	if s.CompareMode == "loadout" {
		limit = 18
	}
	if len(selected) > limit {
		return "", fmt.Errorf("compare at most %d replacements per run", limit)
	}
	fightStyle := "Patchwerk"
	if s.FightStyle == "dungeon_slice" {
		fightStyle = "DungeonSlice"
		s.Seconds = 360
		s.Targets = 1
	}
	optimalRaid := 1
	if s.RaidBuffs == "none" || s.FightStyle == "dungeon_slice" {
		optimalRaid = 0
	}
	text := simcBase(p.Base) + fmt.Sprintf("\niterations=%d\nmax_time=%d\nvary_combat_length=0\ndesired_targets=%d\nthreads=2\nprofileset_work_threads=1\nprofileset_metric=dps\nfight_style=%s\ncalculate_scale_factors=0\noptimal_raid=%d\nitem_db_source=local\n", s.Iterations, s.Seconds, s.Targets, fightStyle, optimalRaid)
	if s.Bloodlust != "" {
		value := 0
		if s.Bloodlust == "on" {
			value = 1
		}
		text += fmt.Sprintf("override.bloodlust=%d\n", value)
	}
	if p.Spec == "augmentation" {
		text = strings.Replace(text, "profileset_metric=dps\n", "profileset_metric=raid_dps\n", 1)
		text += augmentationSettings(s).input()
	}
	comparisons, err := p.Comparisons(s, selected)
	if err != nil {
		return "", err
	}
	for _, c := range comparisons {
		for i, line := range comparisonLines(c) {
			operator := "="
			if i > 0 {
				operator = "+="
			}
			text += fmt.Sprintf("profileset.%s%s%s\n", c.ID, operator, line)
		}
	}
	return text, nil
}
