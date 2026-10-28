package ssh

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cryptossh "golang.org/x/crypto/ssh"

	"github.com/ilaziness/vela/internal/sshkey"
)

func newTestSigner(t *testing.T) cryptossh.Signer {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := cryptossh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

func methodTypes(methods []cryptossh.AuthMethod) []string {
	out := make([]string, len(methods))
	for i, method := range methods {
		out[i] = fmt.Sprintf("%T", method)
	}
	return out
}

func TestAuthMethodsAgentOnly(t *testing.T) {
	methods, err := authMethods(Endpoint{}, []cryptossh.Signer{newTestSigner(t)})
	if err != nil {
		t.Fatal(err)
	}
	got := methodTypes(methods)
	if len(got) != 1 || !strings.Contains(got[0], "publicKey") {
		t.Fatalf("methods = %v", got)
	}
}

func TestAuthMethodsAgentWithoutKeys(t *testing.T) {
	methods, err := authMethods(Endpoint{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(methods) != 0 {
		t.Fatalf("methods = %v", methodTypes(methods))
	}
}

func TestAuthMethodsAgentAndPassword(t *testing.T) {
	methods, err := authMethods(Endpoint{Password: "secret"}, []cryptossh.Signer{newTestSigner(t)})
	if err != nil {
		t.Fatal(err)
	}
	got := methodTypes(methods)
	if len(got) != 2 || !strings.Contains(got[0], "publicKey") || !strings.Contains(got[1], "password") {
		t.Fatalf("methods = %v", got)
	}
}

func TestSerialReadWriterNotCloser(t *testing.T) {
	var rw io.ReadWriter = serialReadWriter{rw: bytes.NewBuffer(nil)}
	if _, ok := rw.(io.Closer); ok {
		t.Fatal("serial wrapper must not implement io.Closer")
	}
}

func TestAuthMethodsEmpty(t *testing.T) {
	methods, err := authMethods(Endpoint{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(methods) != 0 {
		t.Fatalf("methods = %v", methodTypes(methods))
	}
}

func TestFinalizeAuthMethodsEmpty(t *testing.T) {
	_, err := finalizeAuthMethods(nil, nil)
	if !errors.Is(err, ErrNoAuthMethods) {
		t.Fatalf("err = %v", err)
	}
}

func TestAuthMethodOrder(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, err := cryptossh.MarshalPrivateKey(key, "")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "id_ecdsa")
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}
	methods, err := authMethods(Endpoint{Key: path, Password: "secret"}, []cryptossh.Signer{newTestSigner(t)})
	if err != nil {
		t.Fatal(err)
	}
	keyboard := cryptossh.KeyboardInteractive(func(string, string, []string, []bool) ([]string, error) {
		return nil, nil
	})
	methods, err = finalizeAuthMethods(methods, keyboard)
	if err != nil {
		t.Fatal(err)
	}
	got := methodTypes(methods)
	if len(got) != 4 ||
		!strings.Contains(got[0], "publicKey") ||
		!strings.Contains(got[1], "publicKey") ||
		!strings.Contains(got[2], "password") ||
		!strings.Contains(got[3], "KeyboardInteractive") {
		t.Fatalf("methods = %v", got)
	}
}

func TestAuthMethodsKeyPEM(t *testing.T) {
	_, privatePEM, err := sshkey.Generate(sshkey.AlgoEd25519, "test")
	if err != nil {
		t.Fatal(err)
	}
	methods, err := authMethods(Endpoint{KeyPEM: privatePEM, Key: "ignored"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := methodTypes(methods)
	if len(got) != 1 || !strings.Contains(got[0], "publicKey") {
		t.Fatalf("methods = %v", got)
	}
}

func TestAgentUnavailableIsFatal(t *testing.T) {
	if !agentUnavailableIsFatal(Endpoint{}, false) {
		t.Fatal("expected fatal without other auth")
	}
	if agentUnavailableIsFatal(Endpoint{Password: "x"}, false) {
		t.Fatal("password should keep dialing")
	}
	if agentUnavailableIsFatal(Endpoint{Key: "k"}, false) {
		t.Fatal("key file should keep dialing")
	}
	if agentUnavailableIsFatal(Endpoint{KeyPEM: "pem"}, false) {
		t.Fatal("stored key should keep dialing")
	}
	if agentUnavailableIsFatal(Endpoint{}, true) {
		t.Fatal("keyboard-interactive should keep dialing")
	}
	if !agentUnavailableIsFatal(Endpoint{Certificate: "/cert.pub"}, true) {
		t.Fatal("certificate without local key requires agent even with keyboard")
	}
	if agentUnavailableIsFatal(Endpoint{Certificate: "/cert.pub", Key: "k"}, true) {
		t.Fatal("certificate with key file should keep dialing")
	}
}

func TestUserCertificateSignerMatchesKey(t *testing.T) {
	user := newTestSigner(t)
	path := writeTestCert(t, user, []string{"root"}, cryptossh.UserCert, time.Now().Add(-time.Minute), time.Now().Add(time.Hour))
	got, fromAgent, err := userCertificateSigner(path, "root", user, nil)
	if err != nil {
		t.Fatal(err)
	}
	if fromAgent {
		t.Fatal("expected file key, not agent")
	}
	cert, ok := got.PublicKey().(*cryptossh.Certificate)
	if !ok || cert.CertType != cryptossh.UserCert {
		t.Fatalf("public key = %T", got.PublicKey())
	}
}

func TestUserCertificateSignerEmptyPrincipals(t *testing.T) {
	user := newTestSigner(t)
	path := writeTestCert(t, user, nil, cryptossh.UserCert, time.Now().Add(-time.Minute), time.Now().Add(time.Hour))
	if _, _, err := userCertificateSigner(path, "anyone", user, nil); err != nil {
		t.Fatal(err)
	}
}

func TestUserCertificateSignerKeyMismatch(t *testing.T) {
	user := newTestSigner(t)
	path := writeTestCert(t, user, []string{"root"}, cryptossh.UserCert, time.Now().Add(-time.Minute), time.Now().Add(time.Hour))
	_, _, err := userCertificateSigner(path, "root", newTestSigner(t), nil)
	if err == nil || !strings.Contains(err.Error(), "不匹配") {
		t.Fatalf("err = %v", err)
	}
}

func TestUserCertificateSignerRejectsHostCert(t *testing.T) {
	user := newTestSigner(t)
	path := writeTestCert(t, user, nil, cryptossh.HostCert, time.Now().Add(-time.Minute), time.Now().Add(time.Hour))
	_, _, err := userCertificateSigner(path, "root", user, nil)
	if err == nil || !strings.Contains(err.Error(), "用户证书") {
		t.Fatalf("err = %v", err)
	}
}

func TestUserCertificateSignerExpired(t *testing.T) {
	user := newTestSigner(t)
	path := writeTestCert(t, user, []string{"root"}, cryptossh.UserCert, time.Now().Add(-2*time.Hour), time.Now().Add(-time.Minute))
	_, _, err := userCertificateSigner(path, "root", user, nil)
	if err == nil || !strings.Contains(err.Error(), "过期") {
		t.Fatalf("err = %v", err)
	}
}

func TestUserCertificateSignerNotYetValid(t *testing.T) {
	user := newTestSigner(t)
	path := writeTestCert(t, user, []string{"root"}, cryptossh.UserCert, time.Now().Add(time.Hour), time.Now().Add(2*time.Hour))
	_, _, err := userCertificateSigner(path, "root", user, nil)
	if err == nil || !strings.Contains(err.Error(), "尚未生效") {
		t.Fatalf("err = %v", err)
	}
}

func TestUserCertificateSignerPrincipalMismatch(t *testing.T) {
	user := newTestSigner(t)
	path := writeTestCert(t, user, []string{"alice"}, cryptossh.UserCert, time.Now().Add(-time.Minute), time.Now().Add(time.Hour))
	_, _, err := userCertificateSigner(path, "root", user, nil)
	if err == nil || !strings.Contains(err.Error(), "不适用于用户") {
		t.Fatalf("err = %v", err)
	}
}

func TestUserCertificateSignerMatchesAgent(t *testing.T) {
	match := newTestSigner(t)
	other := newTestSigner(t)
	path := writeTestCert(t, match, []string{"root"}, cryptossh.UserCert, time.Now().Add(-time.Minute), time.Now().Add(time.Hour))
	got, fromAgent, err := userCertificateSigner(path, "root", nil, []cryptossh.Signer{other, match})
	if err != nil {
		t.Fatal(err)
	}
	if !fromAgent {
		t.Fatal("expected agent key")
	}
	if _, ok := got.PublicKey().(*cryptossh.Certificate); !ok {
		t.Fatalf("public key = %T", got.PublicKey())
	}
	methods, err := authMethods(Endpoint{Certificate: path, User: "root"}, []cryptossh.Signer{other, match})
	if err != nil {
		t.Fatal(err)
	}
	gotMethods := methodTypes(methods)
	if len(gotMethods) != 2 || !strings.Contains(gotMethods[0], "publicKey") || !strings.Contains(gotMethods[1], "publicKey") {
		t.Fatalf("methods = %v", gotMethods)
	}
}

func TestUserCertificateSignerMatchesAgentAdvertisedCert(t *testing.T) {
	raw := newTestSigner(t)
	path := writeTestCert(t, raw, []string{"root"}, cryptossh.UserCert, time.Now().Add(-time.Minute), time.Now().Add(time.Hour))
	loaded, _, err := userCertificateSigner(path, "root", raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	agent := publicKeySigner{Signer: raw, pub: loaded.PublicKey()}
	got, fromAgent, err := userCertificateSigner(path, "root", nil, []cryptossh.Signer{agent})
	if err != nil {
		t.Fatal(err)
	}
	if !fromAgent {
		t.Fatal("expected agent key")
	}
	if _, ok := got.PublicKey().(*cryptossh.Certificate); !ok {
		t.Fatalf("public key = %T", got.PublicKey())
	}
	methods, err := authMethods(Endpoint{Certificate: path, User: "root"}, []cryptossh.Signer{agent})
	if err != nil {
		t.Fatal(err)
	}
	if len(methods) != 1 {
		t.Fatalf("methods = %v", methodTypes(methods))
	}
}

type algCertSigner struct {
	publicKeySigner
	alg cryptossh.AlgorithmSigner
}

func (s algCertSigner) SignWithAlgorithm(rand io.Reader, data []byte, algorithm string) (*cryptossh.Signature, error) {
	return s.alg.SignWithAlgorithm(rand, data, algorithm)
}

func TestUserCertificateSignerKeepsAgentAlgorithm(t *testing.T) {
	raw := newTestSigner(t)
	alg, ok := raw.(cryptossh.AlgorithmSigner)
	if !ok {
		t.Fatal("test signer has no algorithm support")
	}
	path := writeTestCert(t, raw, []string{"root"}, cryptossh.UserCert, time.Now().Add(-time.Minute), time.Now().Add(time.Hour))
	loaded, _, err := userCertificateSigner(path, "root", raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	agent := algCertSigner{publicKeySigner: publicKeySigner{Signer: raw, pub: loaded.PublicKey()}, alg: alg}
	got, _, err := userCertificateSigner(path, "root", nil, []cryptossh.Signer{agent})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.(cryptossh.AlgorithmSigner); !ok {
		t.Fatalf("cert signer = %T", got)
	}
}

func TestAuthMethodsCertificateAndPassword(t *testing.T) {
	_, privatePEM, err := sshkey.Generate(sshkey.AlgoEd25519, "test")
	if err != nil {
		t.Fatal(err)
	}
	signer, err := signerFromPEM(privatePEM)
	if err != nil {
		t.Fatal(err)
	}
	path := writeTestCert(t, signer, []string{"root"}, cryptossh.UserCert, time.Now().Add(-time.Minute), time.Now().Add(time.Hour))
	methods, err := authMethods(Endpoint{KeyPEM: privatePEM, Certificate: path, User: "root", Password: "secret"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := methodTypes(methods)
	if len(got) != 2 || !strings.Contains(got[0], "publicKey") || !strings.Contains(got[1], "password") {
		t.Fatalf("methods = %v", got)
	}
}

func writeTestCert(t *testing.T, userSigner cryptossh.Signer, principals []string, certType uint32, after, before time.Time) string {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caSigner, err := cryptossh.NewSignerFromKey(caKey)
	if err != nil {
		t.Fatal(err)
	}
	cert := &cryptossh.Certificate{
		Key:             userSigner.PublicKey(),
		Serial:          1,
		CertType:        certType,
		KeyId:           "test",
		ValidPrincipals: principals,
		ValidAfter:      uint64(after.Unix()),
		ValidBefore:     uint64(before.Unix()),
	}
	if err := cert.SignCert(rand.Reader, caSigner); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "user-cert.pub")
	if err := os.WriteFile(path, cryptossh.MarshalAuthorizedKey(cert), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
