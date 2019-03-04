package blockexplorer

import (
	"io"

	"github.com/fletaio/common/util"
)

type currentChainInfo struct {
	Foumulators         int    `json:"foumulators"`
	Blocks              uint32 `json:"blocks"`
	Transactions        int    `json:"transactions"`
	currentTransactions int
}

// WriteTo is a serialization function
func (c *currentChainInfo) WriteTo(w io.Writer) (int64, error) {
	var wrote int64
	if n, err := util.WriteUint32(w, uint32(c.Foumulators)); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	if n, err := util.WriteUint32(w, c.Blocks); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	if n, err := util.WriteUint32(w, uint32(c.Transactions)); err != nil {
		return wrote, err
	} else {
		wrote += n
	}
	if n, err := util.WriteUint32(w, uint32(c.currentTransactions)); err != nil {
		return wrote, err
	} else {
		wrote += n
	}

	return wrote, nil
}

// ReadFrom is a deserialization function
func (c *currentChainInfo) ReadFrom(r io.Reader) (int64, error) {
	var read int64
	if v, n, err := util.ReadUint32(r); err != nil {
		return read, err
	} else {
		read += n
		c.Foumulators = int(v)
	}
	if v, n, err := util.ReadUint32(r); err != nil {
		return read, err
	} else {
		read += n
		c.Blocks = v
	}
	if v, n, err := util.ReadUint32(r); err != nil {
		return read, err
	} else {
		read += n
		c.Transactions = int(v)
	}
	if v, n, err := util.ReadUint32(r); err != nil {
		return read, err
	} else {
		read += n
		c.currentTransactions = int(v)
	}

	return read, nil
}
