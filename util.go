package blockexplorer

import (
	"git.fleta.io/fleta/core/transaction"
)

func extractVin(vin []*transaction.TxIn) interface{} {
	ins := []struct {
		Height uint32
		Index  uint16
		N      uint16
	}{}
	for _, o := range vin {
		ins = append(ins, struct {
			Height uint32
			Index  uint16
			N      uint16
		}{
			Height: o.Height,
			Index:  o.Index,
			N:      o.N,
		})
	}
	return ins
}
