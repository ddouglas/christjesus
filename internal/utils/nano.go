package utils

import gonanoid "github.com/matoous/go-nanoid/v2"

var (
	NanoidSize     = 32
	nanoidAlphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

func NanoID() string {
	return NanoIDSize(NanoidSize)
}

func NanoIDSize(size int) string {
	if size == 0 {
		size = NanoidSize
	}

	return gonanoid.MustGenerate(nanoidAlphabet, size)
}
