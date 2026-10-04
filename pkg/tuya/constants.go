package tuya

import (
	"fmt"

	wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
)

const (
	// LoginURI is used to generate QR codes, and login and validate. This is separate from the cloud API URL.
	LoginURI = string(wire.AuthenticationOriginDefault)
)

// Region represents different Tuya cloud regions.
type Region string

const (
	// TuyaRegionChina represents China data center.
	TuyaRegionChina Region = Region(wire.CloudRegionChina)
	// TuyaRegionUS represents US data center.
	TuyaRegionUS Region = Region(wire.CloudRegionUS)
	// TuyaRegionEU represents Europe data center.
	TuyaRegionEU Region = Region(wire.CloudRegionEU)
	// TuyaRegionIndia represents India data center.
	TuyaRegionIndia Region = Region(wire.CloudRegionIndia)
)

const (
	regionAPIEndpointChina = string(wire.CloudAPIOriginChina)
	regionAPIEndpointUS    = "https://apigw.tuyaus.com"
	regionAPIEndpointEU    = "https://openapi.tuyaeu.com"
	regionAPIEndpointIndia = string(wire.CloudAPIOriginIndia)
)

// GetRegionEndpoint returns the API endpoint for a given region.
func GetRegionEndpoint(region Region) (string, error) {
	switch region {
	case TuyaRegionChina:
		return regionAPIEndpointChina, nil
	case TuyaRegionUS:
		return regionAPIEndpointUS, nil
	case TuyaRegionEU:
		return regionAPIEndpointEU, nil
	case TuyaRegionIndia:
		return regionAPIEndpointIndia, nil
	default:
		return "", fmt.Errorf("%w: %s", errUnsupportedRegion, region)
	}
}
