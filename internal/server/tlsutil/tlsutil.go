package tlsutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

type CertPair struct {
	CertPath    string
	KeyPath     string
	Fingerprint string
}

func Dir(dataDir string) string { return filepath.Join(dataDir, "tls") }

func EnsureCert(dataDir, hostname string) (*CertPair, error) {
	d := Dir(dataDir)
	certPath := filepath.Join(d, "cert.pem")
	keyPath := filepath.Join(d, "key.pem")

	if _, err := os.Stat(certPath); err == nil {
		return loadExisting(certPath, keyPath)
	}

	return generate(certPath, keyPath, hostname)
}

func loadExisting(certPath, keyPath string) (*CertPair, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("tls: read cert: %w", err)
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("tls: no PEM block in cert")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("tls: parse cert: %w", err)
	}
	fp := sha256.Sum256(cert.Raw)
	return &CertPair{
		CertPath:    certPath,
		KeyPath:     keyPath,
		Fingerprint: hex.EncodeToString(fp[:]),
	}, nil
}

func generate(certPath, keyPath, hostname string) (*CertPair, error) {
	if err := os.MkdirAll(filepath.Dir(certPath), 0o700); err != nil {
		return nil, fmt.Errorf("tls: mkdir: %w", err)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("tls: generate key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("tls: generate serial: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{Organization: []string{"symvibe"}},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	if hostname != "" {
		if ip := net.ParseIP(hostname); ip != nil {
			template.IPAddresses = []net.IP{ip}
		} else {
			template.DNSNames = []string{hostname}
		}
	}
	template.IPAddresses = append(template.IPAddresses, net.ParseIP("127.0.0.1"))
	template.DNSNames = append(template.DNSNames, "localhost")

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("tls: create cert: %w", err)
	}

	certFile, err := os.Create(certPath)
	if err != nil {
		return nil, fmt.Errorf("tls: create cert file: %w", err)
	}
	defer certFile.Close()
	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return nil, fmt.Errorf("tls: write cert: %w", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("tls: marshal key: %w", err)
	}
	keyFile, err := os.Create(keyPath)
	if err != nil {
		return nil, fmt.Errorf("tls: create key file: %w", err)
	}
	defer keyFile.Close()
	if err := pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}); err != nil {
		return nil, fmt.Errorf("tls: write key: %w", err)
	}

	fp := sha256.Sum256(certDER)
	return &CertPair{
		CertPath:    certPath,
		KeyPath:     keyPath,
		Fingerprint: hex.EncodeToString(fp[:]),
	}, nil
}
