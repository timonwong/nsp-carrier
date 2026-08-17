package sphaira

import (
	"errors"
	"strings"
	"unicode/utf8"
)

var ErrInvalidWireName = errors.New("invalid SPH0 wire name")

func ValidateWireName(name string) error {
	if name == "" || name == "." || name == ".." || !utf8.ValidString(name) || strings.ContainsAny(name, "\x00\r\n/\\") {
		return ErrInvalidWireName
	}
	return nil
}
