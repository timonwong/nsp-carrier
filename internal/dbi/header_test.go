package dbi_test

import (
	"encoding/hex"
	"testing"

	"github.com/timonwong/nsp-carrier/internal/dbi"
)

func TestHeaderRoundTripMatchesObservedWireFormat(t *testing.T) {
	want, err := hex.DecodeString("44424930020000000200000020000000")
	if err != nil {
		t.Fatal(err)
	}

	encoded := dbi.EncodeHeader(dbi.Header{
		Type:        dbi.CommandTypeAcknowledgement,
		Command:     dbi.CommandFileRange,
		PayloadSize: 32,
	})
	if string(encoded[:]) != string(want) {
		t.Fatalf("EncodeHeader() = %x, want %x", encoded, want)
	}

	decoded, err := dbi.DecodeHeader(encoded[:])
	if err != nil {
		t.Fatalf("DecodeHeader() error = %v", err)
	}
	if decoded.Type != dbi.CommandTypeAcknowledgement ||
		decoded.Command != dbi.CommandFileRange ||
		decoded.PayloadSize != 32 {
		t.Fatalf("DecodeHeader() = %#v", decoded)
	}
}

func FuzzDecodeHeader(f *testing.F) {
	f.Add([]byte("DBI0\x00\x00\x00\x00\x03\x00\x00\x00\x00\x00\x00\x00"))
	f.Add([]byte("not-a-header"))
	f.Fuzz(func(t *testing.T, encoded []byte) {
		_, _ = dbi.DecodeHeader(encoded)
	})
}
