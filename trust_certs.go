package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

func newHTTPClient(c *Config) (*http.Client, error) {
	client := &http.Client{}
	if c == nil || c.TrustCertsFile == "" {
		return client, nil
	}

	certs, err := readTrustCerts(c.TrustCertsFile)
	if err != nil {
		return nil, err
	}

	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("failed to load system certificate pool: %w", err)
	}
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}
	rootCAs.AppendCertsFromPEM(certs)

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{RootCAs: rootCAs}
	client.Transport = transport
	return client, nil
}

func newTrustedGitCommand(workingDir string, args ...string) (*exec.Cmd, func(), error) {
	cleanup := func() {}
	gitArgs := args

	if config != nil && config.TrustCertsFile != "" {
		certsFile, err := filepath.Abs(config.TrustCertsFile)
		if err != nil {
			return nil, cleanup, fmt.Errorf("failed to resolve trust-certs-file %q: %w", config.TrustCertsFile, err)
		}
		if _, err := readTrustCerts(certsFile); err != nil {
			return nil, cleanup, err
		}
		gitArgs = append([]string{"-c", "http.sslCAInfo=" + certsFile}, args...)
	}

	cmd := exec.Command("git", gitArgs...)
	cmd.Dir = workingDir
	return cmd, cleanup, nil
}

func readTrustCerts(path string) ([]byte, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read trust-certs-file %q: %w", path, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(content) {
		return nil, fmt.Errorf("Base64 encoded PEM certificate beginning with -----BEGIN CERTIFICATE----- and ending with -----END CERTIFICATE----- is expected: %s", path)
	}
	return content, nil
}
