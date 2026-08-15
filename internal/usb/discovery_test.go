package usb_test

import (
	"errors"
	"testing"

	"github.com/google/gousb"
	usbtransport "github.com/timonwong/ya-dbibackend/internal/usb"
)

func TestFindBulkPairDiscoversDescriptorCoordinates(t *testing.T) {
	descriptor := &gousb.DeviceDesc{Configs: map[int]gousb.ConfigDesc{
		1: {
			Number: 1,
			Interfaces: []gousb.InterfaceDesc{{
				Number: 2,
				AltSettings: []gousb.InterfaceSetting{{
					Number:    2,
					Alternate: 3,
					Endpoints: map[gousb.EndpointAddress]gousb.EndpointDesc{
						0x81: {Address: 0x81, Number: 1, Direction: gousb.EndpointDirectionIn, TransferType: gousb.TransferTypeBulk},
						0x02: {Address: 0x02, Number: 2, Direction: gousb.EndpointDirectionOut, TransferType: gousb.TransferTypeBulk},
					},
				}},
			}},
		},
	}}

	selection, err := usbtransport.FindBulkPair(descriptor)
	if err != nil {
		t.Fatalf("FindBulkPair() error = %v", err)
	}
	want := usbtransport.EndpointSelection{Config: 1, Interface: 2, Alternate: 3, InEndpoint: 1, OutEndpoint: 2}
	if selection != want {
		t.Fatalf("FindBulkPair() = %#v, want %#v", selection, want)
	}
}

func TestFindBulkPairRequiresExactlyOneUsablePair(t *testing.T) {
	endpointPair := func(number int) gousb.InterfaceSetting {
		return gousb.InterfaceSetting{
			Number: number,
			Endpoints: map[gousb.EndpointAddress]gousb.EndpointDesc{
				0x81: {Address: 0x81, Number: 1, Direction: gousb.EndpointDirectionIn, TransferType: gousb.TransferTypeBulk},
				0x01: {Address: 0x01, Number: 1, Direction: gousb.EndpointDirectionOut, TransferType: gousb.TransferTypeBulk},
			},
		}
	}

	tests := []struct {
		name string
		desc *gousb.DeviceDesc
		want error
	}{
		{name: "none", desc: &gousb.DeviceDesc{Configs: map[int]gousb.ConfigDesc{}}, want: usbtransport.ErrNoBulkPair},
		{
			name: "multiple",
			desc: &gousb.DeviceDesc{Configs: map[int]gousb.ConfigDesc{
				1: {Number: 1, Interfaces: []gousb.InterfaceDesc{{Number: 0, AltSettings: []gousb.InterfaceSetting{endpointPair(0), endpointPair(0)}}}},
			}},
			want: usbtransport.ErrMultipleBulkPairs,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := usbtransport.FindBulkPair(test.desc)
			if !errors.Is(err, test.want) {
				t.Fatalf("FindBulkPair() error = %v, want %v", err, test.want)
			}
		})
	}
}
