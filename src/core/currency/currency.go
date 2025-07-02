package currency

import (
	"errors"
	"strconv"
	"strings"
)

// CurrencyInfo holds information about supported currencies
type CurrencyInfo struct {
	Symbol          string
	Name            string
	SmallUnitName   string
	SmallUnitFactor int64
	SupportedChains []string
}

// SupportedCurrencies maps currency symbols to their info
var SupportedCurrencies = map[string]CurrencyInfo{
	"ETH": {
		Symbol:          "ETH",
		Name:            "Ethereum",
		SmallUnitName:   "wei",
		SmallUnitFactor: 1000000000000000000, // 1e18
		SupportedChains: []string{"base", "ethereum"},
	},
	// Stubbed currencies for forward compatibility
	"BTC": {
		Symbol:          "BTC",
		Name:            "Bitcoin",
		SmallUnitName:   "sats",
		SmallUnitFactor: 100000000, // 1e8
		SupportedChains: []string{"bitcoin"},
	},
	"ALGO": {
		Symbol:          "ALGO",
		Name:            "Algorand",
		SmallUnitName:   "microalgos",
		SmallUnitFactor: 1000000, // 1e6
		SupportedChains: []string{"algorand"},
	},
	"SOL": {
		Symbol:          "SOL",
		Name:            "Solana",
		SmallUnitName:   "lamports",
		SmallUnitFactor: 1000000000, // 1e9
		SupportedChains: []string{"solana"},
	},
}

// Currency represents a currency amount with both major and minor units
type Currency struct {
	Symbol     string
	Amount     string // Major unit (e.g., "1.5" ETH)
	SmallUnit  string // Minor unit (e.g., "1500000000000000000" wei)
	Blockchain string
}

// NewCurrency creates a new Currency instance
func NewCurrency(symbol, amount, smallUnit, blockchain string) (*Currency, error) {
	info, exists := SupportedCurrencies[strings.ToUpper(symbol)]
	if !exists {
		return nil, errors.New("unsupported currency: " + symbol)
	}

	// Validate blockchain support (only for implemented currencies)
	if symbol == "ETH" {
		validChain := false
		for _, chain := range info.SupportedChains {
			if strings.ToLower(blockchain) == chain {
				validChain = true
				break
			}
		}
		if !validChain {
			return nil, errors.New("currency " + symbol + " not supported on blockchain " + blockchain)
		}
	}

	return &Currency{
		Symbol:     strings.ToUpper(symbol),
		Amount:     amount,
		SmallUnit:  smallUnit,
		Blockchain: strings.ToLower(blockchain),
	}, nil
}

// ConvertToSmallUnit converts major unit to small unit
func (c *Currency) ConvertToSmallUnit() (string, error) {
	if c.SmallUnit != "" {
		return c.SmallUnit, nil
	}

	info, exists := SupportedCurrencies[c.Symbol]
	if !exists {
		return "", errors.New("unsupported currency: " + c.Symbol)
	}

	amount, err := strconv.ParseFloat(c.Amount, 64)
	if err != nil {
		return "", errors.New("invalid amount: " + c.Amount)
	}

	smallUnitAmount := int64(amount * float64(info.SmallUnitFactor))
	return strconv.FormatInt(smallUnitAmount, 10), nil
}

// ConvertToMajorUnit converts small unit to major unit
func (c *Currency) ConvertToMajorUnit() (string, error) {
	if c.Amount != "" {
		return c.Amount, nil
	}

	info, exists := SupportedCurrencies[c.Symbol]
	if !exists {
		return "", errors.New("unsupported currency: " + c.Symbol)
	}

	smallUnitAmount, err := strconv.ParseInt(c.SmallUnit, 10, 64)
	if err != nil {
		return "", errors.New("invalid small unit amount: " + c.SmallUnit)
	}

	majorAmount := float64(smallUnitAmount) / float64(info.SmallUnitFactor)
	return strconv.FormatFloat(majorAmount, 'f', -1, 64), nil
}

// GetSmallUnitName returns the name of the small unit (e.g., "wei", "sats")
func (c *Currency) GetSmallUnitName() string {
	info, exists := SupportedCurrencies[c.Symbol]
	if !exists {
		return "units"
	}
	return info.SmallUnitName
}

// IsSupported checks if a currency is supported
func IsSupported(symbol string) bool {
	_, exists := SupportedCurrencies[strings.ToUpper(symbol)]
	return exists
}

// IsImplemented checks if a currency is fully implemented (not just stubbed)
func IsImplemented(symbol string) bool {
	return strings.ToUpper(symbol) == "ETH"
}

// ValidateForBlockchain checks if currency is supported on the given blockchain
func ValidateForBlockchain(symbol, blockchain string) error {
	info, exists := SupportedCurrencies[strings.ToUpper(symbol)]
	if !exists {
		return errors.New("unsupported currency: " + symbol)
	}

	// Only validate for implemented currencies
	if IsImplemented(symbol) {
		for _, chain := range info.SupportedChains {
			if strings.ToLower(blockchain) == chain {
				return nil
			}
		}
		return errors.New("currency " + symbol + " not supported on blockchain " + blockchain)
	}

	return nil // Allow stubbed currencies for forward compatibility
}

// FormatDisplay returns a human-readable format of the currency
func (c *Currency) FormatDisplay() string {
	if c.Amount != "" {
		return c.Amount + " " + c.Symbol
	}

	majorUnit, err := c.ConvertToMajorUnit()
	if err != nil {
		return c.SmallUnit + " " + c.GetSmallUnitName()
	}

	return majorUnit + " " + c.Symbol
}
