package shadowsocks

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/intxff/rdcross/component/proxy"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
)

// ErrCipherNotSupported occurs when a cipher is not supported (likely because of security concerns).
var ErrCipherNotSupported = errors.New("cipher not supported")

const (
	aeadAes128Gcm        = "AEAD_AES_128_GCM"
	aeadAes256Gcm        = "AEAD_AES_256_GCM"
	aeadChacha20Poly1305 = "AEAD_CHACHA20_POLY1305"
)

type cipherCreateFunc func(salt []byte) (cipher.AEAD, error)

// List of AEAD ciphers: key size in bytes and constructor
var aeadList = map[string]struct {
	KeySize int
	New     func([]byte) (cipherCreateFunc, error)
}{
	aeadAes128Gcm:        {16, AESGCM},
	aeadAes256Gcm:        {32, AESGCM},
	aeadChacha20Poly1305: {32, Chacha20Poly1305},
}

type aeadCipher struct {
	psk     []byte
	keySize int
	creator cipherCreateFunc
}

func newAeadCipher(password, key []byte, cipher string) (*aeadCipher, []byte, error) {
	creator, psk, err := cipherCreator(password, key, cipher)
	if err != nil {
		return nil, psk, err
	}
	return &aeadCipher{psk: psk, creator: creator, keySize: len(psk)}, psk, nil
}

// implement Cipher.Encrypter
func (a *aeadCipher) Encrypter(extra ...any) (proxy.Encrypter, error) {
	salt := extra[0].([]byte)
	subkey := make([]byte, a.keySize)
	hkdfSHA1(a.psk, salt, []byte("ss-subkey"), subkey)
	encrypter, err := a.creator(subkey)
	if err != nil {
		return nil, err
	}
	return &aeadEncryter{nonce: make([]byte, encrypter.NonceSize()), AEAD: encrypter}, nil
}

// implement Cipher.Decrypter
func (a *aeadCipher) Decrypter(extra ...any) (proxy.Decrypter, error) {
	salt := extra[0].([]byte)
	subkey := make([]byte, a.keySize)
	hkdfSHA1(a.psk, salt, []byte("ss-subkey"), subkey)
	decrypter, err := a.creator(subkey)
	if err != nil {
		return nil, err
	}
	return &aeadDecryter{nonce: make([]byte, decrypter.NonceSize()), AEAD: decrypter}, nil
}

func hkdfSHA1(secret, salt, info, outkey []byte) {
	r := hkdf.New(sha1.New, secret, salt, info)
	if _, err := io.ReadFull(r, outkey); err != nil {
		panic(err) // should never happen
	}
}

// key-derivation function from original Shadowsocks
func kdf(password []byte, keyLen int) []byte {
	var b, prev []byte
	h := md5.New()
	for len(b) < keyLen {
		h.Write(prev)
		h.Write(password)
		b = h.Sum(b)
		prev = b[len(b)-h.Size():]
		h.Reset()
	}
	return b[:keyLen]
}

func cipherCreator(password, key []byte, cipher string) (cipherCreateFunc, []byte, error) {
	cipher = strings.ToUpper(cipher)

	switch cipher {
	case "CHACHA20-IETF-POLY1305":
		cipher = aeadChacha20Poly1305
	case "AES-128-GCM":
		cipher = aeadAes128Gcm
	case "AES-256-GCM":
		cipher = aeadAes256Gcm
	}

	if choice, ok := aeadList[cipher]; ok {
		if len(key) == 0 {
			key = kdf(password, choice.KeySize)
		}
		if len(key) != choice.KeySize {
			return nil, key, fmt.Errorf("key size error: need %v byte", choice.KeySize)
		}
		aead, err := choice.New(key)
		return aead, key, err
	}

	return nil, key, ErrCipherNotSupported
}

func aesGCM(key []byte) (cipher.AEAD, error) {
	blk, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(blk)
}

// AESGCM creates a new Cipher with a pre-shared key. len(psk) must be
// one of 16, 24, or 32 to select AES-128/196/256-GCM.
func AESGCM(psk []byte) (cipherCreateFunc, error) {
	switch l := len(psk); l {
	case 16, 24, 32: // AES 128/196/256
	default:
		return nil, aes.KeySizeError(l)
	}
	return aesGCM, nil
}

// Chacha20Poly1305 creates a new Cipher with a pre-shared key. len(psk)
// must be 32.
func Chacha20Poly1305(psk []byte) (cipherCreateFunc, error) {
	if len(psk) != chacha20poly1305.KeySize {
		return nil, fmt.Errorf("key size error: need %v byte", chacha20poly1305.KeySize)
	}
	return chacha20poly1305.New, nil
}

type aeadEncryter struct {
	nonce []byte
	cipher.AEAD
}

func (a *aeadEncryter) Encrypt(planetext []byte, extra ...any) []byte {
	defer increment(a.nonce)
	if len(extra) != 0 && extra[0].(string) == "packet" {
        return a.Seal(extra[1].([]byte)[:0], zeroNonce[:a.NonceSize()], planetext, nil)
	}
	return a.Seal(planetext[:0], a.nonce, planetext, nil)
}

type aeadDecryter struct {
	nonce []byte
	cipher.AEAD
}

func (a *aeadDecryter) Decrypt(ciphertext []byte, extra ...any) ([]byte, error) {
	defer increment(a.nonce)
	if len(extra) != 0 && extra[0].(string) == "packet" {
        return a.Open(extra[1].([]byte)[:0], zeroNonce[:a.NonceSize()], ciphertext, nil)
	}
	return a.Open(ciphertext[:0], a.nonce, ciphertext, nil)
}

// increment little-endian encoded unsigned integer b. Wrap around on overflow.
func increment(b []byte) {
	for i := range b {
		b[i]++
		if b[i] != 0 {
			return
		}
	}
}
