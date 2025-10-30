package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

var secret = []byte("loljwt-whyyoustilhere-forgotten-nobody")

// generic JWT verifier
func verifyJWT[T any](token string, out *T) error {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return errors.New("invalid token format")
	}

	headerB64, payloadB64, sigB64 := parts[0], parts[1], parts[2]
	sig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return errors.New("invalid signature encoding")
	}

	data := headerB64 + "." + payloadB64
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(data))
	expectedSig := mac.Sum(nil)
	if !hmac.Equal(sig, expectedSig) {
		return errors.New("signature mismatch")
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return errors.New("invalid payload encoding")
	}
	return json.Unmarshal(payloadJSON, out)
}

// VerifyDriverJWT verifies a driver token and decodes its claims.
func VerifyDriverJWT(token string) (*DriverClaims, error) {
	var claims DriverClaims
	if err := verifyJWT(token, &claims); err != nil {
		return nil, err
	}
	return &claims, nil
}

// VerifyPassengerJWT verifies a passenger token.
func VerifyPassengerJWT(token string) (*PassengerClaims, error) {
	var claims PassengerClaims
	if err := verifyJWT(token, &claims); err != nil {
		return nil, err
	}
	return &claims, nil
}

// VerifyServiceJWT verifies a service-to-service token.
func VerifyServiceJWT(token string) (*ServiceClaims, error) {
	var claims ServiceClaims
	if err := verifyJWT(token, &claims); err != nil {
		return nil, err
	}
	return &claims, nil
}
