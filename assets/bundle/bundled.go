package bundle

import (
	"fyne.io/fyne/v2"
	_ "embed"
)

//go:embed ../icons/png/icon-256.png
var iconData []byte

var ResourceIcon256Png = &fyne.StaticResource{
	StaticName:    "icon-256.png",
	StaticContent: iconData,
}
