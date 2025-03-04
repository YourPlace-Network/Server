package services

import (
	"YourPlace/src/core"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
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
