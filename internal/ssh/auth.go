package ssh

import (
	"fmt"
	"os"

	cryptossh "golang.org/x/crypto/ssh"
)

// authMethods 按 Agent、密钥文件、密码的顺序组装静态认证方法。
// 配置了用户证书时，匹配的私钥改为证书签名，不再以裸公钥提供。
// 没有任何静态凭证时返回空切片，由拨号方决定是否再追加 keyboard-interactive。
func authMethods(ep Endpoint, agentSigners []cryptossh.Signer) ([]cryptossh.AuthMethod, error) {
	var methods []cryptossh.AuthMethod
	keySigner, err := keySignerFromEndpoint(ep)
	if err != nil {
		return nil, err
	}
	if ep.Certificate != "" {
		certSigner, fromAgent, err := userCertificateSigner(ep.Certificate, ep.User, keySigner, agentSigners)
		if err != nil {
			return nil, err
		}
		rest := agentSignersExceptCertKey(agentSigners, certSigner)
		if fromAgent {
			methods = append(methods, cryptossh.PublicKeys(certSigner))
			if len(rest) > 0 {
				methods = append(methods, cryptossh.PublicKeys(rest...))
			}
		} else {
			if len(rest) > 0 {
				methods = append(methods, cryptossh.PublicKeys(rest...))
			}
			methods = append(methods, cryptossh.PublicKeys(certSigner))
		}
	} else {
		if len(agentSigners) > 0 {
			methods = append(methods, cryptossh.PublicKeys(agentSigners...))
		}
		if keySigner != nil {
			methods = append(methods, cryptossh.PublicKeys(keySigner))
		}
	}
	if ep.Password != "" {
		methods = append(methods, cryptossh.Password(ep.Password))
	}
	return methods, nil
}

func keySignerFromEndpoint(ep Endpoint) (cryptossh.Signer, error) {
	if ep.KeyPEM != "" {
		return signerFromPEM(ep.KeyPEM)
	}
	if ep.Key != "" {
		return signerFromKeyFile(ep.Key, ep.KeyPassword)
	}
	return nil, nil
}

// finalizeAuthMethods 在静态方法之后追加 keyboard-interactive。都没有时返回错误。
func finalizeAuthMethods(methods []cryptossh.AuthMethod, keyboard cryptossh.AuthMethod) ([]cryptossh.AuthMethod, error) {
	if keyboard != nil {
		methods = append(methods, keyboard)
	}
	if len(methods) == 0 {
		return nil, ErrNoAuthMethods
	}
	return methods, nil
}

func agentUnavailableIsFatal(ep Endpoint, keyboardEnabled bool) bool {
	if ep.Password != "" || ep.Key != "" || ep.KeyPEM != "" {
		return false
	}
	// 用户证书没有本地私钥时只能靠 Agent，与是否启用 keyboard-interactive 无关。
	if ep.Certificate != "" {
		return true
	}
	return !keyboardEnabled
}

func signerFromPEM(pem string) (cryptossh.Signer, error) {
	signer, err := cryptossh.ParsePrivateKey([]byte(pem))
	if err != nil {
		return nil, fmt.Errorf("unable to parse private key: %w", err)
	}
	return signer, nil
}

func signerFromKeyFile(path, passphrase string) (cryptossh.Signer, error) {
	keyContent, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read private key: %w", err)
	}
	if passphrase == "" {
		return cryptossh.ParsePrivateKey(keyContent)
	}
	return cryptossh.ParsePrivateKeyWithPassphrase(keyContent, []byte(passphrase))
}
