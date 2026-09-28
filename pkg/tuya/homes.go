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
