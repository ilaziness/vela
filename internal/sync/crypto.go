package sync

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
)

const (
	saltSize  = 16
	nonceSize = 12
	keySize   = 32

	// Match internal/secret Argon2id parameters.
	argonTime    = 3
	argonMemory  = 64 * 1024
	argonThreads = 4

	plainChunkSize = 32 * 1024
)

// DeriveKey derives an AES key from the master password and salt via Argon2id.
func DeriveKey(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, keySize)
}

func clearKey(key []byte) {
	subtle.ConstantTimeCopy(1, key, make([]byte, len(key)))
}

// EncryptStream encrypts in fixed plaintext frames of plainChunkSize (last frame may be shorter).
// Wire format: salt(16) + nonce(12) + sealed frames (each ciphertext + 16-byte GCM tag).
type EncryptStream struct {
	writer io.Writer
	cipher cipher.AEAD
	nonce  []byte
	buffer []byte
}

// NewEncryptStream creates an encrypting writer keyed by password.
func NewEncryptStream(writer io.Writer, password string) (*EncryptStream, error) {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	key := DeriveKey(password, salt)
	defer clearKey(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	if _, err := writer.Write(salt); err != nil {
		return nil, fmt.Errorf("failed to write salt: %w", err)
	}
	if _, err := writer.Write(nonce); err != nil {
		return nil, fmt.Errorf("failed to write nonce: %w", err)
	}

	return &EncryptStream{
		writer: writer,
		cipher: aead,
		nonce:  nonce,
		buffer: make([]byte, 0, plainChunkSize),
	}, nil
}

func (e *EncryptStream) Write(p []byte) (n int, err error) {
	e.buffer = append(e.buffer, p...)
	for len(e.buffer) >= plainChunkSize {
		if err := e.flushExact(plainChunkSize); err != nil {
			return len(p), err
		}
	}
	return len(p), nil
}

func (e *EncryptStream) Close() error {
	if len(e.buffer) == 0 {
		return nil
	}
	return e.flushExact(len(e.buffer))
}

func (e *EncryptStream) flushExact(size int) error {
	if size <= 0 || size > len(e.buffer) {
		return nil
	}
	chunk := e.buffer[:size]
	ciphertext := e.cipher.Seal(nil, e.nonce, chunk, nil)
	if _, err := e.writer.Write(ciphertext); err != nil {
		return err
	}
	incrementNonce(e.nonce)
	e.buffer = append(e.buffer[:0], e.buffer[size:]...)
	return nil
}

// DecryptStream decrypts fixed-size sealed frames produced by EncryptStream.
type DecryptStream struct {
	reader    io.Reader
	cipher    cipher.AEAD
	nonce     []byte
	cipherBuf []byte
	plainBuf  []byte
	password  string
	saltRead  bool
	readerEOF bool
}

// NewDecryptStream creates a decrypting reader keyed by password.
func NewDecryptStream(reader io.Reader, password string) (*DecryptStream, error) {
	return &DecryptStream{
		reader:    reader,
		nonce:     make([]byte, nonceSize),
		cipherBuf: make([]byte, 0, plainChunkSize+16),
		password:  password,
	}, nil
}

func (d *DecryptStream) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	if len(d.plainBuf) > 0 {
		n = copy(p, d.plainBuf)
		d.plainBuf = d.plainBuf[n:]
		return n, nil
	}

	if err := d.ensureCipher(); err != nil {
		return 0, err
	}

	frameSize := plainChunkSize + d.cipher.Overhead()
	for len(d.cipherBuf) < frameSize && !d.readerEOF {
		tmp := make([]byte, frameSize-len(d.cipherBuf))
		nr, rerr := d.reader.Read(tmp)
		if nr > 0 {
			d.cipherBuf = append(d.cipherBuf, tmp[:nr]...)
		}
		if rerr == io.EOF {
			d.readerEOF = true
			break
		}
		if rerr != nil {
			return 0, rerr
		}
	}

	if len(d.cipherBuf) >= frameSize {
		plaintext, openErr := d.cipher.Open(nil, d.nonce, d.cipherBuf[:frameSize], nil)
		if openErr != nil {
			return 0, fmt.Errorf("failed to decrypt frame: %w", openErr)
		}
		d.cipherBuf = append(d.cipherBuf[:0], d.cipherBuf[frameSize:]...)
		incrementNonce(d.nonce)
		n = copy(p, plaintext)
		if n < len(plaintext) {
			d.plainBuf = append(d.plainBuf[:0], plaintext[n:]...)
		}
		return n, nil
	}

	if len(d.cipherBuf) == 0 {
		return 0, io.EOF
	}
	if len(d.cipherBuf) <= d.cipher.Overhead() {
		return 0, fmt.Errorf("truncated ciphertext frame")
	}
	plaintext, openErr := d.cipher.Open(nil, d.nonce, d.cipherBuf, nil)
	if openErr != nil {
		return 0, fmt.Errorf("failed to decrypt final frame: %w", openErr)
	}
	d.cipherBuf = d.cipherBuf[:0]
	n = copy(p, plaintext)
	if n < len(plaintext) {
		d.plainBuf = append(d.plainBuf[:0], plaintext[n:]...)
		return n, nil
	}
	return n, io.EOF
}

func (d *DecryptStream) ensureCipher() error {
	if d.saltRead {
		return nil
	}
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(d.reader, salt); err != nil {
		return fmt.Errorf("failed to read salt: %w", err)
	}
	if _, err := io.ReadFull(d.reader, d.nonce); err != nil {
		return fmt.Errorf("failed to read nonce: %w", err)
	}

	key := DeriveKey(d.password, salt)
	defer clearKey(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}
	d.cipher = aead
	d.saltRead = true
	return nil
}

func incrementNonce(nonce []byte) {
	for i := len(nonce) - 1; i >= 0; i-- {
		nonce[i]++
		if nonce[i] != 0 {
			break
		}
	}
}
