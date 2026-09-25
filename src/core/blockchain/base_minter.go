package blockchain

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

var baseNFTFeeABI, _ = abi.JSON(strings.NewReader(`[{"type":"function","name":"getL1FeeUpperBound","inputs":[{"type":"uint256"}],"outputs":[{"type":"uint256"}]}]`))

func (base *Base) nftExtraFee(ctx context.Context, unsigned []byte) (*big.Int, error) {
	oracle := common.HexToAddress("0x420000000000000000000000000000000000000F")
	data, err := baseNFTFeeABI.Pack("getL1FeeUpperBound", big.NewInt(int64(len(unsigned)+80)))
	if err != nil {
		return nil, err
	}
	fee, err := base.EthClient.CallContract(ctx, ethereum.CallMsg{To: &oracle, Data: data}, nil)
	if err != nil || len(fee) != 32 {
		return nil, errors.New("l1_fee_unavailable")
	}
	return new(big.Int).Mul(new(big.Int).SetBytes(fee), big.NewInt(2)), nil
}
