package topup_test

import (
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/maticnetwork/heimdall/x/topup/test_helper"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/testutil/testdata"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/maticnetwork/heimdall/app"
	hmCommon "github.com/maticnetwork/heimdall/common"
	"github.com/maticnetwork/heimdall/helper/mocks"
	hmTypes "github.com/maticnetwork/heimdall/types"
	hmCommonTypes "github.com/maticnetwork/heimdall/types/common"
	"github.com/maticnetwork/heimdall/types/simulation"
	chainTypes "github.com/maticnetwork/heimdall/x/chainmanager/types"
	"github.com/maticnetwork/heimdall/x/topup"
	"github.com/maticnetwork/heimdall/x/topup/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// HandlerTestSuite integrate test suite context object
type HandlerTestSuite struct {
	suite.Suite

	app            *app.HeimdallApp
	ctx            sdk.Context
	cliCtx         client.Context
	handler        sdk.Handler
	contractCaller mocks.IContractCaller
	chainParams    chainTypes.Params
}

func (suite *HandlerTestSuite) SetupTest() {
	suite.app, suite.ctx, suite.cliCtx = test_helper.CreateTestApp(false)

	suite.contractCaller = mocks.IContractCaller{}
	suite.handler = topup.NewHandler(suite.app.TopupKeeper, &suite.contractCaller)
	suite.chainParams = suite.app.ChainKeeper.GetParams(suite.ctx)
}

// TestHandlerTestSuite
func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

func (suite *HandlerTestSuite) TestHandleMsgUnknown() {
	t, _, ctx := suite.T(), suite.app, suite.ctx

	result, err := suite.handler(ctx, nil)
	require.NotNil(t, err)
	require.Nil(t, result)
}

func (suite *HandlerTestSuite) TestHandleMsgTopup() {
	t, initApp, ctx := suite.T(), suite.app, suite.ctx
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)

	txHash := hmCommonTypes.BytesToHeimdallHash([]byte("topup hash"))
	logIndex := r1.Uint64()
	blockNumber := r1.Uint64()

	_, _, addr := testdata.KeyTestPubAddr()
	fee := sdk.NewInt(100000000000000000)
	generatedAddress, _ := sdk.AccAddressFromHex(addr.String())

	t.Run("Success", func(t *testing.T) {
		msg := types.NewMsgTopup(
			generatedAddress,
			generatedAddress,
			fee,
			txHash,
			logIndex,
			blockNumber,
		)

		// handler
		result, err := suite.handler(ctx, &msg)
		require.Nil(t, err)
		require.NotNil(t, result)
	})

	t.Run("OlderTx", func(t *testing.T) {
		msg := types.NewMsgTopup(
			generatedAddress,
			generatedAddress,
			fee,
			txHash,
			logIndex,
			blockNumber,
		)

		// sequence id
		blockNumber := new(big.Int).SetUint64(msg.BlockNumber)
		sequence := new(big.Int).Mul(blockNumber, big.NewInt(hmTypes.DefaultLogIndexUnit))
		sequence.Add(sequence, new(big.Int).SetUint64(msg.LogIndex))

		// set sequence
		initApp.TopupKeeper.SetTopupSequence(ctx, sequence.String())

		// handler
		result, err := suite.handler(ctx, &msg)
		//check if the error code is the same as CodeOldTx
		require.Error(t, err)
		require.Equal(t, hmCommon.ErrOldTx, err)
		require.Nil(t, result)
	})
}

func (suite *HandlerTestSuite) TestHandleMsgWithdrawFee() {
	t, initApp, ctx := suite.T(), suite.app, suite.ctx

	t.Run("FullAmount", func(t *testing.T) {
		_, _, addr := testdata.KeyTestPubAddr()
		tAddr, err := sdk.AccAddressFromHex(addr.String())
		require.Nil(t, err)
		msg := types.NewMsgWithdrawFee(
			tAddr,
			sdk.NewInt(0),
		)

		// execute handler
		result, err := suite.handler(ctx, &msg)
		require.Nil(t, result)
		require.Error(t, err)
		require.Equal(t, types.ErrNoBalanceToWithdraw, err)

		// set coins
		coins := simulation.RandomFeeCoins()
		acc1 := initApp.AccountKeeper.NewAccountWithAddress(ctx, tAddr)
		initApp.AccountKeeper.SetAccount(ctx, acc1)
		err = initApp.BankKeeper.SetBalances(ctx, tAddr, coins)
		require.Nil(t, err)

		// check if coins > 0
		require.True(t, initApp.BankKeeper.GetAllBalances(ctx, tAddr).AmountOf(hmTypes.FeeToken).GT(sdk.NewInt(0)))

		// execute handler
		result, err = suite.handler(ctx, &msg)
		require.Nil(t, err)
		require.NotNil(t, result)
		require.Greater(t, len(result.Events), 0)

		// check if account has zero
		require.True(t, initApp.BankKeeper.GetAllBalances(ctx, tAddr).AmountOf(hmTypes.FeeToken).IsZero())
	})

	t.Run("PartialAmount", func(t *testing.T) {
		_, _, addr := testdata.KeyTestPubAddr()
		tAddr, err := sdk.AccAddressFromHex(addr.String())
		require.Nil(t, err)
		// set coins
		coins := simulation.RandomFeeCoins()
		acc1 := initApp.AccountKeeper.NewAccountWithAddress(ctx, tAddr)
		initApp.AccountKeeper.SetAccount(ctx, acc1)
		err = initApp.BankKeeper.SetBalances(ctx, tAddr, coins)
		require.Nil(t, err)

		// check if coins > 0
		require.True(t, initApp.BankKeeper.GetAllBalances(ctx, tAddr).AmountOf(hmTypes.FeeToken).GT(sdk.NewInt(0)))

		m, _ := sdk.NewIntFromString("2")
		coins = coins.Sub(sdk.Coins{sdk.Coin{Denom: hmTypes.FeeToken, Amount: m}})
		msg := types.NewMsgWithdrawFee(
			tAddr,
			coins.AmountOf(hmTypes.FeeToken),
		)

		// execute handler
		result, err := suite.handler(ctx, &msg)
		require.Nil(t, err)
		require.NotNil(t, result)
		require.Greater(t, len(result.Events), 0)

		// check if account has 1 tok
		require.True(t, initApp.BankKeeper.GetAllBalances(ctx, tAddr).AmountOf(hmTypes.FeeToken).Equal(m))
	})

	t.Run("NotEnoughAmount", func(t *testing.T) {
		_, _, addr := testdata.KeyTestPubAddr()
		tAddr, err := sdk.AccAddressFromHex(addr.String())
		require.Nil(t, err)
		// set coins
		coins := simulation.RandomFeeCoins()
		acc1 := initApp.AccountKeeper.NewAccountWithAddress(ctx, tAddr)
		initApp.AccountKeeper.SetAccount(ctx, acc1)
		err = initApp.BankKeeper.SetBalances(ctx, tAddr, coins)
		require.Nil(t, err)

		m, _ := sdk.NewIntFromString("1")
		coins = coins.Add(sdk.Coin{Denom: hmTypes.FeeToken, Amount: m})
		msg := types.NewMsgWithdrawFee(
			tAddr,
			coins.AmountOf(hmTypes.FeeToken),
		)

		result, err := suite.handler(ctx, &msg)
		require.Error(t, err)
		require.Nil(t, result)
	})
}
