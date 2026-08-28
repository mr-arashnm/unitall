// Package openapi embeds the consolidated Unital API specification
// (single source of truth: openapi/unital-v1.yaml) so the gateway can
// serve it at /api/schema without external files.
package openapi

import _ "embed"

// Spec is the raw OpenAPI 3.1 YAML document.
//
//go:embed unital-v1.yaml
var Spec []byte
