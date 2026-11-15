package market

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

type profileContextKey struct{}

func requestProfile(r *http.Request, fallback *Store) *Store {
	if p, ok := r.Context().Value(profileContextKey{}).(*Store); ok {
		return p
	}
	return fallback
}
func withProfile(r *http.Request, p *Store) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), profileContextKey{}, p))
}
func marketRevision(shared, profile *Store) string {
	v := strconv.FormatUint(shared.revision.Load(), 10)
	if shared != profile {
		v += "/" + strconv.FormatUint(profile.revision.Load(), 10)
	}
	return v
}

// Profiles keeps private characters, ledgers, research settings and lists out of
// the shared observations database. Paths derive only from verified identities.
type Profiles struct {
	mu          sync.Mutex
	dir, client string
	stores      map[string]*Store
}

func OpenProfiles(dir, client string) (*Profiles, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &Profiles{dir: dir, client: client, stores: map[string]*Store{}}, nil
}
func (p *Profiles) Get(subject string) (*Store, error) {
	if subject == "" {
		return nil, fmt.Errorf("missing verified identity")
	}
	id := fmt.Sprintf("%x", sha256.Sum256([]byte(subject)))
	p.mu.Lock()
	defer p.mu.Unlock()
	if s := p.stores[id]; s != nil {
		return s, nil
	}
	if len(p.stores) >= 512 {
		return nil, fmt.Errorf("profile capacity reached")
	}
	s, err := Open(filepath.Join(p.dir, id+".sqlite"), p.client)
	if err != nil {
		return nil, err
	}
	p.stores[id] = s
	return s, nil
}
func (p *Profiles) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	var first error
	for _, s := range p.stores {
		if err := s.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}
