// Package api embarque le contrat OpenAPI du service.
package api

import _ "embed"

// Spec est le contenu de openapi.yaml.
//
//go:embed openapi.yaml
var Spec []byte
