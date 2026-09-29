package tuya

import "fmt"

const (
	//https://github.com/home-assistant/core/blob/dev/homeassistant/components/tuya/const.py#L39
	// Hardcoded tuya client id for tuya sharing based access.
	clientID = "HA_3y9q4ak7g4ephrvke"

	// AuthenticationSchema defines the authorization schema for tuya sharing based access
	// https://github.com/home-assistant/core/blob/dev/homeassistant/components/tuya/const.py#L40
	AuthenticationSchema = "haauthorize"

	// LoginURI is used to generate QR codes, and login and validate. This is separate from the cloud API URL.
	LoginURI = "https://apigw.iotbing.com"
)

// Region represents different Tuya cloud regions.
type Region string

const (
	// TuyaRegionChina represents China data center.
	TuyaRegionChina Region = "CN"
	// TuyaRegionUS represents US data center.
	TuyaRegionUS Region = "US"
	// TuyaRegionEU represents Europe data center.
	TuyaRegionEU Region = "EU"
	// TuyaRegionIndia represents India data center.
	TuyaRegionIndia Region = "IN"
)

const (
	regionAPIEndpointChina = "https://openapi.tuyacn.com"
	regionAPIEndpointUS    = "https://apigw.tuyaus.com"
	regionAPIEndpointEU    = "https://openapi.tuyaeu.com"
	regionAPIEndpointIndia = "https://openapi.tuyain.com"
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
