package market

import (
	"path/filepath"
	"reflect"
	"testing"
)

func listByID(t *testing.T, s *Store, key, owner, id string) FavoriteList {
	t.Helper()
	lists, err := s.Lists(key, owner)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range lists {
		if l.ID == id {
			return l
		}
	}
	t.Fatalf("missing list %s", id)
	return FavoriteList{}
}
func TestListsOwnershipMembershipAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lists.sqlite")
	s, err := Open(path, "forever")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { s.Close() }()
	key := fixtureMarket.Key()
	for _, owner := range []string{"alice", "bob"} {
		if err = s.SaveList(key, owner, "metal", "Operation Heavy Metal", true); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.SaveList(key, "alice", "tinker", "Tinker time", true); err != nil {
		t.Fatal(err)
	}
	if err = s.ListItems(key, "alice", []string{"metal", "tinker"}, []string{"ore", "bar", "stone"}, "add"); err != nil {
		t.Fatal(err)
	}
	if len(listByID(t, s, key, "bob", "metal").Items) != 0 {
		t.Fatal("another user's membership leaked")
	}
	if err = s.ListItems(key, "bob", []string{"tinker"}, []string{"ore"}, "add"); err == nil {
		t.Fatal("foreign list writable")
	}
	// A move names only one visible target, preserving every filtered-out card.
	if err = s.MoveListItem(key, "alice", "metal", "stone", "ore"); err != nil {
		t.Fatal(err)
	}
	if got := listByID(t, s, key, "alice", "metal").Items; !reflect.DeepEqual(got, []string{"stone", "ore", "bar"}) {
		t.Fatal(got)
	}
	if err = s.ListItems(key, "alice", []string{"metal"}, []string{"ore"}, "assign"); err != nil {
		t.Fatal(err)
	}
	if got := listByID(t, s, key, "alice", "tinker").Items; !reflect.DeepEqual(got, []string{"bar", "stone"}) {
		t.Fatal(got)
	}
	if err = s.SaveList(key, "alice", "metal", "Metals", false); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveList(key, "alice", "another", " metals ", true); err == nil {
		t.Fatal("duplicate list name accepted")
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path, "forever")
	if err != nil {
		t.Fatal(err)
	}
	l := listByID(t, s, key, "alice", "metal")
	if l.Name != "Metals" || !reflect.DeepEqual(l.Items, []string{"stone", "ore", "bar"}) {
		t.Fatal(l)
	}
	if err = s.DeleteList(key, "alice", "tinker"); err != nil {
		t.Fatal(err)
	}
	if len(listByID(t, s, key, "bob", "metal").Items) != 0 {
		t.Fatal("delete touched another user")
	}
}
func TestListsLegacyMigrationRollbackAndUnquotedItems(t *testing.T) {
	s := fixture(t)
	key := fixtureMarket.Key()
	_, err := s.db.Exec(`INSERT INTO watches VALUES(?,?)`, key, "ore")
	if err != nil {
		t.Fatal(err)
	}
	if got := listByID(t, s, key, "local", DefaultListID).Items; !reflect.DeepEqual(got, []string{"ore"}) {
		t.Fatal(got)
	}
	if len(listByID(t, s, key, "alice", DefaultListID).Items) != 0 {
		t.Fatal("legacy watches visible to another user")
	}
	if err = s.SaveList(key, "local", "test", "Testing", true); err != nil {
		t.Fatal(err)
	}
	if err = s.ListItems(key, "local", []string{"test"}, []string{"never-scanned"}, "add"); err != nil {
		t.Fatal(err)
	}
	if err = s.ListItems(key, "local", []string{"missing"}, []string{"ore"}, "assign"); err == nil {
		t.Fatal("unknown list accepted")
	}
	if got := listByID(t, s, key, "local", DefaultListID).Items; !reflect.DeepEqual(got, []string{"ore"}) {
		t.Fatal("failed assign lost membership", got)
	}
	if err = s.Watch(key, "ore", false); err != nil {
		t.Fatal(err)
	}
	if len(listByID(t, s, key, "local", DefaultListID).Items) != 0 {
		t.Fatal("legacy remove resurrected")
	}
	if err = s.DeleteList(key, "local", DefaultListID); err == nil {
		t.Fatal("default deleted")
	}
	ingest(t, s, scan("prices", fixtureNow.Unix(), []Lot{{Item: "never-scanned", Price: 12, Quantity: 1, Count: 1}}))
	if err = s.DeleteList(key, "local", "test"); err != nil {
		t.Fatal(err)
	}
	cards, err := s.Cards(key, 0)
	if err != nil || len(cards) != 1 || cards[0].Price != 12 {
		t.Fatal("list deletion touched price history", cards, err)
	}
}
