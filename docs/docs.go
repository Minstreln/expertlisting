// Package docs embeds the OpenAPI specification for the Expert Listing API.
package docs

import _ "embed"

//go:embed openapi.yaml
var OpenAPISpec []byte
