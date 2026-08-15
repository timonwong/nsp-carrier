package usb

import (
	"errors"
	"sort"

	"github.com/google/gousb"
)

var (
	ErrNoBulkPair        = errors.New("no usable bulk endpoint pair")
	ErrMultipleBulkPairs = errors.New("multiple usable bulk endpoint pairs")
)

type EndpointSelection struct {
	Config      int
	Interface   int
	Alternate   int
	InEndpoint  int
	OutEndpoint int
}

func FindBulkPair(descriptor *gousb.DeviceDesc) (EndpointSelection, error) {
	configNumbers := make([]int, 0, len(descriptor.Configs))
	for number := range descriptor.Configs {
		configNumbers = append(configNumbers, number)
	}
	sort.Ints(configNumbers)

	var selections []EndpointSelection
	for _, configNumber := range configNumbers {
		config := descriptor.Configs[configNumber]
		for _, iface := range config.Interfaces {
			for _, alternate := range iface.AltSettings {
				var inEndpoints []int
				var outEndpoints []int
				for _, endpoint := range alternate.Endpoints {
					if endpoint.TransferType != gousb.TransferTypeBulk {
						continue
					}
					if endpoint.Direction == gousb.EndpointDirectionIn {
						inEndpoints = append(inEndpoints, endpoint.Number)
					} else {
						outEndpoints = append(outEndpoints, endpoint.Number)
					}
				}
				sort.Ints(inEndpoints)
				sort.Ints(outEndpoints)
				for _, inEndpoint := range inEndpoints {
					for _, outEndpoint := range outEndpoints {
						selections = append(selections, EndpointSelection{
							Config:      config.Number,
							Interface:   iface.Number,
							Alternate:   alternate.Alternate,
							InEndpoint:  inEndpoint,
							OutEndpoint: outEndpoint,
						})
					}
				}
			}
		}
	}

	switch len(selections) {
	case 0:
		return EndpointSelection{}, ErrNoBulkPair
	case 1:
		return selections[0], nil
	default:
		return EndpointSelection{}, ErrMultipleBulkPairs
	}
}
