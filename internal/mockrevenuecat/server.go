package mockrevenuecat

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// DefaultPageSize is how many items a list response carries before paginating.
// It is small so pagination is exercised without creating many objects.
const DefaultPageSize = 2

// Options configures a Server.
type Options struct {
	// APIKey, when set, is the only bearer token accepted. Empty accepts any
	// non-empty bearer token.
	APIKey string

	// PageSize bounds list responses. Zero selects DefaultPageSize.
	PageSize int

	// RateLimitRequests makes the server answer that many requests with 429
	// before serving normally. Zero disables fault injection.
	RateLimitRequests int

	// SeedProjectID, when set, pre-creates a project with that exact
	// identifier so a container-based run has a known project to target.
	SeedProjectID string

	// SeedProjectName names the seeded project.
	SeedProjectName string
}

// Server is a stateful fake of the RevenueCat API v2 catalog.
type Server struct {
	store    *store
	options  Options
	pageSize int

	mu             sync.Mutex
	rateLimitsLeft int
	requestCount   int
}

// New returns a Server configured by opts.
func New(opts Options) *Server {
	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}

	s := &Server{
		store:          newStore(),
		options:        opts,
		pageSize:       pageSize,
		rateLimitsLeft: opts.RateLimitRequests,
	}

	if opts.SeedProjectID != "" {
		name := opts.SeedProjectName
		if name == "" {
			name = "Seeded Project"
		}
		s.store.mu.Lock()
		s.store.byKind["project"] = map[string]*object{
			opts.SeedProjectID: {
				ID:        opts.SeedProjectID,
				CreatedAt: s.store.now(),
				Fields:    map[string]any{"name": name},
			},
		}
		s.store.order["project"] = []string{opts.SeedProjectID}
		s.store.mu.Unlock()
	}

	return s
}

// CreateProject adds a project to the store and returns its identifier.
// Projects are not creatable through the API, so tests and the standalone
// binary seed them directly.
func (s *Server) CreateProject(name string) string {
	obj := s.store.create("project", "", map[string]any{"name": name})
	return obj.ID
}

// CreateTestStoreApp adds a Test Store app to a project and returns its
// identifier. RevenueCat makes a project's Test Store itself and its API cannot
// create one, so tests seed it directly, as they do projects.
func (s *Server) CreateTestStoreApp(projectID, name string) string {
	obj := s.store.create("app", projectID, map[string]any{"name": name, "type": "test_store"})
	return obj.ID
}

// validateProductCreate mirrors the real API requiring a title on a product of
// a Test Store app, which RevenueCat simulates and so cannot name from a store.
func (s *Server) validateProductCreate(body map[string]any) *validationError {
	appID, _ := body["app_id"].(string)
	app, ok := s.store.get("app", "", appID)
	if !ok || app.Fields["type"] != "test_store" {
		return nil
	}
	if title, _ := body["title"].(string); title == "" {
		return &validationError{code: "parameter_error", message: "'title' is required for Test Store products"}
	}
	return nil
}

// ProductTitle returns the title the mock holds for a product, which no HTTP
// response reveals, so a test can assert the provider sent it.
func (s *Server) ProductTitle(productID string) string {
	obj, ok := s.store.get("product", "", productID)
	if !ok {
		return ""
	}
	title, _ := obj.Fields[productTitleField].(string)
	return title
}

// RequestCount reports how many requests the server has served, so a test can
// assert that a retry actually happened.
func (s *Server) RequestCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requestCount
}

// AttachedProducts reports the products attached to a parent, letting a test
// inspect the relationship without going through the API.
func (s *Server) AttachedProducts(kind, parentID string) []string {
	out := []string{}
	for _, entry := range s.store.attached(kind, parentID) {
		out = append(out, entry.ProductID)
	}
	return out
}

// Exists reports whether an object of the given kind and identifier is present,
// letting a test assert that a destroy really removed something.
func (s *Server) Exists(kind, id string) bool {
	_, ok := s.store.get(kind, "", id)
	return ok
}

// Count reports how many objects of a kind exist.
func (s *Server) Count(kind string) int {
	return len(s.store.list(kind, nil))
}

// PlayServiceAccountCredentials returns the credential the mock holds for an
// app, which no HTTP response ever reveals. A test asserting a rotation needs
// this: the configured flag stays true across a rotation, so it alone cannot
// tell a key that was replaced from one that silently was not.
func (s *Server) PlayServiceAccountCredentials(appID string) string {
	obj, ok := s.store.get("app", "", appID)
	if !ok {
		return ""
	}
	credentials, _ := obj.Fields[playCredentialsField].(string)
	return credentials
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// The health endpoint is deliberately outside authentication so a
	// container orchestrator can probe it without a credential.
	if r.URL.Path == "/health" || r.URL.Path == "/v2/health" {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok"}`)
		return
	}

	s.mu.Lock()
	s.requestCount++
	rateLimit := s.rateLimitsLeft > 0
	if rateLimit {
		s.rateLimitsLeft--
	}
	s.mu.Unlock()

	if rateLimit {
		w.Header().Set("Retry-After", "1")
		s.writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
		return
	}

	if !s.authorized(r) {
		s.writeError(w, http.StatusUnauthorized, "unauthorized", "a bearer token is required")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/v2")
	segments := splitPath(path)

	if len(segments) == 0 || segments[0] != "projects" {
		s.writeError(w, http.StatusNotFound, "not_found", "unknown endpoint "+r.URL.Path)
		return
	}

	s.routeProjects(w, r, segments[1:])
}

// authorized enforces that a bearer token is present, and matches the
// configured key when one is set. Without this a provider that dropped its
// Authorization header would pass every end-to-end test.
func (s *Server) authorized(r *http.Request) bool {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return false
	}

	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if token == "" {
		return false
	}
	if s.options.APIKey != "" && token != s.options.APIKey {
		return false
	}
	return true
}

func splitPath(path string) []string {
	var segments []string
	for _, segment := range strings.Split(path, "/") {
		if segment != "" {
			segments = append(segments, segment)
		}
	}
	return segments
}

func (s *Server) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type":    code,
		"code":    code,
		"message": message,
	})
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (s *Server) notFound(w http.ResponseWriter, what string) {
	s.writeError(w, http.StatusNotFound, "not_found", "no such "+what)
}

// decode reads a JSON request body into a map.
func decode(r *http.Request) (map[string]any, error) {
	body := map[string]any{}
	if r.Body == nil {
		return body, nil
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&body); err != nil {
		return nil, err
	}
	return body, nil
}

// paginate returns the page of items starting after the given cursor, plus the
// next cursor when more remain.
func paginate[T any](items []T, idOf func(T) string, startingAfter string, pageSize int) (page []T, nextCursor string) {
	start := 0
	if startingAfter != "" {
		for i, item := range items {
			if idOf(item) == startingAfter {
				start = i + 1
				break
			}
		}
	}

	if start >= len(items) {
		return nil, ""
	}

	end := start + pageSize
	if end >= len(items) {
		return items[start:], ""
	}
	return items[start:end], idOf(items[end-1])
}

// listEnvelope renders a paginated list response in the documented shape.
func (s *Server) listEnvelope(w http.ResponseWriter, r *http.Request, basePath string, items []map[string]any) {
	cursor := r.URL.Query().Get("starting_after")

	page, next := paginate(items, func(item map[string]any) string {
		id, _ := item["id"].(string)
		return id
	}, cursor, s.pageSize)

	if page == nil {
		page = []map[string]any{}
	}

	body := map[string]any{
		"object": "list",
		"items":  page,
		"url":    basePath,
	}
	if next != "" {
		body["next_page"] = basePath + "?starting_after=" + next
	}

	s.writeJSON(w, http.StatusOK, body)
}
