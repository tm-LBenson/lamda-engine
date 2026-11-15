package market

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func (s *Store) Recipes(key string) ([]Recipe, error) {
	return s.recipesFor(s, key)
}
func (s *Store) recipesFor(profile *Store, key string) ([]Recipe, error) {
	var observed, custom []Recipe
	if err := s.documents(key, "recipe", &observed); err != nil {
		return nil, err
	}
	if err := profile.documents(key, "recipe_override", &custom); err != nil {
		return nil, err
	}
	byID := map[string]Recipe{}
	for _, r := range observed {
		r.Custom = false
		byID[r.ID] = r
	}
	for _, r := range custom {
		r.Custom = true
		byID[r.ID] = r
	}
	out := make([]Recipe, 0, len(byID))
	for _, r := range byID {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
func (s *Store) SaveRecipe(key string, r Recipe) error {
	p := strings.Split(key, "/")
	if len(p) != 4 {
		return fmt.Errorf("invalid market")
	}
	e := Event{Version: 1, ID: "manual", Market: Identity{p[0], p[1], p[2], p[3]}, At: time.Now().Unix(), Kind: "recipes", Recipes: []Recipe{r}}
	if err := e.Validate(time.Now()); err != nil {
		return err
	}
	if r.Name == "" || len(r.Name) > 500 {
		return fmt.Errorf("give the recipe a name")
	}
	r.Custom = true
	r.Source = "Custom model entered in LamdaMarket"
	_, err := s.db.Exec(`INSERT INTO documents VALUES(?,'recipe_override',?,?,?) ON CONFLICT(market,kind,id) DO UPDATE SET at=excluded.at,body=excluded.body`, key, r.ID, e.At, encoded(r))
	return err
}
func (s *Store) ResetRecipe(key, id string) error {
	if !validID(id) {
		return fmt.Errorf("invalid recipe")
	}
	_, err := s.db.Exec(`DELETE FROM documents WHERE market=? AND kind='recipe_override' AND id=?`, key, id)
	return err
}
