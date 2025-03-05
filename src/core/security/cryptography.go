package security

import (
	"YourPlace/src/core"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"github.com/google/uuid"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/crypto/sha3"
	"io"
	"log"
	"math/big"
	_rand "math/rand"
	"net"
	"os"
	"strings"
	"time"
)

func EncryptString(password, plainText string) (string, error) {
	cipherTextBytes, err := EncryptBytes(password, []byte(plainText))
	if err != nil {
		return "", err
	}
	return Base64EncodeBytes(cipherTextBytes), nil
}
func EncryptBytes(password string, plainText []byte) ([]byte, error) {
	if len(password) < 32 {
		return nil, errors.New("encryption password too short")
	}
	key := make([]byte, 32) // create 32 byte key from password
	copy(key, password)
	nonce := make([]byte, 12) // generate nonce
	_, err := io.ReadFull(rand.Reader, nonce)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key) // create AES-256 cipher block
	if err != nil {
		return nil, err
	}
	aesGCM, err := cipher.NewGCM(block) // create GCM cipher
	if err != nil {
		return nil, err
	}
	cipherText := aesGCM.Seal(nil, nonce, plainText, nil) // encrypt data
	cipherText = append(cipherText, nonce...)             // append nonce to cipherText
	return cipherText, nil
}
func DecryptString(password, cipherText string) (string, error) {
	decodedCipherText := Base64Decode(cipherText)
	plainText, err := DecryptBytes(password, []byte(decodedCipherText))
	if err != nil {
		return "", err
	}
	return string(plainText), nil
}
func DecryptBytes(password string, cipherText []byte) ([]byte, error) {
	nonceSize := 12
	if len(password) < 32 {
		return nil, core.LogErrorReturn("decryption password too short")
	}
	if len(cipherText) <= nonceSize {
		return nil, core.LogErrorReturn("cipher text too short")
	}
	nonce := cipherText[len(cipherText)-nonceSize:]     // extract nonce
	cipherText = cipherText[:len(cipherText)-nonceSize] // extract cipherText
	key := make([]byte, 32)                             // create 32 byte key from password
	copy(key, password)
	block, err := aes.NewCipher(key) // create AES-256 cipher block
	if err != nil {
		return nil, err
	}
	aesGCM, err := cipher.NewGCM(block) // create GCM cipher
	if err != nil {
		return nil, err
	}
	plainText, err := aesGCM.Open(nil, nonce, cipherText, nil) // decrypt data
	if err != nil {
		return nil, err
	}
	return plainText, nil
}
func DeriveKey(password string) ([]byte, []byte, error) {
	salt := make([]byte, 12)
	_, err := io.ReadFull(rand.Reader, salt)
	if err != nil {
		return nil, nil, err
	}
	key := pbkdf2.Key([]byte(password), salt, 1000000, 32, sha3.New512)
	return key, salt, nil
}
func DeriveKeySalt(password string, salt []byte) []byte {
	return pbkdf2.Key([]byte(password), salt, 1000000, 32, sha3.New512)
}
func GenerateSalt(saltSize int) []byte {
	var salt = make([]byte, saltSize)
	_, err := rand.Read(salt[:])
	if err != nil {
		panic(err)
	}
	return salt
}
func Hash(value string) string {
	return string(HashBytes([]byte(value)))
}
func HashBytes(value []byte) []byte {
	digest := sha3.New512()
	digest.Write(value)
	return digest.Sum(nil)
}
func HashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", core.LogErrorReturn("Could not open file to hash: " + err.Error())
	}
	defer file.Close()
	hash := sha3.New512()
	_, err = io.Copy(hash, file)
	if err != nil {
		return "", core.LogErrorReturn("Could not copy file data into hash object: " + err.Error())
	}
	hashBytes := hash.Sum(nil)
	hashString := hex.EncodeToString(hashBytes)
	return hashString, nil
}
func HMAC(key, data []byte) string {
	mac := hmac.New(sha3.New512, key)
	mac.Write(data)
	hmacBytes := mac.Sum(nil)
	hmacString := hex.EncodeToString(hmacBytes)
	return hmacString
}
func ValidateHMAC(key, data []byte, hmacString string) bool {
	expectedHMAC := HMAC(key, data)
	if hmacString != expectedHMAC {
		return false
	} else {
		return true
	}
}
func Nonce(length int) string {
	result := ""
	for {
		if len(result) >= length {
			return result
		}
		number, err := rand.Int(rand.Reader, big.NewInt(int64(127)))
		if err != nil {
			panic(err)
		}
		n := number.Int64()
		if n > 32 && n < 127 {
			result += string(n)
		}
	}
}
func UUID() string {
	return uuid.New().String()
}
func TLS() tls.Config {
	cert, err := tls.LoadX509KeyPair("certs/publickey.cer", "certs/private.key")
	if err != nil {
		log.Fatalf("TLS error. Exiting: %s\n", err)
	}
	tlsConfig := tls.Config{Certificates: []tls.Certificate{cert}, InsecureSkipVerify: false}
	tlsConfig.Time = func() time.Time { return time.Now() }
	tlsConfig.Rand = rand.Reader
	return tlsConfig
}
func RandomBytes(length int) []byte {
	byteArray := make([]byte, length)
	_, err := io.ReadFull(rand.Reader, byteArray)
	if err != nil {
		core.LogFatal("Random Byte Generation Failed")
		return nil
	}
	return byteArray
}
func RandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	bytes := make([]byte, length)
	bytes = RandomBytes(length * 5)
	for i, v := range bytes {
		bytes[i] = charset[v%byte(len(charset))]
	}
	return string(bytes)
}
func RandomUint(min, max int) uint64 {
	return uint64(min + _rand.Intn(max-min))
}
func Base64EncodeBytes(value []byte) string {
	return base64.StdEncoding.EncodeToString(value)
}
func Base64Encode(value string) string {
	return Base64EncodeBytes([]byte(value))
}
func Base64Decode(value string) string {
	return string(Base64DecodeBytes(value))
}
func Base64DecodeBytes(value string) []byte {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil
	}
	return decoded
}
func CheckChecksumFile(checkFile string, binaryFileStr string) bool {
	checksumFile, err := os.Open(checkFile)
	if err != nil {
		core.LogError("Could not open checksum file: " + err.Error())
		return false
	}
	defer checksumFile.Close()
	checksumData, err := io.ReadAll(checksumFile)
	if err != nil {
		core.LogError("Could not read checksum file: " + err.Error())
		return false
	}
	fields := strings.Fields(string(checksumData))
	if len(fields) < 1 {
		core.LogError("Checksum file is empty")
		return false
	}
	expectedChecksum := fields[0]
	installerFile, err := os.Open(binaryFileStr)
	if err != nil {
		core.LogError("Could not open file to check checksums: " + err.Error())
		return false
	}
	defer installerFile.Close()
	hash := sha256.New()
	_, err = io.Copy(hash, installerFile)
	if err != nil {
		core.LogError("Could not hash file: " + err.Error())
		return false
	}
	computedChecksum := hex.EncodeToString(hash.Sum(nil))
	if computedChecksum != expectedChecksum {
		core.LogError("Checksum mismatch")
		return false
	}
	return true
}
func PGPVerifySignature(signatureFilePath string, binaryFilePath string) bool {
	binaryFile, err := os.Open(binaryFilePath)
	if err != nil {
		core.LogError("Could not open Lighthouse binary: " + err.Error())
		return false
	}
	defer binaryFile.Close()
	signatureFile, err := os.Open(signatureFilePath)
	if err != nil {
		core.LogError("Could not open Lighthouse PGP signature: " + err.Error())
		return false
	}
	defer signatureFile.Close()
	keyring, err := openpgp.ReadArmoredKeyRing(signatureFile)
	if err != nil {
		core.LogError("Could not read PGP keyring from Lighthouse signature: " + err.Error())
		return false
	}
	_, err = openpgp.CheckDetachedSignature(keyring, binaryFile, signatureFile)
	if err != nil {
		return false
	} else {
		return true
	}
}
func GenerateCACert(path string) (string, string, error) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader) // Generate CA private key
	if err != nil {
		return "", "", err
	}
	caTemplate := x509.Certificate{ // Create CA certificate template
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"YourPlace Inc"},
			CommonName:   "YourPlace Server CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caCertDER, err := x509.CreateCertificate(rand.Reader, &caTemplate, &caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return "", "", err
	}
	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader) // Generate YourPlace server private key
	if err != nil {
		return "", "", err
	}
	serverTemplate := x509.Certificate{ // Create YourPlace server certificate
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization: []string{"YourPlace Inc"},
			CommonName:   "localhost",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}
	serverCertDER, err := x509.CreateCertificate(rand.Reader, &serverTemplate, &caTemplate, &serverKey.PublicKey, caKey)
	if err != nil {
		return "", "", err
	}
	err = WritePEMFile(path+"ca.crt", "CERTIFICATE", caCertDER) // Write CA certificate
	if err != nil {
		return "", "", err
	}
	caKeyDER, err := x509.MarshalPKCS8PrivateKey(caKey)
	if err != nil {
		return "", "", err
	}
	err = WritePEMFile(path+"ca.key", "PRIVATE KEY", caKeyDER) // Write CA private key
	if err != nil {
		return "", "", err
	}
	err = WritePEMFile(path+"server.crt", "CERTIFICATE", serverCertDER) // Write YourPlace server certificate
	if err != nil {
		return "", "", err
	}
	serverKeyDER, err := x509.MarshalPKCS8PrivateKey(serverKey)
	if err != nil {
		return "", "", err
	}
	err = WritePEMFile(path+"server.key", "PRIVATE KEY", serverKeyDER) // Write YourPlace server private key
	if err != nil {
		return "", "", err
	}
	return path + "server.crt", path + "server.key", nil
}
func WritePEMFile(path string, blockType string, bytes []byte) error {
	pemFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer pemFile.Close()
	err = pem.Encode(pemFile, &pem.Block{Type: blockType, Bytes: bytes})
	if err != nil {
		return err
	}
	return nil
}
func GenerateTLSCert(path string) (string, string, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return "", "", err
	}
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"YourPlace Inc"},
			CommonName:   "localhost",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // Valid for 1 year
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return "", "", err
	}
	privateKeyFile, err := os.Create(path + "server.key")
	if err != nil {
		return "", "", err
	}
	defer privateKeyFile.Close()
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return "", "", err
	}
	err = pem.Encode(privateKeyFile, &pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyBytes})
	if err != nil {
		return "", "", err
	}
	certFile, err := os.Create(path + "server.crt")
	if err != nil {
		return "", "", err
	}
	defer certFile.Close()
	if err = pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return "", "", err
	}
	return path + "server.crt", path + "server.key", nil
}
func Keccak256Hash(data []byte) []byte {
	hash := sha3.NewLegacyKeccak256()
	hash.Write(data)
	return hash.Sum(nil)
}
func HexCharToByte(c byte) byte {
	if c >= '0' && c <= '9' {
		return c - '0'
	}
	return c - 'a' + 10
}
