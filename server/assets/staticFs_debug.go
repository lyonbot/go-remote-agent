//go:build !release

package assets

import (
	"io/fs"
	"os"
)

// like embed.FS !
type combinedFS interface {
	fs.FS
	fs.ReadDirFS
	fs.ReadFileFS
}

var StaticFs = os.DirFS(".").(combinedFS)
