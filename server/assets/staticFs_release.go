//go:build release

package assets

import "embed"

//go:embed frontend
var StaticFs embed.FS
