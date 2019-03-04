package blockexplorer

import (
	"encoding/hex"
	"log"
	"os"
	"testing"

	"github.com/fletaio/common"
	"github.com/fletaio/core/account"
	"github.com/fletaio/core/amount"
	"github.com/fletaio/core/consensus"
	"github.com/fletaio/core/data"
	"github.com/fletaio/core/kernel"
	"github.com/fletaio/core/key"
	"github.com/fletaio/core/transaction"
	"github.com/fletaio/extension/account_def"

	_ "github.com/fletaio/extension/account_tx"
	_ "github.com/fletaio/extension/utxo_tx"
	_ "github.com/fletaio/solidity"
)

func TestBlockExplorer_startExplorer(t *testing.T) {

	basePath := "./test/"
	os.RemoveAll(basePath)

	kn := newKernel(basePath)

	e, err := NewBlockExplorer(basePath, kn)
	if err != nil {
		panic(err)
	}
	e.StartExplorer()
}

func newKernel(basePath string) *kernel.Kernel {
	GenCoord := common.NewCoordinate(0, 0)
	act := data.NewAccounter(GenCoord)
	tran := data.NewTransactor(GenCoord)
	if err := initChainComponent(act, tran); err != nil {
		panic(err)
	}

	ks, err := kernel.NewStore(basePath+"kernel", 1, act, tran) //kernelstore.NewStore("./kernel")
	if err != nil {
		panic(err)
	}

	GenesisContextData := data.NewContextData(data.NewEmptyLoader(ks.ChainCoord(), ks.Accounter(), ks.Transactor()), nil)
	if err := initGenesisContextData(ks, GenesisContextData); err != nil {
		panic(err)
	}
	obstrs := []string{
		"cca49818f6c49cf57b6c420cdcd98fcae08850f56d2ff5b8d287fddc7f9ede08",
		"39f1a02bed5eff3f6247bb25564cdaef20d410d77ef7fc2c0181b1d5b31ce877",
		"2b97bc8f21215b7ed085cbbaa2ea020ded95463deef6cbf31bb1eadf826d4694",
		"3b43d728deaa62d7c8790636bdabbe7148a6641e291fd1f94b157673c0172425",
		"e6cf2724019000a3f703db92829ecbd646501c0fd6a5e97ad6774d4ad621f949",
	}

	obkeys := make([]key.Key, 0, len(obstrs))
	ObserverKeys := make([]string, 0, len(obstrs))
	for _, v := range obstrs {
		if bs, err := hex.DecodeString(v); err != nil {
			panic(err)
		} else if Key, err := key.NewMemoryKeyFromBytes(bs); err != nil {
			panic(err)
		} else {
			obkeys = append(obkeys, Key)
			ObserverKeys = append(ObserverKeys, common.NewPublicHash(Key.PublicKey()).String())
		}
	}

	cfg := &kernel.Config{
		ChainCoord:   GenCoord,
		ObserverKeys: ObserverKeys,
	}
	rd := &mockRewarder{}

	kn, err := kernel.NewKernel(cfg, ks, rd, GenesisContextData)
	if err != nil {
		panic(err)
	}

	return kn
}

type mockRewarder struct {
}

// ProcessReward gives a reward to the block generator address
func (rd *mockRewarder) ProcessReward(addr common.Address, ctx *data.Context) error {
	balance, err := ctx.AccountBalance(addr)
	if err != nil {
		return err
	}
	balance.AddBalance(ctx.ChainCoord(), amount.NewCoinAmount(1, 0))
	return nil
}

func initChainComponent(act *data.Accounter, tran *data.Transactor) error {
	// transaction_type transaction types
	const (
		// FLETA Transactions
		TransferTransctionType              = transaction.Type(10)
		WithdrawTransctionType              = transaction.Type(18)
		BurnTransctionType                  = transaction.Type(19)
		CreateAccountTransctionType         = transaction.Type(20)
		CreateMultiSigAccountTransctionType = transaction.Type(21)
		// UTXO Transactions
		AssignTransctionType      = transaction.Type(30)
		DepositTransctionType     = transaction.Type(38)
		OpenAccountTransctionType = transaction.Type(41)
		// Formulation Transactions
		CreateFormulationTransctionType = transaction.Type(60)
		RevokeFormulationTransctionType = transaction.Type(61)
		// Solidity Transactions
		SolidityCreateContractType = transaction.Type(70)
		SolidityCallContractType   = transaction.Type(71)
	)

	// account_type account types
	const (
		// FLTEA Accounts
		SingleAccountType   = account.Type(10)
		MultiSigAccountType = account.Type(11)
		LockedAccountType   = account.Type(19)
		// Formulation Accounts
		FormulationAccountType = account.Type(60)
		// Solidity Accounts
		SolidityAccount = account.Type(70)
	)

	type txFee struct {
		Type transaction.Type
		Fee  *amount.Amount
	}

	TxFeeTable := map[string]*txFee{
		"fleta.CreateAccount":         &txFee{CreateAccountTransctionType, amount.COIN.MulC(10)},
		"fleta.CreateMultiSigAccount": &txFee{CreateMultiSigAccountTransctionType, amount.COIN.MulC(10)},
		"fleta.Transfer":              &txFee{TransferTransctionType, amount.COIN.DivC(10)},
		"fleta.Withdraw":              &txFee{WithdrawTransctionType, amount.COIN.DivC(10)},
		"fleta.Burn":                  &txFee{BurnTransctionType, amount.COIN.DivC(10)},
		"fleta.Assign":                &txFee{AssignTransctionType, amount.COIN.DivC(2)},
		"fleta.Deposit":               &txFee{DepositTransctionType, amount.COIN.DivC(2)},
		"fleta.OpenAccount":           &txFee{OpenAccountTransctionType, amount.COIN.MulC(10)},
		"consensus.CreateFormulation": &txFee{CreateFormulationTransctionType, amount.COIN.MulC(50000)},
		"consensus.RevokeFormulation": &txFee{RevokeFormulationTransctionType, amount.COIN.DivC(10)},
		"solidity.CreateContract":     &txFee{SolidityCreateContractType, amount.COIN.MulC(10)},
		"solidity.CallContract":       &txFee{SolidityCallContractType, amount.COIN.DivC(10)},
	}
	for name, item := range TxFeeTable {
		if err := tran.RegisterType(name, item.Type, item.Fee); err != nil {
			log.Println(name, item, err)
			return err
		}
	}

	AccTable := map[string]account.Type{
		"fleta.SingleAccount":          SingleAccountType,
		"fleta.MultiSigAccount":        MultiSigAccountType,
		"fleta.LockedAccount":          LockedAccountType,
		"consensus.FormulationAccount": FormulationAccountType,
		"solidity.ContractAccount":     SolidityAccount,
	}
	for name, t := range AccTable {
		if err := act.RegisterType(name, t); err != nil {
			log.Println(name, t, err)
			return err
		}
	}
	return nil
}

func initGenesisContextData(loader data.Loader, ctd *data.ContextData) error {
	acg := &accCoordGenerator{}
	addSingleAccount(loader, ctd, common.MustParsePublicHash("3Zmc4bGPP7TuMYxZZdUhA9kVjukdsE2S8Xpbj4Laovv"), common.NewAddress(acg.Generate(), loader.ChainCoord(), 0))
	addFormulator(loader, ctd, common.MustParsePublicHash("2xASBuEWw6LcQGjYxeGZH9w1DUsEDt7fvUh8p3auxyN"), common.NewAddress(acg.Generate(), loader.ChainCoord(), 0))
	//addFormulator(loader, ctd, common.MustParsePublicHash("2VdGunZe8yZNm2mErqQqrFx2B7Mb4SBRPWviWnapahw"), common.NewAddress(acg.Generate(), st.ChainCoord(), 0))
	/*
		addFormulator(loader, ctd, common.MustParsePublicHash("3eiovnNMgNCSkmxqwkjAabRTbNkkauMVk167Pgqon2Q"), common.NewAddress(acg.Generate(), st.ChainCoord(), 0))
		addFormulator(loader, ctd, common.MustParsePublicHash("cNXbd7o43DkX48DaEy7hzuR6iy6DBxMAqNWmhxJLyA"), common.NewAddress(acg.Generate(), st.ChainCoord(), 0))
		addFormulator(loader, ctd, common.MustParsePublicHash("3S7zbNCsAkHJns4Z3GP6RoQKcffHDxv8fPbk1tKD2Bb"), common.NewAddress(acg.Generate(), st.ChainCoord(), 0))
		addFormulator(loader, ctd, common.MustParsePublicHash("39q6QQ9pfiP1yEAceCu11p5cmVhG8mHMiVayCD3UEa5"), common.NewAddress(acg.Generate(), st.ChainCoord(), 0))
		addFormulator(loader, ctd, common.MustParsePublicHash("37pB69UiK7GX1sYcawoUq8c8yXS9WWbQnkmzoQjUmZB"), common.NewAddress(acg.Generate(), st.ChainCoord(), 0))
		addFormulator(loader, ctd, common.MustParsePublicHash("2r9mQmdfvK62ELWezK8tUvDztettkUkEGrvMWUXL7D"), common.NewAddress(acg.Generate(), st.ChainCoord(), 0))
		addFormulator(loader, ctd, common.MustParsePublicHash("2CQBhmtferf2qWDjqSnEE3f1ECimj4Lck2CxndgqEVq"), common.NewAddress(acg.Generate(), st.ChainCoord(), 0))
		addFormulator(loader, ctd, common.MustParsePublicHash("4D5m6ssnsf3NxJmqKg7PpwoyG2PdMNPAuQjpB8ZKjDo"), common.NewAddress(acg.Generate(), st.ChainCoord(), 0))
	*/
	return nil
}

func addSingleAccount(loader data.Loader, ctd *data.ContextData, KeyHash common.PublicHash, addr common.Address) {
	a, err := loader.Accounter().NewByTypeName("fleta.SingleAccount")
	if err != nil {
		panic(err)
	}
	acc := a.(*account_def.SingleAccount)
	acc.Address_ = addr
	acc.KeyHash = KeyHash
	ctd.CreatedAccountHash[acc.Address_] = acc
	balance := account.NewBalance()
	balance.AddBalance(loader.ChainCoord(), amount.NewCoinAmount(10000000000, 0))
	ctd.AccountBalanceHash[acc.Address_] = balance
}

func addFormulator(loader data.Loader, ctd *data.ContextData, KeyHash common.PublicHash, addr common.Address) {
	a, err := loader.Accounter().NewByTypeName("consensus.FormulationAccount")
	if err != nil {
		panic(err)
	}
	acc := a.(*consensus.FormulationAccount)
	acc.Address_ = addr
	acc.KeyHash = KeyHash
	ctd.CreatedAccountHash[acc.Address_] = acc
}

type accCoordGenerator struct {
	idx uint16
}

func (acg *accCoordGenerator) Generate() *common.Coordinate {
	coord := common.NewCoordinate(0, acg.idx)
	acg.idx++
	return coord
}
