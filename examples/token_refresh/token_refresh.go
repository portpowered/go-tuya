// Package main demonstrates a token refresh without printing credentials.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/portpowered/go-tuya/pkg/tuya"
)

func main() {
	accessToken := os.Getenv("TUYA_AUTH_TOKEN")
	refreshToken := os.Getenv("TUYA_REFRESH_TOKEN")

	expireTime, err := strconv.ParseInt(os.Getenv("TUYA_AUTH_TOKEN_EXPIRED"), 10, 64)

	if accessToken == "" || refreshToken == "" || err != nil {
		log.Fatal("set TUYA_AUTH_TOKEN, TUYA_REFRESH_TOKEN, and TUYA_AUTH_TOKEN_EXPIRED")
	}

	base, err := tuya.NewClient(tuya.WithClientID(os.Getenv("TUYA_CLIENT_ID")))
	if err != nil {
		log.Fatal("could not configure the Tuya client")
	}

	client := base.NewSession(tuya.Tokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpireTime:   expireTime,
	})

	updated, err := client.AuthService.RefreshToken(context.Background(), tuya.RefreshTokenRequest{
		Request:      tuya.Request{AuthorizationContext: nil}, //nolint:exhaustruct,exhaustruct_v5 // The deprecated refresh switch intentionally stays zero.
		RefreshToken: refreshToken,
	})
	if err != nil {
		log.Fatal("token refresh failed; redact credentials before inspecting diagnostics")
	}

	_ = updated // Persist rotated tokens atomically with application-managed secret storage.

	fmt.Println("Token refresh succeeded. Persist the rotated tokens securely; do not reuse the previous refresh token.")
}
