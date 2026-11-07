package overlays

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

// Logic is local to an overlay. Expressions operate only on captured facts and
// explicit options; they cannot execute scripts or interact with the game.
type Logic struct {
	Values  []NamedValue  `json:"values,omitempty"`
	Options []LogicOption `json:"options,omitempty"`
}
type NamedValue struct {
	Remember *RememberValue `json:"remember,omitempty"`
	Key      string         `json:"key"`
	Name     string         `json:"name"`
	Expr     ValueExpr      `json:"expr"`
}
type LogicOption struct {
	Shared  bool     `json:"shared,omitempty"`
	Key     string   `json:"key"`
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Default any      `json:"default"`
	Value   any      `json:"value,omitempty"`
	Min     *float64 `json:"min,omitempty"`
	Max     *float64 `json:"max,omitempty"`
	Choices []string `json:"choices,omitempty"`
}
type ValueExpr struct {
	Kind   string         `json:"kind"`
	Value  any            `json:"value,omitempty"`
	Key    string         `json:"key,omitempty"`
	Source map[string]any `json:"source,omitempty"`
	Field  string         `json:"field,omitempty"`
	Op     string         `json:"op,omitempty"`
	Args   []ValueExpr    `json:"args,omitempty"`
	Check  map[string]any `json:"check,omitempty"`
}

func (l *Logic) UnmarshalJSON(b []byte) error {
	type plain Logic
	var v plain
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&v); err != nil {
		return err
	}
	*l = Logic(v)
	return nil
}
func (o *LogicOption) UnmarshalJSON(b []byte) error {
	type plain LogicOption
	var v plain
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&v); err != nil {
		return err
	}
	*o = LogicOption(v)
	return nil
}
func (n *NamedValue) UnmarshalJSON(b []byte) error {
	type plain NamedValue
	var v plain
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&v); err != nil {
		return err
	}
	*n = NamedValue(v)
	return nil
}
func (e *ValueExpr) UnmarshalJSON(b []byte) error {
	type plain ValueExpr
	var v plain
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return err
	}
	*e = ValueExpr(v)
	return nil
}

var logicKey = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,31}$`)
var namedValueToken = regexp.MustCompile(`\{value:([A-Za-z][A-Za-z0-9_]{0,31})\}`)

const logicLimit = 1e12
const logicBudget = 1024

type logicValue struct {
	kind    string
	number  float64
	text    string
	boolean bool
}

func logicNumber(n float64) logicValue {
	if !bounded(n, -logicLimit, logicLimit) {
		return logicValue{}
	}
	if n == 0 {
		n = 0
	}
	return logicValue{kind: "number", number: n}
}
func logicLiteral(v any) logicValue {
	switch n := v.(type) {
	case float64:
		return logicNumber(n)
	case int:
		return logicNumber(float64(n))
	case json.Number:
		f, err := n.Float64()
		if err == nil {
			return logicNumber(f)
		}
	case bool:
		return logicValue{kind: "boolean", boolean: n}
	case string:
		if utf8.RuneCountInString(n) <= 512 && !strings.ContainsRune(n, 0) {
			return logicValue{kind: "text", text: n}
		}
	}
	return logicValue{}
}
func (v logicValue) display() string {
	switch v.kind {
	case "number":
		return strconv.FormatFloat(v.number, 'f', -1, 64)
	case "text":
		return v.text
	case "boolean":
		if v.boolean {
			return "true"
		}
		return "false"
	}
	return "—"
}
func (l *Logic) named(key string) *NamedValue {
	if l != nil {
		for i := range l.Values {
			if l.Values[i].Key == key {
				return &l.Values[i]
			}
		}
	}
	return nil
}
func (l *Logic) option(key string) *LogicOption {
	if l != nil {
		for i := range l.Options {
			if l.Options[i].Key == key {
				return &l.Options[i]
			}
		}
	}
	return nil
}
func validLogicLabel(v string) bool {
	return strings.TrimSpace(v) != "" && utf8.RuneCountInString(v) <= 60 && !strings.ContainsAny(v, "\x00\r\n")
}
func (o LogicOption) accepts(raw any) bool {
	v := logicLiteral(raw)
	switch o.Type {
	case "number":
		return v.kind == "number" && (o.Min == nil || v.number >= *o.Min) && (o.Max == nil || v.number <= *o.Max)
	case "boolean":
		return v.kind == "boolean"
	case "text":
		return v.kind == "text" && utf8.RuneCountInString(v.text) <= 120
	case "choice":
		return v.kind == "text" && one(v.text, o.Choices...)
	}
	return false
}
func validLogic(l *Logic) bool {
	if l == nil {
		return true
	}
	if len(l.Values) > 16 || len(l.Options) > 16 {
		return false
	}
	names := map[string]bool{}
	for _, o := range l.Options {
		if !logicKey.MatchString(o.Key) || names["o:"+o.Key] || !validLogicLabel(o.Name) || !one(o.Type, "number", "boolean", "text", "choice") {
			return false
		}
		names["o:"+o.Key] = true
		if o.Min != nil && !bounded(*o.Min, -logicLimit, logicLimit) || o.Max != nil && !bounded(*o.Max, -logicLimit, logicLimit) || o.Min != nil && o.Max != nil && *o.Min > *o.Max {
			return false
		}
		if o.Type != "number" && (o.Min != nil || o.Max != nil) || o.Type != "choice" && len(o.Choices) > 0 {
			return false
		}
		if o.Type == "choice" {
			seen := map[string]bool{}
			if len(o.Choices) == 0 || len(o.Choices) > 20 {
				return false
			}
			for _, s := range o.Choices {
				if !validLogicLabel(s) || seen[s] {
					return false
				}
				seen[s] = true
			}
		}
		if !o.accepts(o.Default) || o.Value != nil && !o.accepts(o.Value) {
			return false
		}
	}
	for _, v := range l.Values {
		if !logicKey.MatchString(v.Key) || names["v:"+v.Key] || !validLogicLabel(v.Name) {
			return false
		}
		names["v:"+v.Key] = true
		count := 0
		if !validValueExpr(v.Expr, 1, &count) {
			return false
		}
		if !validRemember(v.Remember) {
			return false
		}
	}
	// Bound the expanded graph too: short definitions can otherwise double their
	// work at every reference. Cycles and missing references never reach runtime.
	for _, v := range l.Values {
		budget := logicBudget
		if !validValueGraph(v.Expr, l, map[string]bool{v.Key: true}, 0, &budget) {
			return false
		}
	}
	return true
}
func validValueExpr(e ValueExpr, depth int, count *int) bool {
	*count++
	if depth > 8 || *count > 64 {
		return false
	}
	switch e.Kind {
	case "literal":
		return logicLiteral(e.Value).kind != "" && e.Key == "" && len(e.Source) == 0 && e.Field == "" && e.Op == "" && len(e.Args) == 0 && len(e.Check) == 0
	case "value", "option":
		return logicKey.MatchString(e.Key) && e.Value == nil && len(e.Source) == 0 && e.Field == "" && e.Op == "" && len(e.Args) == 0 && len(e.Check) == 0
	case "data":
		if e.Value != nil || e.Key != "" || e.Op != "" || len(e.Args) > 0 || len(e.Check) > 0 || !validLogicData(e.Source, e.Field) {
			return false
		}
		return true
	case "predicate":
		if e.Value != nil || e.Key != "" || len(e.Source) > 0 || e.Field != "" || e.Op != "" || len(e.Args) > 0 {
			return false
		}
		count := 0
		ok := validPredicate(e.Check, 0, &count)
		walkPredicate(e.Check, func(p map[string]any) {
			if str(p, "condition") == "value" {
				ok = false
			}
		})
		return ok
	case "op":
		if e.Value != nil || e.Key != "" || len(e.Source) > 0 || e.Field != "" || len(e.Check) > 0 {
			return false
		}
		lo, hi := 2, 2
		switch e.Op {
		case "abs", "floor", "ceil", "round", "neg", "not":
			lo, hi = 1, 1
		case "clamp", "choose":
			lo, hi = 3, 3
		case "min", "max", "all", "any", "concat":
			lo, hi = 2, 8
		case "add", "sub", "mul", "div", "mod", "eq", "ne", "lt", "lte", "gt", "gte":
		default:
			return false
		}
		if len(e.Args) < lo || len(e.Args) > hi {
			return false
		}
		for _, a := range e.Args {
			if !validValueExpr(a, depth+1, count) {
				return false
			}
		}
		return true
	}
	return false
}
func validValueGraph(e ValueExpr, l *Logic, visiting map[string]bool, depth int, budget *int) bool {
	*budget--
	if *budget < 0 || depth > 32 {
		return false
	}
	if e.Kind == "option" {
		return l.option(e.Key) != nil
	}
	if e.Kind == "value" {
		v := l.named(e.Key)
		if v != nil && v.Remember != nil {
			return true
		}
		if v == nil || visiting[e.Key] {
			return false
		}
		visiting[e.Key] = true
		ok := validValueGraph(v.Expr, l, visiting, depth+1, budget)
		delete(visiting, e.Key)
		return ok
	}
	for _, a := range e.Args {
		if !validValueGraph(a, l, visiting, depth+1, budget) {
			return false
		}
	}
	return true
}
func validLogicData(p map[string]any, field string) bool {
	if len(p) == 0 || !validShowData(p) || one(str(p, "dataSource"), "", "auto", "value") {
		return false
	}
	for key := range p {
		if !one(key, "dataSource", "dataID", "dataUnit", "dataResource", "dataProvider", "dataText", "dataKind", "dataTimelineSource", "dataTalentKind") {
			return false
		}
	}
	return displayToken.MatchString("{"+field+"}") && displayToken.FindString("{"+field+"}") == "{"+field+"}"
}

type valueEvaluation struct {
	logic    *Logic
	input    *lookup.OverlayInput
	actor    string
	trigger  *triggerContext
	budget   int
	visiting map[string]bool
	memo     map[string]logicValue
}

func newValueEvaluation(l *Logic, p *lookup.OverlayInput, actor string, c *triggerContext) *valueEvaluation {
	return &valueEvaluation{logic: l, input: p, actor: actor, trigger: c, budget: logicBudget, visiting: map[string]bool{}, memo: map[string]logicValue{}}
}
func (e *valueEvaluation) named(key string) logicValue {
	if v, ok := e.memo[key]; ok {
		return v
	}
	if e.visiting[key] {
		return logicValue{}
	}
	n := e.logic.named(key)
	if n == nil {
		return logicValue{}
	}
	if n.Remember != nil {
		if e.trigger != nil && e.trigger.Memory != nil {
			if v, ok := e.trigger.Memory.Values[key]; ok {
				return v
			}
		}
		return logicLiteral(n.Remember.Initial)
	}
	e.visiting[key] = true
	v := e.eval(n.Expr, 0)
	delete(e.visiting, key)
	e.memo[key] = v
	return v
}
func (e *valueEvaluation) option(key string) logicValue {
	o := e.logic.option(key)
	if o == nil {
		return logicValue{}
	}
	if o.Value != nil {
		return logicLiteral(o.Value)
	}
	return logicLiteral(o.Default)
}
func (e *valueEvaluation) eval(x ValueExpr, depth int) logicValue {
	e.budget--
	if e.budget < 0 || depth > 32 {
		return logicValue{}
	}
	switch x.Kind {
	case "literal":
		return logicLiteral(x.Value)
	case "value":
		return e.named(x.Key)
	case "option":
		return e.option(x.Key)
	case "data":
		return logicData(x.Source, x.Field, e.input, e.actor, e.trigger)
	case "predicate":
		v := predicate(x.Check, e.input, e.actor, e.trigger)
		if v == unknown {
			return logicValue{}
		}
		return logicLiteral(v == yes)
	case "op":
		if x.Op == "choose" && len(x.Args) == 3 {
			test := e.eval(x.Args[0], depth+1)
			if test.kind != "boolean" {
				return logicValue{}
			}
			if test.boolean {
				return e.eval(x.Args[1], depth+1)
			}
			return e.eval(x.Args[2], depth+1)
		}
		args := make([]logicValue, 0, len(x.Args))
		for _, a := range x.Args {
			v := e.eval(a, depth+1)
			if x.Op == "all" && v.kind == "boolean" && !v.boolean {
				return logicLiteral(false)
			}
			if x.Op == "any" && v.kind == "boolean" && v.boolean {
				return logicLiteral(true)
			}
			args = append(args, v)
		}
		return applyValueOp(x.Op, args)
	}
	return logicValue{}
}
func applyValueOp(op string, args []logicValue) logicValue {
	for _, a := range args {
		if a.kind == "" {
			return logicValue{}
		}
	}
	if len(args) == 0 {
		return logicValue{}
	}
	if op == "all" || op == "any" || op == "not" {
		for _, a := range args {
			if a.kind != "boolean" {
				return logicValue{}
			}
		}
		if op == "not" && len(args) == 1 {
			return logicLiteral(!args[0].boolean)
		}
		for _, a := range args {
			if op == "all" && !a.boolean {
				return logicLiteral(false)
			}
			if op == "any" && a.boolean {
				return logicLiteral(true)
			}
		}
		return logicLiteral(op == "all")
	}
	if one(op, "eq", "ne", "lt", "lte", "gt", "gte") && len(args) == 2 {
		a, b := args[0], args[1]
		if a.kind != b.kind {
			return logicValue{}
		}
		if a.kind == "number" {
			return logicLiteral(compare(a.number, b.number, op))
		}
		if !one(op, "eq", "ne") {
			return logicValue{}
		}
		equal := a.text == b.text
		if a.kind == "boolean" {
			equal = a.boolean == b.boolean
		}
		return logicLiteral(equal == (op == "eq"))
	}
	if op == "concat" {
		parts := make([]string, len(args))
		for i, v := range args {
			parts[i] = v.display()
		}
		return logicLiteral(strings.Join(parts, ""))
	}
	for _, a := range args {
		if a.kind != "number" {
			return logicValue{}
		}
	}
	a := args[0].number
	var b float64
	if len(args) > 1 {
		b = args[1].number
	}
	switch op {
	case "add":
		return logicNumber(a + b)
	case "sub":
		return logicNumber(a - b)
	case "mul":
		return logicNumber(a * b)
	case "div":
		if b != 0 {
			return logicNumber(a / b)
		}
	case "mod":
		if b != 0 {
			return logicNumber(math.Mod(a, b))
		}
	case "abs":
		return logicNumber(math.Abs(a))
	case "neg":
		return logicNumber(-a)
	case "floor":
		return logicNumber(math.Floor(a))
	case "ceil":
		return logicNumber(math.Ceil(a))
	case "round":
		return logicNumber(math.Floor(a + .5))
	case "min", "max":
		for _, v := range args[1:] {
			if op == "min" {
				a = math.Min(a, v.number)
			} else {
				a = math.Max(a, v.number)
			}
		}
		return logicNumber(a)
	case "clamp":
		if len(args) == 3 && b <= args[2].number {
			return logicNumber(math.Max(b, math.Min(args[2].number, a)))
		}
	}
	return logicValue{}
}
func logicError(name string) error { return fmt.Errorf("Invalid values or settings in %s", name) }
