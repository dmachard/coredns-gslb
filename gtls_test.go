package gslb

import (
	"os"
	"path/filepath"
	"testing"
)

func getPEMFiles(t *testing.T) (cert, key, ca string) {
	t.Helper()
	tempDir, err := writePEMFiles(t)
	if err != nil {
		t.Fatalf("Could not write PEM files: %s", err)
	}

	cert = filepath.Join(tempDir, "cert.pem")
	key = filepath.Join(tempDir, "key.pem")
	ca = filepath.Join(tempDir, "ca.pem")

	return
}

func TestNewTLSClientConfig(t *testing.T) {
	cert, key, ca := getPEMFiles(t)
	_, err := NewTLSClientConfig(cert, key, ca)
	if err != nil {
		t.Errorf("Failed to create TLSConfig: %s", err)
	}
}

func TestNewHTTPSTransport(t *testing.T) {
	cert, key, ca := getPEMFiles(t)

	cc, err := NewTLSClientConfig(cert, key, ca)
	if err != nil {
		t.Errorf("Failed to create TLSConfig: %s", err)
	}

	tr := NewHTTPSTransport(cc)
	if tr == nil {
		t.Errorf("Failed to create https transport with cc")
	}

	tr = NewHTTPSTransport(nil)
	if tr == nil {
		t.Errorf("Failed to create https transport without cc")
	}
}

// WritePEMFiles creates a tmp dir with ca.pem, cert.pem, and key.pem
func writePEMFiles(t *testing.T) (string, error) {
	t.Helper()
	tempDir := t.TempDir()

	data := `-----BEGIN CERTIFICATE-----
MIIBizCCAT2gAwIBAgIUM3BRGwa3MOn9ZZjjkBm6SpWVyIAwBQYDK2VwMBoxGDAW
BgNVBAMMD2NvcmVkbnMtZ3NsYi1jYTAeFw0yNjA1MDUxNDMwNTFaFw0yNjA2MDQx
NDMwNTFaMBoxGDAWBgNVBAMMD2NvcmVkbnMtZ3NsYi1jYTAqMAUGAytlcAMhAGyt
VrpY/nAJeWjSwqN0xrfbrHZIu6HwRVLpLkPktZ2Io4GUMIGRMB0GA1UdDgQWBBSO
rrsTnQfvh2dMifRkp6OZSvZqRjBVBgNVHSMETjBMgBSOrrsTnQfvh2dMifRkp6OZ
SvZqRqEepBwwGjEYMBYGA1UEAwwPY29yZWRucy1nc2xiLWNhghQzcFEbBrcw6f1l
mOOQGbpKlZXIgDAMBgNVHRMEBTADAQH/MAsGA1UdDwQEAwIBBjAFBgMrZXADQQCQ
3NUdBtmZkPupsOIj442J08CeG1dsHyT+f0jX0THtfx5w1/wHX52OJtOnIDxSBzHv
K8dIO5Sl4vXm3wuG8QEG
-----END CERTIFICATE-----`
	path := filepath.Join(tempDir, "ca.pem")
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		return "", err
	}
	data = `-----BEGIN CERTIFICATE-----
MIIBzDCCAX6gAwIBAgIUFnjMPKZyapZluZRgQdzxVaUSvaMwBQYDK2VwMBoxGDAW
BgNVBAMMD2NvcmVkbnMtZ3NsYi1jYTAeFw0yNjA1MDUxNDMwNTFaFw0yNjA2MDQx
NDMwNTFaMBsxGTAXBgNVBAMMEGNkbnMtZ3NsYi1jbGllbnQwKjAFBgMrZXADIQCT
pbb/9fjix7kUFpUycEr274PoQDo5mytYHSoBH5stoqOB1DCB0TAJBgNVHRMEAjAA
MB0GA1UdDgQWBBR+8ZU50qEZ2TKFRZlgVy2uSdxvqTBVBgNVHSMETjBMgBSOrrsT
nQfvh2dMifRkp6OZSvZqRqEepBwwGjEYMBYGA1UEAwwPY29yZWRucy1nc2xiLWNh
ghQzcFEbBrcw6f1lmOOQGbpKlZXIgDALBgNVHQ8EBAMCBaAwEwYDVR0lBAwwCgYI
KwYBBQUHAwIwLAYDVR0RBCUwI4IQY2Rucy1nc2xiLWNsaWVudIIJbG9jYWxob3N0
hwSsEAAJMAUGAytlcANBAD5Ctw4q7RBDTgeDx7H4sojcHfkkGawrk6UsRGKPbpqI
mHhwEoKCDlS6kAh5B4fmyXbT/fWRPrn15mxZh3dfSQE=
-----END CERTIFICATE-----`
	path = filepath.Join(tempDir, "cert.pem")
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		return "", err
	}

	//nolint:gosec // Test fixture private key.
	data = `-----BEGIN PRIVATE KEY-----
MC4CAQAwBQYDK2VwBCIEIAyPq2Ewm+RPPw617qcne588ouPmlY1v3Jed0M+F1Y9k
-----END PRIVATE KEY-----`
	path = filepath.Join(tempDir, "key.pem")
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		return "", err
	}

	return tempDir, nil
}
