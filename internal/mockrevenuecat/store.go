// Package mockrevenuecat implements a stateful in-memory fake of the RevenueCat
// REST API v2 catalog.
//
// It exists so the provider can be driven by a real terraform binary offline
// and deterministically. It is deliberately stateful rather than a canned
// response stub: a stub cannot notice that a provider's Create and Read
// disagree, or that a delete never happened, so it could never fail for a good
// reason.
//
// The fake encodes the same assumed API contract as internal/revenuecat.
// Agreement between them demonstrates that the provider is self-consistent
// under Terraform's contract; it does not demonstrate fidelity to the real
// RevenueCat service.
package mockrevenuecat

import (
	"fmt"
	"sync"
	"time"
)

// object is the common shape of everything the store holds.
type object struct {
	ID        string
	CreatedAt int64

	// ProjectID scopes the object to a project. Every catalog object has one.
	ProjectID string

	// Fields carries the type-specific attributes, keyed by their JSON name.
	// Keeping them in a map lets one set of handlers serve every object type
	// without a struct per type duplicating what internal/revenuecat already
	// declares.
	Fields map[string]any
}

// clone returns a deep-enough copy for serving a response, so a caller cannot
// mutate stored state by holding onto a returned object.
func (o *object) clone() *object {
	fields := make(map[string]any, len(o.Fields))
	for k, v := range o.Fields {
		if nested, ok := v.(map[string]string); ok {
			copied := make(map[string]string, len(nested))
			for nk, nv := range nested {
				copied[nk] = nv
			}
			fields[k] = copied
			continue
		}
		fields[k] = v
	}
	return &object{ID: o.ID, CreatedAt: o.CreatedAt, ProjectID: o.ProjectID, Fields: fields}
}

// attachment records one product attached to an entitlement or a package.
type attachment struct {
	ProductID           string
	EligibilityCriteria string
}

// store holds every object the mock knows about.
type store struct {
	mu sync.Mutex

	// byKind maps an object kind ("app", "entitlement", ...) to its objects,
	// keyed by identifier.
	byKind map[string]map[string]*object

	// order preserves insertion order per kind, so list responses and their
	// pagination are deterministic.
	order map[string][]string

	// attachments maps "<kind>:<parentID>" to its attached products, in
	// insertion order.
	attachments map[string][]attachment

	// nextID backs identifier generation per kind.
	nextID map[string]int

	// now supplies creation timestamps; replaceable so tests are deterministic.
	now func() int64
}

func newStore() *store {
	return &store{
		byKind:      map[string]map[string]*object{},
		order:       map[string][]string{},
		attachments: map[string][]attachment{},
		nextID:      map[string]int{},
		now:         func() int64 { return time.Now().UnixMilli() },
	}
}

// idPrefix gives each kind a distinguishable identifier prefix, mirroring how
// RevenueCat identifiers are recognizable by type. A test that mixes up two
// identifiers then fails visibly rather than silently matching.
var idPrefix = map[string]string{
	"project":     "proj",
	"app":         "app",
	"product":     "prod",
	"entitlement": "entl",
	"offering":    "ofrng",
	"package":     "pkg",
}

// create stores a new object of the given kind and returns a copy of it.
func (s *store) create(kind, projectID string, fields map[string]any) *object {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.createLocked(kind, projectID, fields)
}

func (s *store) createLocked(kind, projectID string, fields map[string]any) *object {
	s.nextID[kind]++

	prefix, ok := idPrefix[kind]
	if !ok {
		prefix = kind
	}

	obj := &object{
		ID:        fmt.Sprintf("%s%d%s", prefix, s.nextID[kind], randomSuffix()),
		CreatedAt: s.now(),
		ProjectID: projectID,
		Fields:    fields,
	}
	if obj.Fields == nil {
		obj.Fields = map[string]any{}
	}

	if s.byKind[kind] == nil {
		s.byKind[kind] = map[string]*object{}
	}
	s.byKind[kind][obj.ID] = obj
	s.order[kind] = append(s.order[kind], obj.ID)

	return obj.clone()
}

// get returns the object of the given kind and identifier, scoped to a project.
// A projectID of "" skips the scope check. Objects addressed through the wrong
// parent are reported as absent, so a provider sending a mismatched project
// fails rather than passing by accident.
func (s *store) get(kind, projectID, id string) (*object, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getLocked(kind, projectID, id)
}

func (s *store) getLocked(kind, projectID, id string) (*object, bool) {
	obj, ok := s.byKind[kind][id]
	if !ok {
		return nil, false
	}
	if projectID != "" && obj.ProjectID != projectID {
		return nil, false
	}
	return obj.clone(), true
}

// update applies the supplied fields to an existing object, leaving fields the
// caller did not name untouched, and returns the updated object.
func (s *store) update(kind, projectID, id string, fields map[string]any) (*object, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	obj, ok := s.byKind[kind][id]
	if !ok || (projectID != "" && obj.ProjectID != projectID) {
		return nil, false
	}

	for name, value := range fields {
		obj.Fields[name] = value
	}
	return obj.clone(), true
}

// delete removes an object and any attachments recorded against it.
func (s *store) delete(kind, projectID, id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	obj, ok := s.byKind[kind][id]
	if !ok || (projectID != "" && obj.ProjectID != projectID) {
		return false
	}

	delete(s.byKind[kind], id)
	remaining := make([]string, 0, len(s.order[kind]))
	for _, existing := range s.order[kind] {
		if existing != id {
			remaining = append(remaining, existing)
		}
	}
	s.order[kind] = remaining
	delete(s.attachments, attachmentKey(kind, id))

	return true
}

// list returns every object of a kind matching the supplied filter, in
// insertion order.
func (s *store) list(kind string, match func(*object) bool) []*object {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []*object
	for _, id := range s.order[kind] {
		obj := s.byKind[kind][id]
		if obj == nil {
			continue
		}
		if match != nil && !match(obj) {
			continue
		}
		out = append(out, obj.clone())
	}
	return out
}

// hasCurrentOffering reports whether any offering of the project is current.
func (s *store) hasCurrentOffering(projectID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, obj := range s.byKind["offering"] {
		if current, _ := obj.Fields["is_current"].(bool); current && obj.ProjectID == projectID {
			return true
		}
	}
	return false
}

// clearOtherCurrentOfferings clears is_current on every offering of the
// project but keepID.
func (s *store) clearOtherCurrentOfferings(projectID, keepID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, obj := range s.byKind["offering"] {
		if id != keepID && obj.ProjectID == projectID {
			obj.Fields["is_current"] = false
		}
	}
}

func attachmentKey(kind, parentID string) string {
	return kind + ":" + parentID
}

// attach records products against a parent. Re-attaching a product already
// attached updates its eligibility criteria rather than duplicating it.
func (s *store) attach(kind, parentID string, products []attachment) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := attachmentKey(kind, parentID)
	existing := s.attachments[key]

	for _, product := range products {
		var found bool
		for i := range existing {
			if existing[i].ProductID == product.ProductID {
				existing[i].EligibilityCriteria = product.EligibilityCriteria
				found = true
				break
			}
		}
		if !found {
			existing = append(existing, product)
		}
	}
	s.attachments[key] = existing
}

// detach removes the named products from a parent, leaving the rest attached.
func (s *store) detach(kind, parentID string, productIDs []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	removing := make(map[string]struct{}, len(productIDs))
	for _, id := range productIDs {
		removing[id] = struct{}{}
	}

	key := attachmentKey(kind, parentID)
	remaining := make([]attachment, 0, len(s.attachments[key]))
	for _, existing := range s.attachments[key] {
		if _, drop := removing[existing.ProductID]; !drop {
			remaining = append(remaining, existing)
		}
	}
	s.attachments[key] = remaining
}

// attached returns the products currently attached to a parent.
func (s *store) attached(kind, parentID string) []attachment {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]attachment, len(s.attachments[attachmentKey(kind, parentID)]))
	copy(out, s.attachments[attachmentKey(kind, parentID)])
	return out
}
