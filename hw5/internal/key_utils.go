package internal

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

func LoadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("failed to decode PEM block containing public key")
	}

	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	// type assert to RSA
	rsaKey, ok := key.(*rsa.PublicKey)
	if !ok {
			return nil, errors.New("not an RSA public key")
	}

	return rsaKey, nil
}

func LoadPrivateKey(path string) (*rsa.PrivateKey, error) {
		raw, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    block, _ := pem.Decode(raw)
    if block == nil {
        return nil, errors.New("failed to parse PEM block")
    }

    key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
    if err != nil {
        return nil, fmt.Errorf("parse pkcs8: %w", err)
    }

    // type assert to RSA
    rsaKey, ok := key.(*rsa.PrivateKey)
    if !ok {
        return nil, errors.New("not an RSA private key")
    }

    return rsaKey, nil
}
