package cryptutil

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"time"
)

// VerifyCertificateExceptServerName verifies a certificate chain without hostname verification.
// This function performs the same validation as the standard TLS verification process,
// but deliberately omits DNS name checking to support scenarios where certificates
// may not match the internal Kubernetes service names.
func VerifyCertificateExceptServerName(rawCerts [][]byte, config *tls.Config) ([]*x509.Certificate, [][]*x509.Certificate, error) {
	if len(rawCerts) == 0 {
		return nil, nil, errors.New("tls: no certificates provided by peer")
	}

	if config == nil {
		return nil, nil, errors.New("tls: config cannot be nil")
	}

	if config.RootCAs == nil {
		return nil, nil, errors.New("tls: no root CAs configured for verification")
	}

	// Parse all certificates in the chain
	certs := make([]*x509.Certificate, len(rawCerts))
	for i, asn1Data := range rawCerts {
		cert, err := x509.ParseCertificate(asn1Data)
		if err != nil {
			return nil, nil, fmt.Errorf("tls: failed to parse certificate at index %d: %w", i, err)
		}
		certs[i] = cert
	}

	// Get the verification time
	var verifyTime time.Time
	if config.Time != nil {
		verifyTime = config.Time()
	} else {
		verifyTime = time.Now()
	}

	// Build the intermediate certificate pool from the certificate chain
	intermediates := x509.NewCertPool()
	for i := 1; i < len(certs); i++ {
		intermediates.AddCert(certs[i])
	}

	// Set up verification options without DNS name validation
	// This is the key difference from standard TLS verification
	opts := x509.VerifyOptions{
		Roots:         config.RootCAs,
		Intermediates: intermediates,
		CurrentTime:   verifyTime,
		// Deliberately omit DNSName to skip hostname verification
		// This allows certificates that don't match the server hostname
		// but are still valid and signed by a trusted CA
	}

	// Verify the leaf certificate (first in the chain)
	leafCert := certs[0]
	chains, err := leafCert.Verify(opts)
	if err != nil {
		return nil, nil, fmt.Errorf("tls: certificate verification failed: %w", err)
	}

	return certs, chains, nil
}
