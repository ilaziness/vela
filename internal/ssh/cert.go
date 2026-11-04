package ssh

import (
	"bytes"
	"io"
	"os"
	"time"

	cryptossh "golang.org/x/crypto/ssh"

	"github.com/ilaziness/vela/internal/usererr"
)

// userCertificateSigner 读取 OpenSSH 用户证书，并用匹配的私钥组成证书签名者。
// fromAgent 为 true 时，匹配的是 SSH Agent 里的密钥。
func userCertificateSigner(path, user string, keySigner cryptossh.Signer, agentSigners []cryptossh.Signer) (cryptossh.Signer, bool, error) {
	cert, err := parseUserCertificate(path)
	if err != nil {
		return nil, false, err
	}
	if err := certificateAllowsUser(cert, user); err != nil {
		return nil, false, err
	}
	if keySigner != nil && sameSignerKey(keySigner, cert.Key) {
		signer, err := cryptossh.NewCertSigner(cert, signerForCertKey(keySigner, cert.Key))
		if err != nil {
			return nil, false, usererr.Wrap(err, "ssh.cert_key_mismatch", "Certificate does not match the private key: %s")
		}
		return signer, false, nil
	}
	for _, agent := range agentSigners {
		if !sameSignerKey(agent, cert.Key) {
			continue
		}
		signer, err := cryptossh.NewCertSigner(cert, signerForCertKey(agent, cert.Key))
		if err != nil {
			return nil, false, usererr.Wrap(err, "ssh.cert_key_mismatch", "Certificate does not match the private key: %s")
		}
		return signer, true, nil
	}
	return nil, false, usererr.Coded("ssh.cert_no_matching_key", "Certificate does not match any available private key")
}

func parseUserCertificate(path string) (*cryptossh.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, usererr.Wrap(err, "ssh.cert_read_failed", "Failed to read certificate: %s")
	}
	pub, _, _, _, err := cryptossh.ParseAuthorizedKey(data)
	if err != nil {
		return nil, usererr.Wrap(err, "ssh.cert_parse_failed", "Failed to parse certificate: %s")
	}
	cert, ok := pub.(*cryptossh.Certificate)
	if !ok || cert.CertType != cryptossh.UserCert {
		return nil, usererr.Coded("ssh.cert_not_user", "Not an OpenSSH user certificate")
	}
	now := uint64(time.Now().Unix())
	if now < cert.ValidAfter {
		return nil, usererr.Coded("ssh.cert_not_yet_valid", "Certificate is not yet valid")
	}
	if now >= cert.ValidBefore {
		return nil, usererr.Coded("ssh.cert_expired", "Certificate has expired")
	}
	return cert, nil
}

func certificateAllowsUser(cert *cryptossh.Certificate, user string) error {
	if len(cert.ValidPrincipals) == 0 {
		return nil
	}
	for _, principal := range cert.ValidPrincipals {
		if principal == user {
			return nil
		}
	}
	return usererr.Coded("ssh.cert_user_mismatch", "Certificate is not valid for user %s", user)
}

func samePublicKey(a, b cryptossh.PublicKey) bool {
	if a == nil || b == nil {
		return false
	}
	return bytes.Equal(a.Marshal(), b.Marshal())
}

// sameSignerKey 比较签名者对应的裸公钥。Agent 里已加载证书时，公钥是证书本身。
func sameSignerKey(signer cryptossh.Signer, key cryptossh.PublicKey) bool {
	if signer == nil {
		return false
	}
	pub := signer.PublicKey()
	if cert, ok := pub.(*cryptossh.Certificate); ok && cert != nil {
		pub = cert.Key
	}
	return samePublicKey(pub, key)
}

// signerForCertKey 让 NewCertSigner 看到证书里的裸公钥。
// Agent 签名者对外公布的可能已是证书，直接传入会被拒绝。
func signerForCertKey(signer cryptossh.Signer, raw cryptossh.PublicKey) cryptossh.Signer {
	if signer == nil || samePublicKey(signer.PublicKey(), raw) {
		return signer
	}
	if alg, ok := signer.(cryptossh.AlgorithmSigner); ok {
		return algorithmKeySigner{Signer: signer, algorithm: alg, pub: raw}
	}
	return publicKeySigner{Signer: signer, pub: raw}
}

type publicKeySigner struct {
	cryptossh.Signer
	pub cryptossh.PublicKey
}

func (s publicKeySigner) PublicKey() cryptossh.PublicKey { return s.pub }

type algorithmKeySigner struct {
	cryptossh.Signer
	algorithm cryptossh.AlgorithmSigner
	pub       cryptossh.PublicKey
}

func (s algorithmKeySigner) PublicKey() cryptossh.PublicKey { return s.pub }

func (s algorithmKeySigner) SignWithAlgorithm(rand io.Reader, data []byte, algorithm string) (*cryptossh.Signature, error) {
	return s.algorithm.SignWithAlgorithm(rand, data, algorithm)
}

// agentSignersExceptCertKey 去掉与证书对应的那把密钥，避免再以裸公钥提供。
func agentSignersExceptCertKey(agents []cryptossh.Signer, certSigner cryptossh.Signer) []cryptossh.Signer {
	cert, ok := certSigner.PublicKey().(*cryptossh.Certificate)
	if !ok || cert.Key == nil {
		return agents
	}
	rest := make([]cryptossh.Signer, 0, len(agents))
	for _, agent := range agents {
		if sameSignerKey(agent, cert.Key) {
			continue
		}
		rest = append(rest, agent)
	}
	return rest
}
