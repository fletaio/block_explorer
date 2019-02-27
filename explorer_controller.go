package blockexplorer

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"git.fleta.io/fleta/common/hash"

	"git.fleta.io/fleta/common/util"
	"git.fleta.io/fleta/core/data"
	"github.com/dgraph-io/badger"
)

type ExplorerController struct {
	db    *badger.DB
	block *BlockExplorer
}

func NewExplorerController(db *badger.DB, block *BlockExplorer) *ExplorerController {
	return &ExplorerController{
		db:    db,
		block: block,
	}
}

func (e *ExplorerController) Blocks(r *http.Request) (map[string][]byte, error) {
	data := e.block.blocks(0, e.block.Kernel.Provider().Height())
	j, _ := json.Marshal(data)
	return map[string][]byte{
		"blockData": j,
	}, nil
}
func (e *ExplorerController) Transactions(r *http.Request) (map[string][]byte, error) {
	data := e.block.txs(0, 10)
	j, _ := json.Marshal(data)
	return map[string][]byte{
		"txsData":  j,
		"txLength": []byte(strconv.Itoa(e.block.LastestTransactionLen())),
	}, nil
}
func (e *ExplorerController) BlockDetail(r *http.Request) (map[string][]byte, error) {
	param := r.URL.Query()
	// hash := param.Get("hash")
	heightStr := param.Get("height")
	var height uint32
	if heightStr == "" {
		hash := param.Get("hash")
		if hash == "" {
			return nil, ErrNotEnoughParameter
		}

		if err := e.db.View(func(txn *badger.Txn) error {
			item, err := txn.Get([]byte(hash))
			if err != nil {
				return err
			}
			v, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			if len(v) != 4 {
				return ErrNotBlockHash
			}
			height = util.BytesToUint32(v)
			return nil
		}); err != nil {
			return nil, err
		}

	} else {
		heightInt, err := strconv.Atoi(heightStr)
		if err != nil {
			return nil, ErrInvalidHeightFormat
		}
		height = uint32(heightInt)
	}

	m, err := e.block.blockDetailMap(height)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (e *ExplorerController) TransactionDetail(r *http.Request) (map[string][]byte, error) {
	param := r.URL.Query()
	hashStr := param.Get("hash")
	h, err := hash.ParseHex(hashStr)
	if err != nil {
		return nil, err
	}
	var v []byte
	if err := e.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(h[:])
		if err != nil {
			return err
		}
		v, err = item.ValueCopy(nil)
		return nil
	}); err != nil {
		return nil, err
	}

	if len(v) == 8 {
		blockHeight := util.BytesToUint32(v[0:4])
		txIndex := util.BytesToUint32(v[4:8])

		if m, err := e.block.txDetailMap(e.block.Kernel.Transactor(), blockHeight, txIndex); err == nil {
			return m, nil
		} else {
			return nil, err
		}

	} else {
		return nil, ErrNotTransactionHash
	}

}

func (e *BlockExplorer) txDetailMap(tran *data.Transactor, height uint32, txIndex uint32) (map[string][]byte, error) {
	m := map[string]interface{}{}

	b, err := e.Kernel.Block(height)
	if err != nil {
		return nil, err
	}
	t := b.Body.Transactions[int(txIndex)]

	cd, err := e.Kernel.Provider().Data(height)
	if err != nil {
		return nil, err
	}

	name, err := tran.NameByType(t.Type())
	if err != nil {
		m["err"] = "현재 지원하지 않는 transaction 입니다."
	}
	m["Type"] = name

	m["Block Hash"] = cd.Header.Hash().String()

	tm := time.Unix(int64(cd.Header.Timestamp()/uint64(time.Second)), 0)
	m["Block Timestamp"] = tm.Format("2006-01-02 15:04:05")
	m["Tx Hash"] = t.Hash().String()
	tm = time.Unix(int64(t.Timestamp()/uint64(time.Second)), 0)
	m["Tx TimeStamp"] = tm.Format("2006-01-02 15:04:05")
	m["Chain"] = t.ChainCoord().String()

	bs, err := json.Marshal(&m)
	if err != nil {
		return nil, err
	}

	txbs, err := t.MarshalJSON()
	if err != nil {
		return nil, err
	}
	bs = append(bs[:len(bs)-1], byte(','))
	bs = append(bs, txbs[1:]...)
	return map[string][]byte{"TxInfo": bs}, nil
}

func (e *BlockExplorer) blockDetailMap(height uint32) (map[string][]byte, error) {
	cd, err := e.Kernel.Provider().Data(height)
	if err != nil {
		return nil, err
	}
	b, err := e.Kernel.Block(height)
	if err != nil {
		return nil, err
	}

	tm := time.Unix(int64(cd.Header.Timestamp()/uint64(time.Second)), 0)
	m := map[string]interface{}{}
	m["Hash"] = cd.Header.Hash().String()
	m["ChainCoord"] = b.Header.ChainCoord.String()
	m["Height"] = strconv.Itoa(int(cd.Header.Height()))
	m["Version"] = strconv.Itoa(int(cd.Header.Version()))
	m["HashPrevBlock"] = cd.Header.PrevHash().String()
	m["HashLevelRoot"] = b.Header.LevelRootHash.String()
	m["Timestamp"] = tm.Format("2006-01-02 15:04:05")
	m["FormulationAddress"] = b.Header.Formulator.String()
	m["TimeoutCount"] = strconv.Itoa(int(b.Header.TimeoutCount))
	m["Transaction Count"] = strconv.Itoa(len(b.Body.Transactions))

	txs := []string{}
	for _, t := range b.Body.Transactions {
		txs = append(txs, t.Hash().String())
	}
	m["Transactions"] = txs
	bs, err := json.Marshal(&m)
	return map[string][]byte{"TxInfo": bs}, nil
}
