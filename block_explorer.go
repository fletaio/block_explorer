package blockexplorer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"git.fleta.io/fleta/block_explorer/template"
	"git.fleta.io/fleta/common/util"
	"git.fleta.io/fleta/core/kernel"
	"github.com/dgraph-io/badger"
)

var (
	libPath string
)

func init() {
	var pwd string
	{
		pc := make([]uintptr, 10) // at least 1 entry needed
		runtime.Callers(1, pc)
		f := runtime.FuncForPC(pc[0])
		pwd, _ = f.FileLine(pc[0])

		path := strings.Split(pwd, "/")
		pwd = strings.Join(path[:len(path)-1], "/")
	}

	libPath = pwd
}

//Block explorer error list
var (
	ErrDbNotClear          = errors.New("Db is not clear")
	ErrNotEnoughParameter  = errors.New("Not enough parameter")
	ErrNotTransactionHash  = errors.New("This hash is not a transaction hash")
	ErrNotBlockHash        = errors.New("This hash is not a block hash")
	ErrInvalidHeightFormat = errors.New("Invalid height format")
)

// BlockExplorer struct
type BlockExplorer struct {
	Kernel                 *kernel.Kernel
	formulatorCountList    []countInfo
	transactionCountList   []countInfo
	CurrentChainInfo       currentChainInfo
	lastestTransactionList []txInfos

	db       *badger.DB
	Template *template.Template
}

type countInfo struct {
	Time  int64 `json:"time"`
	Count int   `json:"count"`
}

type currentChainInfo struct {
	Foumulators         int    `json:"foumulators"`
	Blocks              uint32 `json:"blocks"`
	Transactions        int    `json:"transactions"`
	currentTransactions int
	lastHeight          uint64
}

//NewBlockExplorer TODO
func NewBlockExplorer(dbPath string, Kernel *kernel.Kernel) (*BlockExplorer, error) {
	opts := badger.DefaultOptions
	opts.Dir = dbPath
	opts.ValueDir = dbPath
	opts.Truncate = true
	opts.SyncWrites = true
	lockfilePath := filepath.Join(opts.Dir, "LOCK")
	os.MkdirAll(dbPath, os.ModeDir)

	os.Remove(lockfilePath)

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	{
	again:
		if err := db.RunValueLogGC(0.7); err != nil {
		} else {
			goto again
		}
	}

	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for range ticker.C {
		again:
			if err := db.RunValueLogGC(0.7); err != nil {
			} else {
				goto again
			}
		}
	}()

	e := &BlockExplorer{
		Kernel:                 Kernel,
		formulatorCountList:    []countInfo{},
		transactionCountList:   []countInfo{},
		lastestTransactionList: []txInfos{},
		Template: template.NewTemplate(&template.TemplateConfig{
			TemplatePath: libPath + "/html/pages/",
			LayoutPath:   libPath + "/html/layout/",
		}),
		db: db,
	}

	go func(e *BlockExplorer) {
		for {
			time.Sleep(time.Second)

			e.updateChainInfoCount()

			e.formulatorCountList = appendListLimit(e.formulatorCountList, e.CurrentChainInfo.Foumulators, 200)
			e.transactionCountList = appendListLimit(e.transactionCountList, e.CurrentChainInfo.currentTransactions, 200)
		}
	}(e)

	return e, nil
}

var blockHeghtBytes = []byte("blockHeght")

func (e *BlockExplorer) LastestTransactionLen() int {
	return len(e.lastestTransactionList)
}
func (e *BlockExplorer) updateChainInfoCount() error {
	currHeight := e.Kernel.Provider().Height()

	e.CurrentChainInfo.currentTransactions = 0
	e.CurrentChainInfo.Foumulators = e.Kernel.CandidateCount()

	var height uint32
	if err := e.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(blockHeghtBytes)
		if err != nil {
			if err != badger.ErrKeyNotFound {
				return err
			}
			height = 0
		} else {
			value, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			height = util.BytesToUint32(value)
		}

		return nil
	}); err != nil {
		return ErrDbNotClear
	}

	if err := e.db.Update(func(txn *badger.Txn) error {
		for e.CurrentChainInfo.Blocks > height {
			height++
			err := e.updateHashs(txn, height)
			if err != nil {
				return err
			}
		}
		txn.Set(blockHeghtBytes, util.Uint32ToBytes(height))
		return nil
	}); err != nil {
		return err
	}

	newTxs := []txInfos{}
	for i := int(currHeight); i > int(e.CurrentChainInfo.Blocks) && i >= 0; i-- {
		height := uint32(i)
		b, err := e.Kernel.Block(height)
		if err != nil {
			continue
		}
		e.CurrentChainInfo.currentTransactions += len(b.Body.Transactions)

		txs := b.Body.Transactions
		for _, tx := range txs {
			name, _ := e.Kernel.Transactor().NameByType(tx.Type())
			newTxs = append(newTxs, txInfos{
				TxHash:    tx.Hash().String(),
				BlockHash: b.Header.Hash().String(),
				ChainID:   b.Header.ChainCoord.String(),
				Time:      tx.Timestamp(),
				TxType:    name,
			})
		}

		if err := e.db.Update(func(txn *badger.Txn) error {
			//start block hash update
			err = e.updateHashs(txn, height)
			if err != nil {
				return err
			}
			//end block hash update
			return nil
		}); err != nil {
			return err
		}

	}

	e.lastestTransactionList = append(newTxs, e.lastestTransactionList...)
	if err := e.db.Update(func(txn *badger.Txn) error {
		txn.Set(blockHeghtBytes, util.Uint32ToBytes(currHeight))
		return nil
	}); err != nil {
		return err
	}

	if len(e.lastestTransactionList) > 500 {
		e.lastestTransactionList = e.lastestTransactionList[len(e.lastestTransactionList)-500 : len(e.lastestTransactionList)]
	}

	e.CurrentChainInfo.Transactions += e.CurrentChainInfo.currentTransactions
	e.CurrentChainInfo.Blocks = currHeight

	return nil
}

func (e *BlockExplorer) updateHashs(txn *badger.Txn, height uint32) error {
	b, err := e.Kernel.Block(height)
	if err != nil {
		return err
	}
	value := util.Uint32ToBytes(height)

	h := b.Header.Hash().String()
	if err := txn.Set([]byte(h), value); err != nil {
		return err
	}

	txs := b.Body.Transactions
	for i, tx := range txs {
		h := tx.Hash().String()
		v := append(value, util.Uint32ToBytes(uint32(i))...)
		if err := txn.Set([]byte(h), v); err != nil {
			return err
		}
	}
	return nil
}

func appendListLimit(ci []countInfo, count int, limit int) []countInfo {
	if len(ci) >= limit {
		ci = ci[len(ci)-limit+1 : len(ci)]
	}
	ci = append(ci, countInfo{
		Time:  time.Now().UnixNano(),
		Count: count,
	})
	return ci
}

// StartExplorer is start web server
func (e *BlockExplorer) StartExplorer() {

	e.Template.AddController("", NewExplorerController(e.db, e))

	http.HandleFunc("/data/", e.dataHandler)
	http.HandleFunc("/", e.pageHandler)

	panic(http.ListenAndServe(":8088", nil))
}

//AddHandleFunc TODO
func (e *BlockExplorer) AddHandleFunc(perfix string, handle func(w http.ResponseWriter, r *http.Request)) {
	http.HandleFunc(perfix, handle)
}

func (e *BlockExplorer) printJSON(v interface{}, w http.ResponseWriter) {
	b, err := json.Marshal(&v)
	if err != nil {
		fmt.Println(err)
		return
	}
	w.Write(b)
}

// Handle HTTP request to either static file server or page server
func (e *BlockExplorer) pageHandler(w http.ResponseWriter, r *http.Request) {
	//remove first "/" character
	urlPath := r.URL.Path[1:]

	//if the path is include a dot direct to static file server
	if strings.Contains(urlPath, ".") {
		// define your static file directory
		staticFilePath := libPath + "/html/resource/"
		//other wise, let read a file path and display to client
		http.ServeFile(w, r, staticFilePath+urlPath)
	} else {
		data, err := e.Template.Route(r, urlPath)
		// data, err := e.routePath(r, urlPath)
		if err != nil {
			handleErrorCode(500, "Unable to retrieve file", w)
		} else {
			w.Write(data)
		}
	}
}

// Generate error page
func handleErrorCode(errorCode int, description string, w http.ResponseWriter) {
	w.WriteHeader(errorCode)                    // set HTTP status code (example 404, 500)
	w.Header().Set("Content-Type", "text/html") // clarify return type (MIME)

	data, _ := ioutil.ReadFile(libPath + "/html/errors/error-1.html")

	w.Write(data)
}

func (e *BlockExplorer) dataHandler(w http.ResponseWriter, r *http.Request) {
	order := r.URL.Path[len("/data/"):]

	switch order {
	case "formulators.data":
		e.printJSON(e.formulators(), w)
	case "transactions.data":
		e.printJSON(e.transactions(), w)
	case "currentChainInfo.data":
		e.printJSON(e.CurrentChainInfo, w)
	case "lastestBlocks.data":
		e.printJSON(e.lastestBlocks(), w)
	case "lastestTransactions.data":
		e.printJSON(e.lastestTransactions(), w)
	case "paginationBlocks.data":
		e.printJSON(e.paginationBlocks(r), w)
	case "paginationTxs.data":
		e.printJSON(e.paginationTxs(r), w)
	}
}
