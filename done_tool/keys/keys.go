// Package certs with key management for clients (and server) certificates
package keys

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/nats-io/nkeys"
)

// DetermineKeyType returns the type of key
func DetermineKeyType(encKey string) KeyType {
	var derBytes []byte
	var err error
	blockPub, _ := pem.Decode([]byte(encKey))
	if blockPub == nil {
		// is this an nkey seed?
		_, err = nkeys.FromSeed([]byte(encKey))
		if err == nil {
			return KeyTypeNKey
		}
		// no nkey, try base64 decoding. Eg PEM content
		derBytes, _ = base64.StdEncoding.DecodeString(encKey)

		// todo: support for hex format?
	} else {
		derBytes = blockPub.Bytes
	}
	// first check the public key type
	genericPublicKey, err := x509.ParsePKIXPublicKey(derBytes)
	if err == nil {
		switch genericPublicKey.(type) {
		case *ecdsa.PublicKey:
			return KeyTypeECDSA
		case *rsa.PublicKey:
			return KeyTypeRSA
		}
	}
	// no luck yet, check private
	// PKCS1 is RSA
	_, err = x509.ParsePKCS1PrivateKey(derBytes)
	if err == nil {
		return KeyTypeRSA
	}
	// try PKCS8 encoding
	rawPrivateKey, err := x509.ParsePKCS8PrivateKey(derBytes)
	if err == nil {
		switch rawPrivateKey.(type) {
		case *ecdsa.PrivateKey:
			return KeyTypeECDSA
		case *rsa.PrivateKey:
			return KeyTypeRSA
		default:
			return KeyTypeUnknown
		}
	}

	return KeyTypeUnknown
}

// NewKey creates a new key of the given type
func NewKey(keyType KeyType) IHiveKey {
	switch keyType {
	case KeyTypeECDSA:
		return NewEcdsaKey()
	case KeyTypeNKey:
		return NewNkeysKey()
	case KeyTypeRSA:
		return NewRsaKey()
	default:
		return nil
	}
}

// NewKeyFromEnc helper creates a HiveKey instance from an encoded private key.
// This returns nil if the key type cannot be determined
//
//	privEnc is the encoded private key
func NewKeyFromEnc(privEnc string) IHiveKey {
	keyType := DetermineKeyType(privEnc)
	if keyType == KeyTypeUnknown {
		return nil
	}
	key := NewKey(keyType)
	_ = key.ImportPrivate(privEnc)
	return key
}

// NewKeyFromFile helper to load a public/private key pair from file
// This returns nil if the key type cannot be determined
//
//	keyPath is the path to the file containing the key
func NewKeyFromFile(keyPath string) (IHiveKey, error) {
	keyEnc, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	k := NewKeyFromEnc(string(keyEnc))
	if k == nil {
		err = fmt.Errorf("unknown key format")
	}
	return k, err
}
