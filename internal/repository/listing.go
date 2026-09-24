// Package repository provides PostgreSQL-backed data access for listings.
package repository

import (
	"context"
	"errors"
	"expert-listing/internal/model"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a listing does not exist.
var ErrNotFound = errors.New("listing not found")

// ErrNoFieldsToUpdate is returned when a PATCH request contains no changes.
var ErrNoFieldsToUpdate = errors.New("no fields to update")

// ListingRepository executes SQL for property listings.
type ListingRepository struct {
	db *pgxpool.Pool
}

// New returns a ListingRepository backed by the given pool.
func New(db *pgxpool.Pool) *ListingRepository {
	return &ListingRepository{db: db}
}

// Create inserts a new listing and returns the persisted record.
func (r *ListingRepository) Create(ctx context.Context, in model.CreateListingInput) (*model.Listing, error) {
	var l model.Listing
	err := r.db.QueryRow(ctx, `
		INSERT INTO listings (title, price, type, bedrooms, latitude, longitude, agent_id, image_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, title, price, type, bedrooms, latitude, longitude, agent_id, image_url, created_at, updated_at
	`, in.Title, in.Price, in.Type, in.Bedrooms, in.Latitude, in.Longitude, in.AgentID, in.ImageURL).
		Scan(&l.ID, &l.Title, &l.Price, &l.Type, &l.Bedrooms,
			&l.Latitude, &l.Longitude, &l.AgentID, &l.ImageURL, &l.CreatedAt, &l.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert listing: %w", err)
	}
	return &l, nil
}

// GetByID fetches a single listing by its primary key.
// Returns ErrNotFound when no row exists.
func (r *ListingRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Listing, error) {
	var l model.Listing
	err := r.db.QueryRow(ctx, `
		SELECT id, title, price, type, bedrooms, latitude, longitude, agent_id, image_url, created_at, updated_at
		FROM listings
		WHERE id = $1
	`, id).Scan(&l.ID, &l.Title, &l.Price, &l.Type, &l.Bedrooms,
		&l.Latitude, &l.Longitude, &l.AgentID, &l.ImageURL, &l.CreatedAt, &l.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get listing: %w", err)
	}
	return &l, nil
}

// Update applies a partial update to a listing and returns the updated record.
// Returns ErrNotFound when the listing does not exist.
// Returns ErrNoFieldsToUpdate when no fields were supplied.
func (r *ListingRepository) Update(ctx context.Context, id uuid.UUID, in model.UpdateListingInput) (*model.Listing, error) {
	sets := []string{}
	args := []any{}
	idx := 1

	if in.Title != nil {
		sets = append(sets, fmt.Sprintf("title = $%d", idx))
		args = append(args, *in.Title)
		idx++
	}
	if in.Price != nil {
		sets = append(sets, fmt.Sprintf("price = $%d", idx))
		args = append(args, *in.Price)
		idx++
	}
	if in.Type != nil {
		sets = append(sets, fmt.Sprintf("type = $%d", idx))
		args = append(args, *in.Type)
		idx++
	}
	if in.Bedrooms != nil {
		sets = append(sets, fmt.Sprintf("bedrooms = $%d", idx))
		args = append(args, *in.Bedrooms)
		idx++
	}
	if in.Latitude != nil {
		sets = append(sets, fmt.Sprintf("latitude = $%d", idx))
		args = append(args, *in.Latitude)
		idx++
	}
	if in.Longitude != nil {
		sets = append(sets, fmt.Sprintf("longitude = $%d", idx))
		args = append(args, *in.Longitude)
		idx++
	}
	if in.AgentID != nil {
		sets = append(sets, fmt.Sprintf("agent_id = $%d", idx))
		args = append(args, *in.AgentID)
		idx++
	}
	if in.ImageURL != nil {
		sets = append(sets, fmt.Sprintf("image_url = $%d", idx))
		args = append(args, *in.ImageURL)
		idx++
	}

	if len(sets) == 0 {
		return nil, ErrNoFieldsToUpdate
	}

	// updated_at is managed by a DB trigger, but set it explicitly here as well
	// so the RETURNING clause reflects the current timestamp immediately.
	sets = append(sets, "updated_at = NOW()")
	args = append(args, id)

	query := fmt.Sprintf(`
		UPDATE listings
		SET %s
		WHERE id = $%d
		RETURNING id, title, price, type, bedrooms, latitude, longitude, agent_id, image_url, created_at, updated_at
	`, strings.Join(sets, ", "), idx)

	var l model.Listing
	err := r.db.QueryRow(ctx, query, args...).
		Scan(&l.ID, &l.Title, &l.Price, &l.Type, &l.Bedrooms,
			&l.Latitude, &l.Longitude, &l.AgentID, &l.ImageURL, &l.CreatedAt, &l.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update listing: %w", err)
	}
	return &l, nil
}

// Delete removes a listing. Returns ErrNotFound when no row was affected.
func (r *ListingRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.Exec(ctx, `DELETE FROM listings WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete listing: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// List returns paginated listings filtered by the supplied criteria.
// The total count reflects matching rows before pagination is applied.
//
// When Lat, Lng and Radius are all provided in the filter, the query computes
// the Haversine distance for each candidate and discards rows outside the
// radius. Filtering happens entirely in PostgreSQL so only matching rows are
// transferred to the application. Matched rows are ordered by distance
// ascending; otherwise results are ordered by created_at descending.
func (r *ListingRepository) List(ctx context.Context, f model.ListFilter) ([]model.ListingWithDistance, int, error) {
	conditions := []string{}
	whereArgs := []any{}
	idx := 1

	if f.Type != "" {
		conditions = append(conditions, fmt.Sprintf("type = $%d::property_type", idx))
		whereArgs = append(whereArgs, f.Type)
		idx++
	}
	if f.MinPrice != nil {
		conditions = append(conditions, fmt.Sprintf("price >= $%d", idx))
		whereArgs = append(whereArgs, *f.MinPrice)
		idx++
	}
	if f.MaxPrice != nil {
		conditions = append(conditions, fmt.Sprintf("price <= $%d", idx))
		whereArgs = append(whereArgs, *f.MaxPrice)
		idx++
	}
	if f.Bedrooms != nil {
		conditions = append(conditions, fmt.Sprintf("bedrooms = $%d", idx))
		whereArgs = append(whereArgs, *f.Bedrooms)
		idx++
	}

	hasGeo := f.Lat != nil && f.Lng != nil && f.Radius != nil

	// The Haversine formula computes the great-circle distance between two
	// points on a sphere. LEAST(1.0, ...) guards against floating-point values
	// marginally above 1 that would cause acos to return NaN.
	var haversineExpr string
	if hasGeo {
		latIdx, lngIdx, radiusIdx := idx, idx+1, idx+2
		whereArgs = append(whereArgs, *f.Lat, *f.Lng, *f.Radius)
		idx += 3

		haversineExpr = fmt.Sprintf(`(
			6371.0 * acos(LEAST(1.0,
				cos(radians($%d)) * cos(radians(latitude))
				* cos(radians(longitude) - radians($%d))
				+ sin(radians($%d)) * sin(radians(latitude))
			))
		)`, latIdx, lngIdx, latIdx)

		conditions = append(conditions, fmt.Sprintf("%s <= $%d", haversineExpr, radiusIdx))
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	if err := r.db.QueryRow(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM listings %s", whereClause),
		whereArgs...,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count listings: %w", err)
	}

	distanceSel := "NULL::DOUBLE PRECISION AS distance_km"
	orderBy := "ORDER BY created_at DESC"
	if hasGeo {
		distanceSel = haversineExpr + " AS distance_km"
		orderBy = "ORDER BY distance_km ASC, created_at DESC"
	}

	limitIdx := idx
	offsetIdx := idx + 1
	selectArgs := append(whereArgs, f.Limit, (f.Page-1)*f.Limit)

	query := fmt.Sprintf(`
		SELECT id, title, price, type, bedrooms, latitude, longitude, agent_id,
		       image_url, created_at, updated_at, %s
		FROM listings
		%s
		%s
		LIMIT $%d OFFSET $%d
	`, distanceSel, whereClause, orderBy, limitIdx, offsetIdx)

	rows, err := r.db.Query(ctx, query, selectArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query listings: %w", err)
	}
	defer rows.Close()

	listings := make([]model.ListingWithDistance, 0, f.Limit)
	for rows.Next() {
		var lwd model.ListingWithDistance
		if err := rows.Scan(
			&lwd.ID, &lwd.Title, &lwd.Price, &lwd.Type, &lwd.Bedrooms,
			&lwd.Latitude, &lwd.Longitude, &lwd.AgentID,
			&lwd.ImageURL, &lwd.CreatedAt, &lwd.UpdatedAt, &lwd.DistanceKM,
		); err != nil {
			return nil, 0, fmt.Errorf("scan listing row: %w", err)
		}
		listings = append(listings, lwd)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate listing rows: %w", err)
	}

	return listings, total, nil
}
