// Package handler implements the HTTP handlers for the listings API.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"expert-listing/internal/model"
	"expert-listing/internal/repository"
	"expert-listing/internal/respond"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// ListingStore abstracts the data layer so handlers can be unit-tested
// without a real database.
type ListingStore interface {
	Create(ctx context.Context, in model.CreateListingInput) (*model.Listing, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Listing, error)
	Update(ctx context.Context, id uuid.UUID, in model.UpdateListingInput) (*model.Listing, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, f model.ListFilter) ([]model.ListingWithDistance, int, error)
}

// ListingHandler handles HTTP requests for property listings.
type ListingHandler struct {
	store  ListingStore
	logger *logrus.Logger
}

// NewListingHandler returns a handler wired to the given store and logger.
func NewListingHandler(store ListingStore, logger *logrus.Logger) *ListingHandler {
	return &ListingHandler{store: store, logger: logger}
}

// RegisterRoutes registers all listing routes on mux.
func (h *ListingHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/listings", h.Create)
	mux.HandleFunc("GET /api/v1/listings", h.List)
	mux.HandleFunc("GET /api/v1/listings/{id}", h.GetByID)
	mux.HandleFunc("PATCH /api/v1/listings/{id}", h.Update)
	mux.HandleFunc("DELETE /api/v1/listings/{id}", h.Delete)
}

type createRequest struct {
	Title     string  `json:"title"`
	Price     float64 `json:"price"`
	Type      string  `json:"type"`
	Bedrooms  int     `json:"bedrooms"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	AgentID   string  `json:"agent_id"`
	ImageURL  *string `json:"image_url"`
}

type updateRequest struct {
	Title     *string  `json:"title"`
	Price     *float64 `json:"price"`
	Type      *string  `json:"type"`
	Bedrooms  *int     `json:"bedrooms"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
	AgentID   *string  `json:"agent_id"`
	ImageURL  *string  `json:"image_url"`
}

type paginationMeta struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// Create handles POST /api/v1/listings.
func (h *ListingHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := decodeJSON(r, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "INVALID_JSON", err.Error(), nil)
		return
	}

	if errs := validateCreate(req); len(errs) > 0 {
		respond.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "validation failed", errs)
		return
	}

	agentID, _ := uuid.Parse(req.AgentID)

	listing, err := h.store.Create(r.Context(), model.CreateListingInput{
		Title:     strings.TrimSpace(req.Title),
		Price:     req.Price,
		Type:      model.PropertyType(req.Type),
		Bedrooms:  req.Bedrooms,
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
		AgentID:   agentID,
		ImageURL:  req.ImageURL,
	})
	if err != nil {
		h.logger.Errorf("create listing: %v", err)
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error", nil)
		return
	}

	respond.JSON(w, http.StatusCreated, map[string]any{"data": listing})
}

// GetByID handles GET /api/v1/listings/{id}.
func (h *ListingHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r.PathValue("id"))
	if !ok {
		return
	}

	listing, err := h.store.GetByID(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		respond.Error(w, http.StatusNotFound, "NOT_FOUND", "listing not found", nil)
		return
	}
	if err != nil {
		h.logger.Errorf("get listing %s: %v", id, err)
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error", nil)
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{"data": listing})
}

// Update handles PATCH /api/v1/listings/{id}.
func (h *ListingHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r.PathValue("id"))
	if !ok {
		return
	}

	var req updateRequest
	if err := decodeJSON(r, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "INVALID_JSON", err.Error(), nil)
		return
	}

	if errs := validateUpdate(req); len(errs) > 0 {
		respond.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "validation failed", errs)
		return
	}

	in := model.UpdateListingInput{
		Price:     req.Price,
		Bedrooms:  req.Bedrooms,
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
		ImageURL:  req.ImageURL,
	}
	if req.Title != nil {
		trimmed := strings.TrimSpace(*req.Title)
		in.Title = &trimmed
	}
	if req.Type != nil {
		pt := model.PropertyType(*req.Type)
		in.Type = &pt
	}
	if req.AgentID != nil {
		uid, _ := uuid.Parse(*req.AgentID)
		in.AgentID = &uid
	}

	listing, err := h.store.Update(r.Context(), id, in)
	if errors.Is(err, repository.ErrNotFound) {
		respond.Error(w, http.StatusNotFound, "NOT_FOUND", "listing not found", nil)
		return
	}
	if errors.Is(err, repository.ErrNoFieldsToUpdate) {
		respond.Error(w, http.StatusBadRequest, "BAD_REQUEST", "no fields to update", nil)
		return
	}
	if err != nil {
		h.logger.Errorf("update listing %s: %v", id, err)
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error", nil)
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{"data": listing})
}

// Delete handles DELETE /api/v1/listings/{id}.
func (h *ListingHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r.PathValue("id"))
	if !ok {
		return
	}

	err := h.store.Delete(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		respond.Error(w, http.StatusNotFound, "NOT_FOUND", "listing not found", nil)
		return
	}
	if err != nil {
		h.logger.Errorf("delete listing %s: %v", id, err)
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error", nil)
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{"message": "listing deleted"})
}

// List handles GET /api/v1/listings with optional search and filter parameters.
//
// Supported query parameters:
//
//	type        – property type (rent | sale | shortlet)
//	min_price   – minimum price (inclusive)
//	max_price   – maximum price (inclusive)
//	bedrooms    – exact bedroom count
//	lat, lng    – geographic centre (both required for geo search)
//	radius      – search radius in kilometres (required when lat/lng are given)
//	page        – page number (default: 1)
//	limit       – results per page (default: 20, max: 100)
func (h *ListingHandler) List(w http.ResponseWriter, r *http.Request) {
	filter, errs := parseListFilter(r)
	if len(errs) > 0 {
		respond.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid query parameters", errs)
		return
	}

	listings, total, err := h.store.List(r.Context(), filter)
	if err != nil {
		h.logger.Errorf("list listings: %v", err)
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error", nil)
		return
	}

	totalPages := 0
	if total > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(filter.Limit)))
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"data": listings,
		"pagination": paginationMeta{
			Page:       filter.Page,
			Limit:      filter.Limit,
			Total:      total,
			TotalPages: totalPages,
		},
	})
}

func validateCreate(req createRequest) map[string]string {
	errs := map[string]string{}

	if strings.TrimSpace(req.Title) == "" {
		errs["title"] = "required"
	} else if len(strings.TrimSpace(req.Title)) > 255 {
		errs["title"] = "must not exceed 255 characters"
	}

	if req.Price < 0 {
		errs["price"] = "must be >= 0"
	}

	if !model.PropertyType(req.Type).IsValid() {
		errs["type"] = "must be one of: rent, sale, shortlet"
	}

	if req.Bedrooms < 0 {
		errs["bedrooms"] = "must be >= 0"
	}

	if req.Latitude < -90 || req.Latitude > 90 {
		errs["latitude"] = "must be between -90 and 90"
	}

	if req.Longitude < -180 || req.Longitude > 180 {
		errs["longitude"] = "must be between -180 and 180"
	}

	if _, err := uuid.Parse(req.AgentID); err != nil {
		errs["agent_id"] = "must be a valid UUID"
	}

	return errs
}

func validateUpdate(req updateRequest) map[string]string {
	errs := map[string]string{}

	if req.Title != nil {
		if strings.TrimSpace(*req.Title) == "" {
			errs["title"] = "must not be empty"
		} else if len(strings.TrimSpace(*req.Title)) > 255 {
			errs["title"] = "must not exceed 255 characters"
		}
	}
	if req.Price != nil && *req.Price < 0 {
		errs["price"] = "must be >= 0"
	}
	if req.Type != nil && !model.PropertyType(*req.Type).IsValid() {
		errs["type"] = "must be one of: rent, sale, shortlet"
	}
	if req.Bedrooms != nil && *req.Bedrooms < 0 {
		errs["bedrooms"] = "must be >= 0"
	}
	if req.Latitude != nil && (*req.Latitude < -90 || *req.Latitude > 90) {
		errs["latitude"] = "must be between -90 and 90"
	}
	if req.Longitude != nil && (*req.Longitude < -180 || *req.Longitude > 180) {
		errs["longitude"] = "must be between -180 and 180"
	}
	if req.AgentID != nil {
		if _, err := uuid.Parse(*req.AgentID); err != nil {
			errs["agent_id"] = "must be a valid UUID"
		}
	}
	return errs
}

func parseListFilter(r *http.Request) (model.ListFilter, map[string]string) {
	q := r.URL.Query()
	errs := map[string]string{}

	f := model.ListFilter{
		Page:  1,
		Limit: 20,
	}

	if v := q.Get("type"); v != "" {
		if !model.PropertyType(v).IsValid() {
			errs["type"] = "must be one of: rent, sale, shortlet"
		} else {
			f.Type = v
		}
	}

	if v := q.Get("min_price"); v != "" {
		n, err := strconv.ParseFloat(v, 64)
		if err != nil || n < 0 {
			errs["min_price"] = "must be a non-negative number"
		} else {
			f.MinPrice = &n
		}
	}

	if v := q.Get("max_price"); v != "" {
		n, err := strconv.ParseFloat(v, 64)
		if err != nil || n < 0 {
			errs["max_price"] = "must be a non-negative number"
		} else {
			f.MaxPrice = &n
		}
	}

	if f.MinPrice != nil && f.MaxPrice != nil && *f.MinPrice > *f.MaxPrice {
		errs["max_price"] = "must be >= min_price"
	}

	if v := q.Get("bedrooms"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			errs["bedrooms"] = "must be a non-negative integer"
		} else {
			f.Bedrooms = &n
		}
	}

	latStr := q.Get("lat")
	lngStr := q.Get("lng")
	radiusStr := q.Get("radius")

	hasLat := latStr != ""
	hasLng := lngStr != ""
	hasRadius := radiusStr != ""

	if hasLat || hasLng || hasRadius {
		if !hasLat || !hasLng || !hasRadius {
			errs["geo"] = "lat, lng and radius must all be provided together"
		} else {
			lat, latErr := strconv.ParseFloat(latStr, 64)
			lng, lngErr := strconv.ParseFloat(lngStr, 64)
			radius, radErr := strconv.ParseFloat(radiusStr, 64)

			if latErr != nil || lat < -90 || lat > 90 {
				errs["lat"] = "must be a number between -90 and 90"
			}
			if lngErr != nil || lng < -180 || lng > 180 {
				errs["lng"] = "must be a number between -180 and 180"
			}
			if radErr != nil || radius <= 0 {
				errs["radius"] = "must be a positive number"
			}
			if latErr == nil && lngErr == nil && radErr == nil &&
				lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180 && radius > 0 {
				f.Lat = &lat
				f.Lng = &lng
				f.Radius = &radius
			}
		}
	}

	if v := q.Get("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			errs["page"] = "must be an integer >= 1"
		} else {
			f.Page = n
		}
	}

	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			errs["limit"] = "must be an integer between 1 and 100"
		} else {
			f.Limit = n
		}
	}

	return f, errs
}

// decodeJSON decodes the request body into dst. It limits the body size to
// 1 MB and rejects unknown fields to surface client errors early.
func decodeJSON(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20) // 1 MB
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

// parseUUID parses a UUID from the given string and writes a 400 response on
// failure. Returns false when the caller should stop handling the request.
func parseUUID(w http.ResponseWriter, s string) (uuid.UUID, bool) {
	id, err := uuid.Parse(s)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return uuid.Nil, false
	}
	return id, true
}
