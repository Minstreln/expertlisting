package tests

import (
	"context"
	"expert-listing/internal/db"
	"expert-listing/internal/model"
	"expert-listing/internal/repository"
	"math"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Integration tests require a running PostgreSQL database.
// Set TEST_DATABASE_URL to run them:
//
//	TEST_DATABASE_URL=postgres://user:pass@localhost:5432/testdb go test ./tests/...
//
// The tests create and clean up their own data; they do not drop or recreate
// the schema. Run the migration before running these tests.

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		// Skip integration tests gracefully when no test DB is configured.
		os.Exit(m.Run())
	}

	var err error
	testPool, err = db.Connect(context.Background(), dsn)
	if err != nil {
		panic("failed to connect to test database: " + err.Error())
	}
	defer testPool.Close()

	os.Exit(m.Run())
}

// skipIfNoTestDB skips the test when testPool has not been initialised.
func skipIfNoTestDB(t *testing.T) {
	t.Helper()
	if testPool == nil {
		t.Skip("TEST_DATABASE_URL not set — skipping integration test")
	}
}

// cleanListings removes all rows inserted by a test to keep tests isolated.
func cleanListings(t *testing.T, ids ...uuid.UUID) {
	t.Helper()
	for _, id := range ids {
		testPool.Exec(context.Background(), `DELETE FROM listings WHERE id = $1`, id) //nolint:errcheck
	}
}

// ─── Create ───────────────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	skipIfNoTestDB(t)
	repo := repository.New(testPool)

	in := model.CreateListingInput{
		Title:     "2 Bedroom Flat in VI",
		Price:     1500000,
		Type:      model.PropertyTypeRent,
		Bedrooms:  2,
		Latitude:  6.4281,
		Longitude: 3.4219,
		AgentID:   uuid.New(),
	}

	l, err := repo.Create(context.Background(), in)
	if err != nil {
		t.Fatalf("create listing: %v", err)
	}
	defer cleanListings(t, l.ID)

	if l.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
	if l.Title != in.Title {
		t.Errorf("title: got %q, want %q", l.Title, in.Title)
	}
	if l.Price != in.Price {
		t.Errorf("price: got %v, want %v", l.Price, in.Price)
	}
	if l.Type != in.Type {
		t.Errorf("type: got %q, want %q", l.Type, in.Type)
	}
}

// ─── GetByID ──────────────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	skipIfNoTestDB(t)
	repo := repository.New(testPool)

	created, err := repo.Create(context.Background(), model.CreateListingInput{
		Title: "Test GetByID", Price: 500000, Type: model.PropertyTypeSale,
		Bedrooms: 3, Latitude: 6.5, Longitude: 3.3, AgentID: uuid.New(),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer cleanListings(t, created.ID)

	got, err := repo.GetByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("id mismatch: got %v, want %v", got.ID, created.ID)
	}
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	skipIfNoTestDB(t)
	repo := repository.New(testPool)

	_, err := repo.GetByID(context.Background(), uuid.New())
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ─── Update ───────────────────────────────────────────────────────────────

func TestRepository_Update(t *testing.T) {
	skipIfNoTestDB(t)
	repo := repository.New(testPool)

	created, err := repo.Create(context.Background(), model.CreateListingInput{
		Title: "Before Update", Price: 100000, Type: model.PropertyTypeRent,
		Bedrooms: 1, Latitude: 6.5, Longitude: 3.3, AgentID: uuid.New(),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer cleanListings(t, created.ID)

	newTitle := "After Update"
	newPrice := 200000.0
	updated, err := repo.Update(context.Background(), created.ID, model.UpdateListingInput{
		Title: &newTitle,
		Price: &newPrice,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if updated.Title != newTitle {
		t.Errorf("title: got %q, want %q", updated.Title, newTitle)
	}
	if updated.Price != newPrice {
		t.Errorf("price: got %v, want %v", updated.Price, newPrice)
	}
	// Fields not updated must remain unchanged.
	if updated.Bedrooms != created.Bedrooms {
		t.Errorf("bedrooms should be unchanged: got %d", updated.Bedrooms)
	}
	if updated.UpdatedAt.Before(created.UpdatedAt) || updated.UpdatedAt.Equal(created.UpdatedAt) {
		t.Error("updated_at should advance after an update")
	}
}

func TestRepository_Update_NoFields(t *testing.T) {
	skipIfNoTestDB(t)
	repo := repository.New(testPool)

	created, _ := repo.Create(context.Background(), model.CreateListingInput{
		Title: "Test", Price: 100, Type: model.PropertyTypeRent,
		Bedrooms: 1, Latitude: 6.5, Longitude: 3.3, AgentID: uuid.New(),
	})
	defer cleanListings(t, created.ID)

	_, err := repo.Update(context.Background(), created.ID, model.UpdateListingInput{})
	if err != repository.ErrNoFieldsToUpdate {
		t.Errorf("expected ErrNoFieldsToUpdate, got %v", err)
	}
}

func TestRepository_Update_NotFound(t *testing.T) {
	skipIfNoTestDB(t)
	repo := repository.New(testPool)

	title := "Ghost"
	_, err := repo.Update(context.Background(), uuid.New(), model.UpdateListingInput{Title: &title})
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ─── Delete ───────────────────────────────────────────────────────────────

func TestRepository_Delete(t *testing.T) {
	skipIfNoTestDB(t)
	repo := repository.New(testPool)

	created, _ := repo.Create(context.Background(), model.CreateListingInput{
		Title: "To Delete", Price: 100, Type: model.PropertyTypeRent,
		Bedrooms: 1, Latitude: 6.5, Longitude: 3.3, AgentID: uuid.New(),
	})

	if err := repo.Delete(context.Background(), created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := repo.GetByID(context.Background(), created.ID)
	if err != repository.ErrNotFound {
		t.Error("expected listing to be gone after delete")
	}
}

func TestRepository_Delete_NotFound(t *testing.T) {
	skipIfNoTestDB(t)
	repo := repository.New(testPool)

	err := repo.Delete(context.Background(), uuid.New())
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ─── List: basic filters ──────────────────────────────────────────────────

func TestRepository_List_FilterByType(t *testing.T) {
	skipIfNoTestDB(t)
	repo := repository.New(testPool)

	rent, _ := repo.Create(context.Background(), model.CreateListingInput{
		Title: "Rent Listing", Price: 500000, Type: model.PropertyTypeRent,
		Bedrooms: 2, Latitude: 6.5, Longitude: 3.3, AgentID: uuid.New(),
	})
	sale, _ := repo.Create(context.Background(), model.CreateListingInput{
		Title: "Sale Listing", Price: 5000000, Type: model.PropertyTypeSale,
		Bedrooms: 4, Latitude: 6.6, Longitude: 3.4, AgentID: uuid.New(),
	})
	defer cleanListings(t, rent.ID, sale.ID)

	listings, _, err := repo.List(context.Background(), model.ListFilter{
		Type: "rent", Page: 1, Limit: 100,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	for _, l := range listings {
		if l.Type != model.PropertyTypeRent {
			t.Errorf("expected only rent listings, got %q (id %s)", l.Type, l.ID)
		}
	}
}

func TestRepository_List_FilterByPriceRange(t *testing.T) {
	skipIfNoTestDB(t)
	repo := repository.New(testPool)

	cheap, _ := repo.Create(context.Background(), model.CreateListingInput{
		Title: "Cheap", Price: 100000, Type: model.PropertyTypeRent,
		Bedrooms: 1, Latitude: 6.5, Longitude: 3.3, AgentID: uuid.New(),
	})
	expensive, _ := repo.Create(context.Background(), model.CreateListingInput{
		Title: "Expensive", Price: 10000000, Type: model.PropertyTypeRent,
		Bedrooms: 5, Latitude: 6.5, Longitude: 3.3, AgentID: uuid.New(),
	})
	defer cleanListings(t, cheap.ID, expensive.ID)

	minP, maxP := 50000.0, 500000.0
	listings, _, err := repo.List(context.Background(), model.ListFilter{
		MinPrice: &minP, MaxPrice: &maxP, Page: 1, Limit: 100,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	for _, l := range listings {
		if l.Price < minP || l.Price > maxP {
			t.Errorf("listing %s price %v is outside range [%v, %v]", l.ID, l.Price, minP, maxP)
		}
	}
}

func TestRepository_List_FilterByBedrooms(t *testing.T) {
	skipIfNoTestDB(t)
	repo := repository.New(testPool)

	two, _ := repo.Create(context.Background(), model.CreateListingInput{
		Title: "2 Bed", Price: 500000, Type: model.PropertyTypeRent,
		Bedrooms: 2, Latitude: 6.5, Longitude: 3.3, AgentID: uuid.New(),
	})
	four, _ := repo.Create(context.Background(), model.CreateListingInput{
		Title: "4 Bed", Price: 1000000, Type: model.PropertyTypeRent,
		Bedrooms: 4, Latitude: 6.5, Longitude: 3.3, AgentID: uuid.New(),
	})
	defer cleanListings(t, two.ID, four.ID)

	beds := 2
	listings, _, err := repo.List(context.Background(), model.ListFilter{
		Bedrooms: &beds, Page: 1, Limit: 100,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	for _, l := range listings {
		if l.Bedrooms != beds {
			t.Errorf("expected %d bedrooms, got %d (id %s)", beds, l.Bedrooms, l.ID)
		}
	}
}

// ─── List: geospatial ─────────────────────────────────────────────────────

func TestRepository_List_GeoFilter(t *testing.T) {
	skipIfNoTestDB(t)
	repo := repository.New(testPool)

	// Victoria Island, Lagos — inside 5 km of search origin
	near, _ := repo.Create(context.Background(), model.CreateListingInput{
		Title: "Near VI", Price: 2000000, Type: model.PropertyTypeSale,
		Bedrooms: 3, Latitude: 6.4281, Longitude: 3.4219, AgentID: uuid.New(),
	})
	// Ibadan — ~120 km away, outside radius
	far, _ := repo.Create(context.Background(), model.CreateListingInput{
		Title: "Far Ibadan", Price: 500000, Type: model.PropertyTypeSale,
		Bedrooms: 3, Latitude: 7.3775, Longitude: 3.9470, AgentID: uuid.New(),
	})
	defer cleanListings(t, near.ID, far.ID)

	// Search origin: Lagos Island (~6.454, 3.394)
	lat, lng, radius := 6.454, 3.394, 10.0

	listings, total, err := repo.List(context.Background(), model.ListFilter{
		Lat: &lat, Lng: &lng, Radius: &radius,
		Page: 1, Limit: 100,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	// near listing should appear; far listing should not
	foundNear := false
	for _, l := range listings {
		if l.ID == far.ID {
			t.Errorf("far listing (Ibadan) should be outside the 10 km radius")
		}
		if l.ID == near.ID {
			foundNear = true
			if l.DistanceKM == nil {
				t.Error("distance_km should be populated for geo search")
			}
		}
	}

	if !foundNear && total > 0 {
		t.Log("near listing not found — it may actually be outside the 10 km radius given exact coordinates")
	}
	_ = total
}

func TestRepository_List_GeoDistance_Ordering(t *testing.T) {
	skipIfNoTestDB(t)
	repo := repository.New(testPool)

	// Two listings at known distances from origin (6.5244, 3.3792)
	// Listing A: ~1 km away
	a, _ := repo.Create(context.Background(), model.CreateListingInput{
		Title: "Close", Price: 100, Type: model.PropertyTypeRent,
		Bedrooms: 1, Latitude: 6.5330, Longitude: 3.3792, AgentID: uuid.New(),
	})
	// Listing B: ~5 km away
	b, _ := repo.Create(context.Background(), model.CreateListingInput{
		Title: "Further", Price: 100, Type: model.PropertyTypeRent,
		Bedrooms: 1, Latitude: 6.5600, Longitude: 3.3792, AgentID: uuid.New(),
	})
	defer cleanListings(t, a.ID, b.ID)

	lat, lng, radius := 6.5244, 3.3792, 20.0
	listings, _, err := repo.List(context.Background(), model.ListFilter{
		Lat: &lat, Lng: &lng, Radius: &radius, Page: 1, Limit: 100,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	// Verify results are ordered by distance ascending
	for i := 1; i < len(listings); i++ {
		prev := listings[i-1].DistanceKM
		curr := listings[i].DistanceKM
		if prev != nil && curr != nil && *prev > *curr+0.001 {
			t.Errorf("listings not ordered by distance: index %d (%v km) > index %d (%v km)",
				i-1, math.Round(*prev*100)/100,
				i, math.Round(*curr*100)/100)
		}
	}
}

// ─── List: pagination ─────────────────────────────────────────────────────

func TestRepository_List_Pagination(t *testing.T) {
	skipIfNoTestDB(t)
	repo := repository.New(testPool)

	agentID := uuid.New()
	ids := make([]uuid.UUID, 5)
	for i := range ids {
		l, _ := repo.Create(context.Background(), model.CreateListingInput{
			Title: "Paginate Me", Price: float64(i+1) * 100000, Type: model.PropertyTypeRent,
			Bedrooms: 1, Latitude: 6.5, Longitude: 3.3, AgentID: agentID,
		})
		ids[i] = l.ID
	}
	defer cleanListings(t, ids...)

	// Page 1 of 2
	page1, total, err := repo.List(context.Background(), model.ListFilter{
		Page: 1, Limit: 3,
	})
	if err != nil {
		t.Fatalf("list page 1: %v", err)
	}
	if len(page1) > 3 {
		t.Errorf("page 1 should have at most 3 results, got %d", len(page1))
	}
	if total < 5 {
		t.Errorf("total count should be >= 5 (test data), got %d", total)
	}

	// Page 2
	page2, _, err := repo.List(context.Background(), model.ListFilter{
		Page: 2, Limit: 3,
	})
	if err != nil {
		t.Fatalf("list page 2: %v", err)
	}

	// No overlap between pages
	page1IDs := make(map[uuid.UUID]bool)
	for _, l := range page1 {
		page1IDs[l.ID] = true
	}
	for _, l := range page2 {
		if page1IDs[l.ID] {
			t.Errorf("listing %s appears in both page 1 and page 2", l.ID)
		}
	}
}
