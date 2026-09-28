# Authentication and token handling

The client supports QR-code authentication through the Tuya Device Sharing
flow. `GenerateQrCodeForLogin` returns a login code and formatted QR payload;
render that payload in a QR-capable terminal or application, have the account
owner approve it, then pass the login code and original access code to
`ValidateLoginCode`.

```go
login, err := client.AuthService.GenerateQrCodeForLogin(ctx, tuya.LoginRequest{
    AccessCode: accessCode,
    Schema:     tuya.AuthenticationSchema,
})
if err != nil {
    return err
}

// Render login.QrFormattedCode with a QR library and wait for approval.
tokens, err := client.AuthService.ValidateLoginCode(ctx, tuya.ValidateLoginCodeRequest{
    LoginCode: login.Code,
    UserCode:  accessCode,
})
if err != nil {
    return err
}
// Securely persist tokens.AccessToken, tokens.RefreshToken, and tokens.ExpireTime.
```

The access code, QR challenge, access token, and refresh token are credentials.
Do not put them in source control, command-line arguments, logs, or terminal
transcripts. The authentication requests include values in their URLs, so
redact URLs and errors before sharing diagnostics.

## Existing tokens and refresh

Applications can initialize a client with existing credentials:

```go
client := tuya.NewClient(&tuya.ClientConfig{
    HTTPClient: &http.Client{Timeout: 30 * time.Second},
    AuthInformation: &tuya.AuthInformation{
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
        ExpireTime:   expiryMilliseconds,
    },
})
```

The package returns refreshed token values but does not persist them or
coordinate concurrent refresh calls. Serialize refreshes and write the new
access/refresh token pair and expiration together. If persistence fails, do not
assume the previous refresh token is still usable; the provider's refresh flow
may rotate it. See the [safe example](../examples/token_refresh/token_refresh.go).

## Errors and transport

Authentication methods return ordinary Go errors. This version does not
expose a stable error type for programmatic classification. Inject a configured
`*http.Client` using `ClientConfig.HTTPClient` to set a timeout or custom
transport. The default client has no timeout.
