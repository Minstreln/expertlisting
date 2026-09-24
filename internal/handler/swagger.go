package handler

import (
	"expert-listing/docs"
	"net/http"
)

// SwaggerHandler serves the Swagger UI and the raw OpenAPI specification.
type SwaggerHandler struct{}

// RegisterRoutes wires the Swagger UI and spec routes onto mux.
func (h SwaggerHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /swagger/openapi.yaml", h.Spec)
	mux.HandleFunc("GET /swagger/", h.UI)
}

// Spec serves the raw OpenAPI YAML specification.
func (h SwaggerHandler) Spec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.WriteHeader(http.StatusOK)
	w.Write(docs.OpenAPISpec) //nolint:errcheck
}

// UI serves a Swagger UI HTML page that loads the spec from /swagger/openapi.yaml.
func (h SwaggerHandler) UI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(swaggerUIHTML)) //nolint:errcheck
}

const swaggerUIHTML = `<!DOCTYPE html>
<html>
<head>
  <title>Expert Listing API</title>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
<script>
  SwaggerUIBundle({
    url: "/swagger/openapi.yaml",
    dom_id: "#swagger-ui",
    presets: [
      SwaggerUIBundle.presets.apis,
      SwaggerUIBundle.SwaggerUIStandalonePreset
    ],
    layout: "BaseLayout",
    deepLinking: true
  });
</script>
</body>
</html>`
