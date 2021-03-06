package blockexplorer

import (
	"strconv"
	"time"

	"github.com/fletaio/core/block"
)

func (e *BlockExplorer) transactions() []*countInfo {
	return e.transactionCountList
}
func (e *BlockExplorer) chainInfo() currentChainInfo {
	return e.CurrentChainInfo
}

type typePerBlock struct {
	BlockTime uint64 `json:"blockTime"`
	Symbol    string `json:"symbol"`
	TxCount   string `json:"txCount"`
	// Types     map[string]int `json:"types"`
}

type blockInfos struct {
	BlockHeight uint32   `json:"Block Height"`
	BlockHash   string   `json:"Block Hash"`
	Time        string   `json:"Time"`
	Status      string   `json:"Status"`
	Txs         string   `json:"Txs"`
	Formulator  string   `json:"Formulator"`
	Msg         string   `json:"Msg"`
	Signs       []string `json:"Signs"`
	BlockCount  uint32   `json:"BlockCount"`
}
type blockInfosCase struct {
	ITotalRecords        int          `json:"iTotalRecords"`
	ITotalDisplayRecords int          `json:"iTotalDisplayRecords"`
	SEcho                int          `json:"sEcho"`
	SColumns             string       `json:"sColumns"`
	AaData               []blockInfos `json:"aaData"`
}

func (e *BlockExplorer) lastestBlocks() (result blockInfosCase) {
	currHeight := e.Kernel.Provider().Height()

	result.AaData = []blockInfos{}

	for i := currHeight; i > 0 && i > currHeight-8; i-- {
		b, err := e.Kernel.Block(i)
		if err != nil {
			continue
		}

		b.Header.Hash().String()

		cd, err := e.Kernel.Provider().Data(i)
		if err != nil {
			continue
		}
		status := 1
		if b.Header.TimeoutCount > 0 {
			status = 2
		}

		bs := block.Signed{
			HeaderHash:         cd.Header.Hash(),
			GeneratorSignature: cd.Signatures[0],
		}
		Signs := []string{
			cd.Signatures[1].String(),
			cd.Signatures[2].String(),
			cd.Signatures[3].String(),
		}

		tm := time.Unix(int64(cd.Header.Timestamp()/uint64(time.Second)), 0)

		result.AaData = append(result.AaData, blockInfos{
			BlockHeight: i,
			BlockHash:   cd.Header.Hash().String(),
			Time:        tm.Format("2006-01-02 15:04:05"),
			Status:      strconv.Itoa(status),
			Txs:         strconv.Itoa(len(b.Body.Transactions)),
			Formulator:  b.Header.Formulator.String(),
			Msg:         bs.Hash().String(),
			Signs:       Signs,
			BlockCount:  e.GetBlockCount(b.Header.Formulator.String()),
		})
	}

	result.ITotalRecords = len(result.AaData)
	result.ITotalDisplayRecords = len(result.AaData)

	return
}

type txInfos struct {
	TxHash    string `json:"TxHash"`
	BlockHash string `json:"BlockHash"`
	ChainID   string `json:"ChainID"`
	Time      uint64 `json:"Time"`
	TxType    string `json:"TxType"`
}

func (e *BlockExplorer) lastestTransactions() []txInfos {
	if len(e.lastestTransactionList) < 8 {
		return e.lastestTransactionList[0:len(e.lastestTransactionList)]
	}
	return e.lastestTransactionList[0:8]
}

func (e *BlockExplorer) blocks(start int, currHeight uint32) []blockInfos {
	length := 10
	aaData := []blockInfos{}

	for i, j := currHeight-uint32(start), 0; i > 0 && j < length; i, j = i-1, j+1 {
		b, err := e.Kernel.Block(i)
		if err != nil {
			continue
		}
		cd, err := e.Kernel.Provider().Data(i)
		if err != nil {
			continue
		}
		status := 1
		if b.Header.TimeoutCount > 0 {
			status = 2
		}

		tm := time.Unix(int64(cd.Header.Timestamp()/uint64(time.Second)), 0)

		aaData = append(aaData, blockInfos{
			BlockHeight: i,
			BlockHash:   cd.Header.Hash().String(),
			Time:        tm.Format("2006-01-02 15:04:05"),
			Status:      strconv.Itoa(status),
			Txs:         strconv.Itoa(len(b.Body.Transactions)),
		})
	}

	return aaData
}

func (e *BlockExplorer) paginationBlocks(startStr string) (result blockInfosCase) {
	start, err := strconv.Atoi(startStr)
	if err != nil {
		return
	}
	currHeight := e.Kernel.Provider().Height()

	result.ITotalRecords = int(currHeight)
	result.ITotalDisplayRecords = int(currHeight)

	result.AaData = e.blocks(start, currHeight)

	return
}

type txInfosCase struct {
	ITotalRecords        int       `json:"iTotalRecords"`
	ITotalDisplayRecords int       `json:"iTotalDisplayRecords"`
	SEcho                int       `json:"sEcho"`
	SColumns             string    `json:"sColumns"`
	AaData               []txInfos `json:"aaData"`
}

func (e *BlockExplorer) txs(start int, length int) []txInfos {
	max := start + length
	if max > len(e.lastestTransactionList) {
		max = len(e.lastestTransactionList)
	}

	return e.lastestTransactionList[start:max]
}

func (e *BlockExplorer) paginationTxs(startStr string) (result txInfosCase) {
	start, err := strconv.Atoi(startStr)
	if err != nil {
		return
	}
	length := 10

	result.ITotalRecords = len(e.lastestTransactionList)
	result.ITotalDisplayRecords = len(e.lastestTransactionList)

	result.AaData = e.txs(start, length)

	return
}
