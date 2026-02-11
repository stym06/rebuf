package signing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"
)

const (
	SignatureHeaderKey = "X-Rebuf-Signature"
	TimestampHeaderKey = "X-Rebuf-Timestamp"
	IDHeaderKey        = "X-Rebuf-ID"
)

// Sign creates an HMAC-SHA256 signature for the webhook payload.
// Format: v1,<base64-encoded-signature>
func Sign(secret string, msgID string, timestamp time.Time, payload []byte) string {
	ts := fmt.Sprintf("%d", timestamp.Unix())
	toSign := fmt.Sprintf("%s.%s.%s", msgID, ts, string(payload))

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(toSign))
	sig := mac.Sum(nil)

	return "v1," + base64.StdEncoding.EncodeToString(sig)
}

// Verify checks an HMAC-SHA256 signature against the expected value.
func Verify(secret string, msgID string, timestamp time.Time, payload []byte, signature string) bool {
	expected := Sign(secret, msgID, timestamp, payload)
	return hmac.Equal([]byte(expected), []byte(signature))
}
