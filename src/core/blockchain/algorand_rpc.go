package blockchain

import (
	"YourPlace/src/core/security"
	"context"
	"encoding/base32"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (algo *Algorand) HandleWalletRPC(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	if algo == nil || algo.algodClient == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/rpc/algorand")
	var response any
	var err error
	switch {
	case r.Method == http.MethodGet && path == "/v2/transactions/params":
		params, paramsErr := algo.algodClient.SuggestedParams().Do(ctx)
		err = paramsErr
		response = map[string]any{"consensus-version": params.ConsensusVersion, "fee": params.Fee, "genesis-hash": params.GenesisHash, "genesis-id": params.GenesisID, "last-round": params.FirstRoundValid, "min-fee": params.MinFee}
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/v2/accounts/"):
		address := strings.TrimPrefix(path, "/v2/accounts/")
		if !security.IsValidAlgoAddress(address) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		response, err = algo.algodClient.AccountInformation(address).Do(ctx)
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/v2/assets/"):
		asset, parseErr := strconv.ParseUint(strings.TrimPrefix(path, "/v2/assets/"), 10, 64)
		if parseErr != nil || asset == 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		response, err = algo.algodClient.GetAssetByID(asset).Do(ctx)
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/v2/transactions/pending/"):
		hash := strings.TrimPrefix(path, "/v2/transactions/pending/")
		decoded, parseErr := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(hash)
		if parseErr != nil || len(decoded) != 32 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		response, _, err = algo.algodClient.PendingTransactionInformation(hash).Do(ctx)
	case r.Method == http.MethodGet && path == "/v2/status":
		response, err = algo.algodClient.Status().Do(ctx)
	case r.Method == http.MethodPost && path == "/v2/transactions":
		body, readErr := io.ReadAll(http.MaxBytesReader(w, r.Body, 65536))
		if readErr != nil || len(body) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var hash string
		hash, err = algo.algodClient.SendRawTransaction(body).Do(ctx)
		response = map[string]string{"txId": hash}
	default:
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err != nil {
		code := http.StatusBadGateway
		if errors.Is(err, context.DeadlineExceeded) {
			code = http.StatusGatewayTimeout
		}
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Algorand request unavailable"})
		return
	}
	_ = json.NewEncoder(w).Encode(response)
}
