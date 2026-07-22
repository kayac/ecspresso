package ecspresso

import (
	"embed"
	_ "embed"
)

//go:embed README.md
var readmeContent string

//go:embed docs/v1-v2.md
var docsV1V2Content string

//go:embed docs/v2-v3.md
var docsV2V3Content string

//go:embed skills
var skillsFS embed.FS
