package tuya

import (
	"context"

	"github.com/portpowered/go-tuya/pkg/tuya/internal/wire"
)

// HomeService provides methods for managing smart homes
type HomeService service

// QueryHomes retrieves all homes for the authenticated user
func (h *HomeService) QueryHomes(ctx context.Context, req QueryHomesRequest) (QueryHomesResponse, error) {
	resp, err := h.client.EncryptedClient.requestOperation(ctx, wire.OperationQueryHomes(), nil, nil, nil, &req)
	if err != nil {
		return QueryHomesResponse{}, err
	}

	wireResponse, err := decodeWireResponse[wire.HomeListEnvelope](resp)
	if err != nil {
		return QueryHomesResponse{}, err
	}
	var wireHomes []wire.HomeRecord
	if wireResponse.Result != nil {
		wireHomes = *wireResponse.Result
	}

	homes := mapWireHomes(wireHomes)
	return QueryHomesResponse{
		Results: homes,
	}, nil
}

func mapWireHomes(records []wire.HomeRecord) []Home {
	homes := make([]Home, len(records))
	for i, home := range records {
		homes[i] = Home{
			// The home id is the owner id. Confusing.
			ID:      dereference(home.OwnerId),
			Name:    dereference(home.Name),
			GeoName: dereference(home.GeoName),
		}
	}
	return homes
}

// "{\"result\":[{\"background\":\"\",\"geoName\":\"\",\"gmtCreate\":12312,\"gmtModified\":1231,\"groupId\":123123,\"id\":12313,\"lat\":0,\"lon\":0,
// \"name\":\"My Home ..\",\"ownerId\":\"123123\",\"status\":true,\"uid\":\"123123\"}],\"sign\":\"123\",\"success\":true,\"t\":123123,\"tid\":\"123\"}"

// HomeResponseResult represents the response structure for home queries
type HomeResponseResult struct {
	Result []HomeResponseResultElement `json:"result"`
}

// HomeResponseResultElement represents a single home in the response
// https://developer.tuya.com/en/docs/cloud/0dbe66fef6
type HomeResponseResultElement struct {
	GeoName    string  `json:"geoName"`
	GroupID    int     `json:"groupId"`
	Name       string  `json:"name"`
	OwnerID    string  `json:"ownerId"`
	ID         int     `json:"id"`
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
	Background string  `json:"background"`
	Status     bool    `json:"status"`
	UID        string  `json:"uid"`
}

// HomeResponse represents the response from home-related API calls
type HomeResponse struct {
	Success bool               `json:"success"`
	Tid     string             `json:"tid"`
	T       int64              `json:"t"`
	Result  HomeResponseResult `json:"result"`
}
