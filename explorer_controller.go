package blockexplorer

import (
	"net/http"
	"strconv"

	"git.fleta.io/fleta/common/util"
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
	return map[string][]byte{
		"test": []byte("test byte"),
	}, nil
}
func (e *ExplorerController) blockDetail(r *http.Request) (map[string][]byte, error) {
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

	m, err := e.block.BlockDetailMap(height)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (e *ExplorerController) transactionDetail(r *http.Request) (map[string][]byte, error) {
	param := r.URL.Query()
	hash := param.Get("hash")
	// heightStr := param.Get("height")
	var v []byte
	if err := e.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(hash))
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

		if m, err := e.block.TxDetailMap(e.block.Kernel.Transactor(), blockHeight, txIndex); err == nil {
			return m, nil
		} else {
			return nil, err
		}

	} else {
		return nil, ErrNotTransactionHash
	}

}
