package sync

import (
	"bytes"
	"io"
	"testing"
)

func TestDeriveKey(t *testing.T) {
	userKey := "test-user-key"
	salt := []byte("test-salt-123456")

	key1 := DeriveKey(userKey, salt)
	key2 := DeriveKey(userKey, salt)

	if len(key1) != keySize {
		t.Errorf("expected key size %d, got %d", keySize, len(key1))
	}

	// 相同的输入应该产生相同的密钥
	if !bytes.Equal(key1, key2) {
		t.Error("same input should produce same key")
	}

	// 不同的盐应该产生不同的密钥
	differentSalt := []byte("different-salt-12")
	key3 := DeriveKey(userKey, differentSalt)
	if bytes.Equal(key1, key3) {
		t.Error("different salt should produce different key")
	}
}

func TestEncryptDecryptStream(t *testing.T) {
	password := "test-encryption-key"
	originalData := []byte("Hello, World! This is a test message for encryption and decryption.")

	var encryptedBuffer bytes.Buffer
	encryptStream, err := NewEncryptStream(&encryptedBuffer, password)
	if err != nil {
		t.Fatalf("failed to create encrypt stream: %v", err)
	}

	_, err = encryptStream.Write(originalData)
	if err != nil {
		t.Fatalf("failed to write data: %v", err)
	}

	err = encryptStream.Close()
	if err != nil {
		t.Fatalf("failed to close encrypt stream: %v", err)
	}

	encryptedData := encryptedBuffer.Bytes()
	if len(encryptedData) == 0 {
		t.Fatal("encrypted data should not be empty")
	}

	decryptStream, err := NewDecryptStream(bytes.NewReader(encryptedData), password)
	if err != nil {
		t.Fatalf("failed to create decrypt stream: %v", err)
	}

	var decryptedBuffer bytes.Buffer
	_, err = decryptedBuffer.ReadFrom(decryptStream)
	if err != nil {
		t.Fatalf("failed to read decrypted data: %v", err)
	}

	decryptedData := decryptedBuffer.Bytes()
	if !bytes.Equal(originalData, decryptedData) {
		t.Errorf("decrypted data mismatch: expected %s, got %s", originalData, decryptedData)
	}
}

func TestEncryptDecryptMultiFrame(t *testing.T) {
	password := "multi-frame-key"
	originalData := bytes.Repeat([]byte("abcdefgh"), plainChunkSize) // > 1 frame

	var encryptedBuffer bytes.Buffer
	enc, err := NewEncryptStream(&encryptedBuffer, password)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if _, err := enc.Write(originalData); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := enc.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	dec, err := NewDecryptStream(bytes.NewReader(encryptedBuffer.Bytes()), password)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	got, err := io.ReadAll(dec)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(originalData, got) {
		t.Fatalf("mismatch: got %d bytes want %d", len(got), len(originalData))
	}
}

func TestClearKey(t *testing.T) {
	key := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	originalKey := make([]byte, len(key))
	copy(originalKey, key)

	clearKey(key)

	// 验证密钥已被清零
	for i, b := range key {
		if b != 0 {
			t.Errorf("key byte at index %d should be 0, got %d", i, b)
		}
	}
}

func TestIncrementNonce(t *testing.T) {
	nonce := []byte{0x00, 0x00, 0x00, 0x00}
	incrementNonce(nonce)

	expected := []byte{0x00, 0x00, 0x00, 0x01}
	if !bytes.Equal(nonce, expected) {
		t.Errorf("nonce increment failed: expected %v, got %v", expected, nonce)
	}

	// 测试溢出
	nonce = []byte{0xFF, 0xFF, 0xFF, 0xFF}
	incrementNonce(nonce)
	expected = []byte{0x00, 0x00, 0x00, 0x00}
	if !bytes.Equal(nonce, expected) {
		t.Errorf("nonce overflow failed: expected %v, got %v", expected, nonce)
	}
}
