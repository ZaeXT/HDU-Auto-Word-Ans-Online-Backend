package utils

import (
	"crypto/rand"
	"io"
	"strings"
)

const Pj = "useandom-26T198340PX75pxJACKVERYMINDBUSHWOLF_GQZbfghjklqvwyzrict"

func GenerateSklTicket() (string, error) {
	length := 21
	bytes := make([]byte, length)

	if _, err := io.ReadFull(rand.Reader, bytes); err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.Grow(length)

	for _, b := range bytes {
		builder.WriteByte(Pj[b&63])
	}

	return builder.String(), nil
}
