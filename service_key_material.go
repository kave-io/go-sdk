package kave

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

const (
	rawServiceKeyPrefix   = "kv2_"
	serviceKeyLookupBytes = 18
	serviceKeySecretBytes = 32
	serviceKeyLookupChars = 24
	serviceKeySecretChars = 43
)

type serviceKeyMaterial struct {
	lookupPrefix string
	secretHash   [sha256.Size]byte
	rawKey       string
}

func generateServiceKeyMaterial() (serviceKeyMaterial, error) {
	return generateServiceKeyMaterialFrom(rand.Reader)
}

func generateServiceKeyMaterialFrom(random io.Reader) (serviceKeyMaterial, error) {
	lookup := make([]byte, serviceKeyLookupBytes)
	if _, err := io.ReadFull(random, lookup); err != nil {
		return serviceKeyMaterial{}, fmt.Errorf("%w: generate service-key lookup prefix: %v", ErrInternal, err)
	}
	secret := make([]byte, serviceKeySecretBytes)
	if _, err := io.ReadFull(random, secret); err != nil {
		return serviceKeyMaterial{}, fmt.Errorf("%w: generate service-key secret: %v", ErrInternal, err)
	}
	defer clear(secret)
	lookupPrefix := base64.RawURLEncoding.EncodeToString(lookup)
	rawKey := rawServiceKeyPrefix + lookupPrefix + "." + base64.RawURLEncoding.EncodeToString(secret)
	return serviceKeyMaterial{lookupPrefix: lookupPrefix, secretHash: sha256.Sum256([]byte(rawKey)), rawKey: rawKey}, nil
}

func parseServiceKeyMaterial(rawKey string) (serviceKeyMaterial, error) {
	if rawKey == "" || strings.TrimSpace(rawKey) != rawKey {
		return serviceKeyMaterial{}, fmt.Errorf("%w: raw service key is invalid", ErrInvalidArgument)
	}
	body, ok := strings.CutPrefix(rawKey, rawServiceKeyPrefix)
	if !ok {
		return serviceKeyMaterial{}, fmt.Errorf("%w: raw service key is invalid", ErrInvalidArgument)
	}
	lookupPrefix, secret, ok := strings.Cut(body, ".")
	if !ok || !validEncodedKeyPart(lookupPrefix, serviceKeyLookupBytes, serviceKeyLookupChars) ||
		!validEncodedKeyPart(secret, serviceKeySecretBytes, serviceKeySecretChars) {
		return serviceKeyMaterial{}, fmt.Errorf("%w: raw service key is invalid", ErrInvalidArgument)
	}
	return serviceKeyMaterial{lookupPrefix: lookupPrefix, secretHash: sha256.Sum256([]byte(rawKey)), rawKey: rawKey}, nil
}

func validEncodedKeyPart(value string, decodedBytes, encodedChars int) bool {
	if len(value) != encodedChars {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	return err == nil && len(decoded) == decodedBytes && base64.RawURLEncoding.EncodeToString(decoded) == value
}
