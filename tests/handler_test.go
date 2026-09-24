package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"expert-listing/internal/handler"
	"expert-listing/internal/model"
	"expert-listing/internal/repository"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ─── Mock store ───────────────────────────────────────────────────────────

// mockStore is an in-memory ListingStore implementation used by handler tests.
type mockStore struct {
	listings map[uuid.UUID]*model.Listing
	err      error // when set, every method returns this error
}

func newMockStore() *mockStore {
	return &mockStore{listings: make(map[uuid.UUID]*model.Listing)}
}

func (m *mockStore) Create(_ context.Context, in model.CreateListingInput) (*model.Listing, error) {
	if m.err != nil {
		return nil, m.err
	}
	l := &model.Listing{
		ID:        uuid.New(),
		Title:     in.Title,
		Price:     in.Price,
		Type:      in.Type,
		Bedrooms:  in.Bedrooms,
		Latitude:  in.Latitude,
		Longitude: in.Longitude,
		AgentID:   in.AgentID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.listings[l.ID] = l
	return l, nil
}

func (m *mockStore) GetByID(_ context.Context, id uuid.UUID) (*model.Listing, error) {
	if m.err != nil {
		return nil, m.err
	}
	l, ok := m.listings[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return l, nil
}

func (m *mockStore) Update(_ context.Context, id uuid.UUID, in model.UpdateListingInput) (*model.Listing, error) {
	if m.err != nil {
		return nil, m.err
	}
	l, ok := m.listings[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	if in.Title != nil {
		l.Title = *in.Title
	}
	if in.Price != nil {
		l.Price = *in.Price
	}
	if in.Type != nil {
		l.Type = *in.Type
	}
	if in.Bedrooms != nil {
		l.Bedrooms = *in.Bedrooms
	}
	if in.Latitude != nil {
		l.Latitude = *in.Latitude
	}
	if in.Longitude != nil {
		l.Longitude = *in.Longitude
	}
	if in.AgentID != nil {
		l.AgentID = *in.AgentID
	}
	l.UpdatedAt = time.Now()
	return l, nil
}

func (m *mockStore) Delete(_ context.Context, id uuid.UUID) error {
	if m.err != nil {
		return m.err
	}
	if _, ok := m.listings[id]; !ok {
		return repository.ErrNotFound
	}
	delete(m.listings, id)
	return nil
}

func (m *mockStore) List(_ context.Context, _ model.ListFilter) ([]model.ListingWithDistance, int, error) {
	if m.err != nil {
		return nil, 0, m.err
	}
	out := make([]model.ListingWithDistance, 0, len(m.listings))
	for _, l := range m.listings {
		out = append(out, model.ListingWithDistance{Listing: *l})
	}
	return out, len(out), nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────

func newTestHandler(store handler.ListingStore) *handler.ListingHandler {
	return handler.NewListingHandler(store, newTestLogger())
}

func bodyJSON(t *testing.T, v any) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	return bytes.NewBuffer(b)
}

func decodeBody(t *testing.T, body *bytes.Buffer) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.NewDecoder(body).Decode(&m); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	return m
}

// ─── Create tests ─────────────────────────────────────────────────────────

func TestCreate_Valid(t *testing.T) {
	h := newTestHandler(newMockStore())
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := bodyJSON(t, map[string]any{
		"title":     "3 Bedroom Flat",
		"price":     1500000,
		"type":      "rent",
		"bedrooms":  3,
		"latitude":  6.5244,
		"longitude": 3.3792,
		"agent_id":  uuid.New().String(),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/listings", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rr.Code)
	}

	resp := decodeBody(t, rr.Body)
	if resp["data"] == nil {
		t.Error("expected data in response")
	}
}

func TestCreate_ValidationErrors(t *testing.T) {
	tests := []struct {
		name     string
		body     map[string]any
		wantCode int
		wantErr  string
	}{
		{
			name:     "missing title",
			wantCode: 400,
			body: map[string]any{
				"price": 100, "type": "rent", "bedrooms": 2,
				"latitude": 6.5, "longitude": 3.3,
				"agent_id": uuid.New().String(),
			},
		},
		{
			name:     "invalid type",
			wantCode: 400,
			body: map[string]any{
				"title": "Test", "price": 100, "type": "lease", "bedrooms": 2,
				"latitude": 6.5, "longitude": 3.3,
				"agent_id": uuid.New().String(),
			},
		},
		{
			name:     "negative price",
			wantCode: 400,
			body: map[string]any{
				"title": "Test", "price": -1, "type": "rent", "bedrooms": 2,
				"latitude": 6.5, "longitude": 3.3,
				"agent_id": uuid.New().String(),
			},
		},
		{
			name:     "invalid latitude",
			wantCode: 400,
			body: map[string]any{
				"title": "Test", "price": 100, "type": "rent", "bedrooms": 2,
				"latitude": 91.0, "longitude": 3.3,
				"agent_id": uuid.New().String(),
			},
		},
		{
			name:     "invalid longitude",
			wantCode: 400,
			body: map[string]any{
				"title": "Test", "price": 100, "type": "rent", "bedrooms": 2,
				"latitude": 6.5, "longitude": 200.0,
				"agent_id": uuid.New().String(),
			},
		},
		{
			name:     "invalid agent_id",
			wantCode: 400,
			body: map[string]any{
				"title": "Test", "price": 100, "type": "rent", "bedrooms": 2,
				"latitude": 6.5, "longitude": 3.3,
				"agent_id": "not-a-uuid",
			},
		},
		{
			name:     "negative bedrooms",
			wantCode: 400,
			body: map[string]any{
				"title": "Test", "price": 100, "type": "rent", "bedrooms": -1,
				"latitude": 6.5, "longitude": 3.3,
				"agent_id": uuid.New().String(),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHandler(newMockStore())
			mux := http.NewServeMux()
			h.RegisterRoutes(mux)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/listings", bodyJSON(t, tc.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if rr.Code != tc.wantCode {
				t.Errorf("expected %d, got %d; body: %s", tc.wantCode, rr.Code, rr.Body)
			}

			resp := decodeBody(t, rr.Body)
			errObj, ok := resp["error"].(map[string]any)
			if !ok {
				t.Fatal("expected error object in response")
			}
			if errObj["code"] != "VALIDATION_ERROR" {
				t.Errorf("expected VALIDATION_ERROR, got %v", errObj["code"])
			}
		})
	}
}

func TestCreate_MalformedJSON(t *testing.T) {
	h := newTestHandler(newMockStore())
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listings", bytes.NewBufferString("{bad json"))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestCreate_StoreError(t *testing.T) {
	store := newMockStore()
	store.err = errors.New("db failure")
	h := newTestHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := bodyJSON(t, map[string]any{
		"title": "Test", "price": 100, "type": "rent", "bedrooms": 2,
		"latitude": 6.5, "longitude": 3.3, "agent_id": uuid.New().String(),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/listings", body)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

// ─── GetByID tests ────────────────────────────────────────────────────────

func TestGetByID_Found(t *testing.T) {
	store := newMockStore()
	agentID := uuid.New()
	listing, _ := store.Create(context.Background(), model.CreateListingInput{
		Title: "Test", Price: 100, Type: model.PropertyTypeRent,
		Bedrooms: 2, Latitude: 6.5, Longitude: 3.3, AgentID: agentID,
	})

	h := newTestHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listings/"+listing.ID.String(), nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	h := newTestHandler(newMockStore())
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listings/"+uuid.New().String(), nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestGetByID_InvalidUUID(t *testing.T) {
	h := newTestHandler(newMockStore())
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listings/not-a-uuid", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// ─── Update tests ─────────────────────────────────────────────────────────

func TestUpdate_Valid(t *testing.T) {
	store := newMockStore()
	listing, _ := store.Create(context.Background(), model.CreateListingInput{
		Title: "Old Title", Price: 100, Type: model.PropertyTypeRent,
		Bedrooms: 2, Latitude: 6.5, Longitude: 3.3, AgentID: uuid.New(),
	})

	h := newTestHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	newPrice := 200.0
	body := bodyJSON(t, map[string]any{"price": newPrice})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/listings/"+listing.ID.String(), body)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", rr.Code, rr.Body)
	}

	resp := decodeBody(t, rr.Body)
	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatal("expected data object")
	}
	if data["price"] != newPrice {
		t.Errorf("expected price %v, got %v", newPrice, data["price"])
	}
}

func TestUpdate_NotFound(t *testing.T) {
	h := newTestHandler(newMockStore())
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := bodyJSON(t, map[string]any{"price": 200.0})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/listings/"+uuid.New().String(), body)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestUpdate_NoFields(t *testing.T) {
	store := newMockStore()
	listing, _ := store.Create(context.Background(), model.CreateListingInput{
		Title: "Test", Price: 100, Type: model.PropertyTypeRent,
		Bedrooms: 2, Latitude: 6.5, Longitude: 3.3, AgentID: uuid.New(),
	})

	// Override Update to return ErrNoFieldsToUpdate
	store.err = repository.ErrNoFieldsToUpdate

	h := newTestHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := bodyJSON(t, map[string]any{}) // empty body
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/listings/"+listing.ID.String(), body)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestUpdate_ValidationErrors(t *testing.T) {
	store := newMockStore()
	listing, _ := store.Create(context.Background(), model.CreateListingInput{
		Title: "Test", Price: 100, Type: model.PropertyTypeRent,
		Bedrooms: 2, Latitude: 6.5, Longitude: 3.3, AgentID: uuid.New(),
	})

	h := newTestHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	invalidType := "not-valid"
	body := bodyJSON(t, map[string]any{"type": invalidType})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/listings/"+listing.ID.String(), body)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// ─── Delete tests ─────────────────────────────────────────────────────────

func TestDelete_Success(t *testing.T) {
	store := newMockStore()
	listing, _ := store.Create(context.Background(), model.CreateListingInput{
		Title: "Test", Price: 100, Type: model.PropertyTypeRent,
		Bedrooms: 2, Latitude: 6.5, Longitude: 3.3, AgentID: uuid.New(),
	})

	h := newTestHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/listings/"+listing.ID.String(), nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestDelete_NotFound(t *testing.T) {
	h := newTestHandler(newMockStore())
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/listings/"+uuid.New().String(), nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

// ─── List tests ───────────────────────────────────────────────────────────

func TestList_NoFilters(t *testing.T) {
	store := newMockStore()
	for range 3 {
		store.Create(context.Background(), model.CreateListingInput{ //nolint:errcheck
			Title: "Test", Price: 100, Type: model.PropertyTypeRent,
			Bedrooms: 2, Latitude: 6.5, Longitude: 3.3, AgentID: uuid.New(),
		})
	}

	h := newTestHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listings", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	resp := decodeBody(t, rr.Body)
	if resp["data"] == nil {
		t.Error("expected data in response")
	}
	if resp["pagination"] == nil {
		t.Error("expected pagination in response")
	}
}

func TestList_InvalidQueryParams(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{name: "invalid type", query: "?type=unknown"},
		{name: "invalid min_price", query: "?min_price=abc"},
		{name: "invalid bedrooms", query: "?bedrooms=-5"},
		{name: "invalid page", query: "?page=0"},
		{name: "invalid limit", query: "?limit=200"},
		{name: "partial geo params", query: "?lat=6.5&lng=3.3"},
		{name: "invalid lat", query: "?lat=200&lng=3.3&radius=10"},
		{name: "invalid radius", query: "?lat=6.5&lng=3.3&radius=-1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHandler(newMockStore())
			mux := http.NewServeMux()
			h.RegisterRoutes(mux)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/listings"+tc.query, nil)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d; body: %s", rr.Code, rr.Body)
			}
		})
	}
}

func TestList_FilterByType(t *testing.T) {
	h := newTestHandler(newMockStore())
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listings?type=rent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestList_GeoParams_AllPresent(t *testing.T) {
	h := newTestHandler(newMockStore())
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listings?lat=6.5244&lng=3.3792&radius=10", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestList_CombinedFilters(t *testing.T) {
	h := newTestHandler(newMockStore())
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	url := "/api/v1/listings?type=rent&bedrooms=2&min_price=200000&max_price=1000000&lat=6.5244&lng=3.3792&radius=10&page=1&limit=20"
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}
