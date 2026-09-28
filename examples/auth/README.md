# QR-code authentication example

This standalone example calls `GenerateQrCodeForLogin`, displays the returned
login challenge as a terminal QR code, and then calls `ValidateLoginCode`.
Provide the Tuya access code through `TUYA_ACCESS_CODE`; do not pass it as a
command-line argument or commit it to a file.

From this directory, run:

```sh
TUYA_ACCESS_CODE="<access-code>" go run .
```

The example suppresses token values. In an application, persist the returned
access and refresh tokens in a secret manager or OS keychain. `RefreshToken`
returns replacement credentials, so serialize refresh operations and store the
new pair atomically. Do not log the access code, tokens, QR challenge, request
URLs, or unredacted errors.
