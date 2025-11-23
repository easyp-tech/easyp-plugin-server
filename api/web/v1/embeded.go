package web

import "embed"

//go:embed web.swagger.json
var OpenAPI embed.FS
