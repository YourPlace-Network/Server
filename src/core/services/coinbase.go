package services

import (
	"YourPlace/src/core"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func CoinbaseGetPriceUSD(symbol string) (float64, error) {
	type PriceData struct {
		Data struct {
			Amount   string `json:"amount" required:"true"`
			Base     string `json:"base"`
			Currency string `json:"currency"`
		} `json:"data" required:"true"`
	}
	price := PriceData{}
	client := &http.Client{
		Timeout: time.Second * 10,
	}
	req, err := http.NewRequest(http.MethodGet, "https://api.coinbase.com/v2/prices/"+symbol+"-USD/spot", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return 0, core.LogErrorReturn("Got non-200 response when downloading " + symbol + " price from Coinbase")
	}
	err = json.Unmarshal(body, &price)
	if err != nil {
		return 0, err
	}
	number, err := strconv.ParseFloat(price.Data.Amount, 64)
	if err != nil {
		return 0, core.LogErrorReturn("Could not parse " + symbol + " float from Coinbase:\n\t" + err.Error())
	}
	return number, nil
}
func CoinbaseOnrampToken(address, blockchain, clientIP string) (string, error) {
	keyName := os.Getenv("CDP_ONRAMP_KEY_NAME")
	privateKeyStr := os.Getenv("CDP_ONRAMP_PRIVATE_KEY")
	if keyName == "" || privateKeyStr == "" {
		return "", fmt.Errorf("CDP onramp credentials not configured")
	}
	if clientIP == "" {
		return "", fmt.Errorf("clientIp is required for CDP onramp token request")
	}
	core.LogDebug("CDP onramp token request for " + address + " on " + blockchain + " from clientIp " + clientIP)
	privKey, err := parseCDPPrivateKey(privateKeyStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse CDP private key: %w", err)
	}
	now := time.Now().Unix()
	uri := "POST api.developer.coinbase.com/onramp/v1/token"
	jwt, err := buildCDPJWT(keyName, uri, now, privKey)
	if err != nil {
		return "", fmt.Errorf("failed to build JWT: %w", err)
	}
	payload := map[string]interface{}{
		"addresses": []map[string]interface{}{
			{
				"address":     address,
				"blockchains": []string{blockchain},
				"assets":      []string{"ETH"},
			},
		},
		"clientIp": clientIP,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodPost, "https://api.developer.coinbase.com/onramp/v1/token", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("onramp token request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		core.LogDebug("CDP onramp token error: " + string(respBody))
		return "", fmt.Errorf("onramp token API returned status %d", resp.StatusCode)
	}
	var result struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}
	if result.Token == "" {
		return "", fmt.Errorf("empty token in response")
	}
	return result.Token, nil
}
func buildCDPJWT(keyName, uri string, now int64, privKey ed25519.PrivateKey) (string, error) {
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return "", err
	}
	nonce := fmt.Sprintf("%x", nonceBytes)
	header := map[string]string{
		"alg":   "EdDSA",
		"kid":   keyName,
		"nonce": nonce,
		"typ":   "JWT",
	}
	claims := map[string]interface{}{
		"aud": "cdp_service",
		"exp": now + 120,
		"iss": "coinbase-cloud",
		"nbf": now,
		"sub": keyName,
		"uri": uri,
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := headerB64 + "." + claimsB64
	sig := ed25519.Sign(privKey, []byte(signingInput))
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)
	return signingInput + "." + sigB64, nil
}
func parseCDPPrivateKey(keyStr string) (ed25519.PrivateKey, error) {
	candidates := []string{keyStr}
	parts := strings.Split(keyStr, "/")
	if len(parts) > 1 {
		candidates = append(candidates, parts[len(parts)-1])
	}
	decodings := []func(string) ([]byte, error){
		base64.StdEncoding.DecodeString,
		base64.RawStdEncoding.DecodeString,
		base64.URLEncoding.DecodeString,
		base64.RawURLEncoding.DecodeString,
	}
	for _, candidate := range candidates {
		for _, decode := range decodings {
			decoded, err := decode(candidate)
			if err != nil {
				continue
			}
			core.LogDebug(fmt.Sprintf("CDP key decoded: %d bytes from %d char input", len(decoded), len(candidate)))
			if len(decoded) == ed25519.SeedSize {
				return ed25519.NewKeyFromSeed(decoded), nil
			}
			if len(decoded) == ed25519.PrivateKeySize {
				return ed25519.PrivateKey(decoded), nil
			}
		}
	}
	return nil, fmt.Errorf("failed to decode Ed25519 key from secret (no valid 32 or 64 byte key found)")
}
