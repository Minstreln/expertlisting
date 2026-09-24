// Package model defines the core domain types for the property listings API.
package model

import (
	"time"

	"github.com/google/uuid"
)

// PropertyType represents the category of a listing.
type PropertyType string

const (
	PropertyTypeRent     PropertyType = "rent"
	PropertyTypeSale     PropertyType = "sale"
	PropertyTypeShortlet PropertyType = "shortlet"
)

// IsValid returns true if the PropertyType is one of the accepted values.
func (t PropertyType) IsValid() bool {
	switch t {
	case PropertyTypeRent, PropertyTypeSale, PropertyTypeShortlet:
		return true
	}
	return false
}

// Listing is the canonical representation of a property listing as stored in
// the database. Returned for single-item and collection responses.
type Listing struct {
	ID        uuid.UUID    `json:"id"`
	Title     string       `json:"title"`
	Price     float64      `json:"price"`
	Type      PropertyType `json:"type"`
	Bedrooms  int          `json:"bedrooms"`
	Latitude  float64      `json:"latitude"`
	Longitude float64      `json:"longitude"`
	AgentID   uuid.UUID    `json:"agent_id"`
	ImageURL  *string      `json:"image_url,omitempty"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// ListingWithDistance extends Listing with a computed distance from a search
// origin. DistanceKM is nil when no geospatial filter was applied.
type ListingWithDistance struct {
	Listing
	DistanceKM *float64 `json:"distance_km,omitempty"`
}

// CreateListingInput holds validated data for creating a new listing.
type CreateListingInput struct {
	Title     string
	Price     float64
	Type      PropertyType
	Bedrooms  int
	Latitude  float64
	Longitude float64
	AgentID   uuid.UUID
	ImageURL  *string
}

// UpdateListingInput holds partial update fields. A nil pointer means the
// field was not provided in the request and should not be changed.
type UpdateListingInput struct {
	Title     *string
	Price     *float64
	Type      *PropertyType
	Bedrooms  *int
	Latitude  *float64
	Longitude *float64
	AgentID   *uuid.UUID
	ImageURL  *string
}

// ListFilter describes search and pagination options for listing queries.
type ListFilter struct {
	Type     string
	MinPrice *float64
	MaxPrice *float64
	Bedrooms *int
	// Lat, Lng and Radius are only used when all three are non-nil.
	Lat    *float64
	Lng    *float64
	Radius *float64
	Page   int
	Limit  int
}
