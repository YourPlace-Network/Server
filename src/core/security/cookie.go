package security

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type SecureCookie struct {
	Address    string `json:"address" required:"true"` // wallet address
	Blockchain string `json:"blockchain" required:"true"`
	Expiration int64  `json:"expiration" required:"true"` // unix timestamp
	Nonce      string `json:"nonce" required:"true"`
	Timestamp  int64  `json:"timestamp" required:"true"` // unix timestamp (cookie last seen)
	UUID       string `json:"uuid" required:"true"`
}

func CreateAuthCookie(address, blockchain string, cryptoSeed []byte, database *db.Database) *http.Cookie {
	expirationHours := 30 * 24 * time.Hour // cookie expires in 30 days
	now := time.Now()
	expirationTimestamp := now.Add(expirationHours).Unix()
	nonce := Nonce(32)
	uuid := UUID()
	cookieValue := &SecureCookie{
		Address:    address,
		Blockchain: blockchain,
		Expiration: expirationTimestamp,
		Nonce:      nonce,
		Timestamp:  now.Unix(),
		UUID:       uuid,
	}
	jsonCookieValue, err := json.Marshal(cookieValue)
	if err != nil {
		core.LogError("Could not encode cookie value")
		return nil
	}
	cryptoSeedHash := HashBytes(cryptoSeed)
	encryptedCookieValue, err := EncryptString(string(cryptoSeedHash), string(jsonCookieValue))
	if err != nil {
		core.LogError("Could not encrypt cookie value")
		return nil
	}
	cookie := &http.Cookie{
		Name:     "yp_auth",
		Value:    encryptedCookieValue,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   true,
		Expires:  time.Now().Add(expirationHours),
		MaxAge:   int(expirationHours.Seconds()),
	}
	database.AuthUpdateNonce(nonce, "valid")
	return cookie
}
func GetCookieValue(cookie *http.Cookie, cryptoSeed []byte, value string, database *db.Database) (string, error) {
	if ValidateCookie(cookie, cryptoSeed, database) == false {
		return "", errors.New("invalid Cookie")
	}
	cryptoSeedHash := HashBytes(cryptoSeed)
	decryptedCookieValue, err := DecryptString(string(cryptoSeedHash), cookie.Value) // decrypt cookie value
	if err != nil {
		return "", errors.New("could not decrypt cookie value")
	}
	var cookieValue SecureCookie
	err = json.Unmarshal([]byte(decryptedCookieValue), &cookieValue) // json parse cookie value
	if err != nil {
		return "", errors.New("could not parse cookie value")
	}
	value = strings.ToLower(value)
	switch value {
	case "address":
		return cookieValue.Address, nil
	case "blockchain":
		return cookieValue.Blockchain, nil
	case "expiration":
		return strconv.FormatInt(cookieValue.Expiration, 10), nil
	case "nonce":
		return cookieValue.Nonce, nil
	case "timestamp":
		return strconv.FormatInt(cookieValue.Timestamp, 10), nil
	case "uuid":
		return cookieValue.UUID, nil
	default:
		return "", errors.New("invalid Cookie Value")
	}
}
func ValidateCookie(cookie *http.Cookie, cryptoSeed []byte, database *db.Database) bool {
	err := cookie.Valid()
	if err != nil {
		return false
	}
	cryptoSeedHash := HashBytes(cryptoSeed)
	decryptedCookieJSON, err := DecryptString(string(cryptoSeedHash), cookie.Value)
	if err != nil {
		return false
	}
	var cookieObj SecureCookie
	err = json.Unmarshal([]byte(decryptedCookieJSON), &cookieObj)
	if err != nil {
		return false
	}
	if database.AuthGetCookieStatus(cookieObj.UUID) == "expired" {
		return false
	}
	if database.AuthGetNonceStatus(cookieObj.Nonce) != "valid" {
		return false
	}
	now := time.Now().Unix()
	if now < cookieObj.Expiration {
		return true
	} else {
		return false
	}
}
func InvalidateCookie(cookie *http.Cookie, cryptoSeed []byte, database *db.Database) {
	uuid, err := GetCookieValue(cookie, cryptoSeed, "uuid", database)
	if err != nil {
		return
	}
	database.AuthExpireCookie(uuid)
}
func IncrementCookie(c *gin.Context, cryptoSeed []byte, database *db.Database) error { // rotate cookie to mitigate theft
	currentTimestamp := time.Now().Unix()
	cryptoSeedHash := HashBytes(cryptoSeed)
	cookie, err := c.Request.Cookie("yp_auth")
	if err != nil {
		return errors.New("could not get cookie during increment")
	}
	if !ValidateCookie(cookie, cryptoSeed, database) {
		return errors.New("invalid cookie during expiration")
	}
	decryptCookieJSON, err := DecryptString(string(cryptoSeedHash), cookie.Value)
	if err != nil {
		return errors.New("could not decrypt cookie value")
	}
	var cookieObj SecureCookie
	err = json.Unmarshal([]byte(decryptCookieJSON), &cookieObj)
	if err != nil {
		return errors.New("could not unmarshal cookie value")
	}
	timeDiffSeconds := currentTimestamp - cookieObj.Timestamp
	if timeDiffSeconds < 300 { // if the cookie is younger than 5 minutes
		return nil
	} else { // Revoke the old cookie and return a new one (cookie theft defense)
		database.AuthDeleteNonce(cookieObj.Nonce)
		http.SetCookie(c.Writer, CreateAuthCookie(cookieObj.Address, cookieObj.Blockchain, cryptoSeed, database))
		return nil
	}
}
