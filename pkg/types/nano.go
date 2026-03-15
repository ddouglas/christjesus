package types

import gonanoid "github.com/matoous/go-nanoid/v2"

const NanoIDDefaultSize = 32

var nanoidAlphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func NanoID() string {
	return NanoIDSize(NanoIDDefaultSize)
}

func NanoIDSize(size int) string {
	if size <= 0 {
		size = NanoIDDefaultSize
	}

	return gonanoid.MustGenerate(nanoidAlphabet, size)
}
