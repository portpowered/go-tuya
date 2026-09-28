package tuya

import "fmt"

const (
	//https://github.com/home-assistant/core/blob/dev/homeassistant/components/tuya/const.py#L39
	// Hardcoded tuya client id for tuya sharing based access
	clientID = "HA_3y9q4ak7g4ephrvke"

	// AuthenticationSchema defines the authorization schema for tuya sharing based access
	// https://github.com/home-assistant/core/blob/dev/homeassistant/components/tuya/const.py#L40
	AuthenticationSchema = "haauthorize"

	// LoginURI is used to generate QR codes, and login and validate. This is separate from the cloud API URL.
	LoginURI = "https://apigw.iotbing.com"
)

// Region represents different Tuya cloud regions
type Region string

const (
	// TuyaRegionChina represents China data center
	TuyaRegionChina Region = "CN"
	// TuyaRegionUS represents US data center
	TuyaRegionUS Region = "US"
	// TuyaRegionEU represents Europe data center
	TuyaRegionEU Region = "EU"
	// TuyaRegionIndia represents India data center
	TuyaRegionIndia Region = "IN"
)

// Regional API endpoints for Customer API
var regionEndpoints = map[Region]string{
	TuyaRegionChina: "https://openapi.tuyacn.com",
	TuyaRegionUS:    "https://apigw.tuyaus.com",
	TuyaRegionEU:    "https://openapi.tuyaeu.com",
	TuyaRegionIndia: "https://openapi.tuyain.com",
}

// GetRegionEndpoint returns the API endpoint for a given region
func GetRegionEndpoint(region Region) (string, error) {
	endpoint, exists := regionEndpoints[region]
	if !exists {
		return "", fmt.Errorf("unsupported region: %s", region)
	}
	return endpoint, nil
}
