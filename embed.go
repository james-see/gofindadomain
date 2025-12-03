package gofindadomain

import (
	_ "embed"
)

//go:embed tlds.txt
var EmbeddedTLDs string

//go:embed top-12.txt
var EmbeddedTop12 string

