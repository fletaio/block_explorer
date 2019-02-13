package blockexplorer

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"git.fleta.io/fleta/block_explorer/template"
	"git.fleta.io/fleta/common/util"
	"git.fleta.io/fleta/core/data"
	"git.fleta.io/fleta/core/kernel"
	"git.fleta.io/fleta/core/transaction"
	"git.fleta.io/fleta/extension/account_tx"
	"git.fleta.io/fleta/framework/log"
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
	db                     *badger.DB

	Template *template.Template

	BlockInfo blockInfo
}

type blockInfo struct {
	minSize  int
	avgSize  float32
	sizeAt95 int
	sizeAt99 int
	maxSize  int

	minApply  int64
	avgApply  float64
	applyAt95 int64
	applyAt99 int64
	maxApply  int64

	minSend  uint64
	avgSend  float64
	sendAt95 uint64
	sendAt99 uint64
	maxSend  uint64
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

	e.BlockInfo.minApply = math.MaxInt64
	e.BlockInfo.minSend = math.MaxUint64
	e.BlockInfo.minSize = math.MaxInt32

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

func (e *BlockExplorer) updateChainInfoCount() error {
	// e.CurrentChainInfo.Foumulators = 10

	currHeight := e.Kernel.Provider().Height()

	e.CurrentChainInfo.currentTransactions = 0

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
		//TODO db error
	}

	if err := e.db.Update(func(txn *badger.Txn) error {
		for e.CurrentChainInfo.Blocks > height {
			height++
			// b, err := e.Kernel.Block(height)
			// if err != nil {
			// 	if err == badger.ErrKeyNotFound {
			// 		return ErrDbNotClear
			// 	}
			// 	return err
			// }
			err := e.updateHashs(txn, height)
			if err != nil {
				return err
			}
			txn.Set(blockHeghtBytes, util.Uint32ToBytes(height))
		}
		return nil

	}); err != nil {
		return err
	}

	if err := e.db.Update(func(txn *badger.Txn) error {
		newTxs := []txInfos{}
		for i := int(currHeight); i > int(e.CurrentChainInfo.Blocks) && i >= 0; i-- {
			height := uint32(i)
			b, err := e.Kernel.Block(height)
			if err != nil {
				continue
			}
			e.CurrentChainInfo.currentTransactions += len(b.Body.Transactions)

			//chain info start
			d := Get(height)
			if d != nil {
				if d.SaveTime > 0 && e.BlockInfo.minApply > d.SaveTime {
					e.BlockInfo.minApply = d.SaveTime
				}
				if e.BlockInfo.maxApply < d.SaveTime {
					e.BlockInfo.maxApply = d.SaveTime
				}
				num := e.CurrentChainInfo.Blocks + (currHeight - height)
				if num > 0 {
					e.BlockInfo.avgApply = float64(((e.BlockInfo.avgApply * (float64(num) - float64(1))) + float64(d.SaveTime)) / float64(num))
				}
				e.BlockInfo.applyAt95 = GetSaveTime(95)
				e.BlockInfo.applyAt99 = GetSaveTime(99)

				if d.Spand > 0 && e.BlockInfo.minSend > d.Spand {
					e.BlockInfo.minSend = d.Spand
				}
				if e.BlockInfo.maxSend < d.Spand {
					e.BlockInfo.maxSend = d.Spand
				}
				if num > 0 {
					e.BlockInfo.avgSend = float64(((e.BlockInfo.avgSend * (float64(num) - float64(1))) + float64(d.Spand)) / float64(num))
				}
				e.BlockInfo.sendAt95 = GetSpand(95)
				e.BlockInfo.sendAt99 = GetSpand(99)
			}

			//start block hash update
			err = e.updateHashs(txn, height)
			if err != nil {
				return err
			}
			//end block hash update

		}

		e.lastestTransactionList = append(newTxs, e.lastestTransactionList...)
		txn.Set(blockHeghtBytes, util.Uint32ToBytes(currHeight))

		return nil
	}); err != nil {
		return err
	}

	if len(e.lastestTransactionList) > 500 {
		e.lastestTransactionList = e.lastestTransactionList[len(e.lastestTransactionList)-500 : len(e.lastestTransactionList)]
	}

	//chain info start
	c := e.CurrentChainInfo.currentTransactions / 2
	if c > 0 && e.BlockInfo.minSize > c {
		e.BlockInfo.minSize = c
	}
	if e.BlockInfo.maxSize < c {
		e.BlockInfo.maxSize = c
	}
	if c > 0 {
		e.BlockInfo.avgSize = float32(((e.BlockInfo.avgSize * (float32(currHeight) - float32(1))) + float32(c)) / float32(currHeight))
		SaveSize(c)
		e.BlockInfo.sizeAt95 = GetSize(95)
		e.BlockInfo.sizeAt99 = GetSize(99)
	}
	//chain info end

	e.CurrentChainInfo.Transactions += e.CurrentChainInfo.currentTransactions
	e.CurrentChainInfo.Blocks = currHeight

	log.Info("e.CurrentChainInfo.Blocks ", e.CurrentChainInfo.Blocks)

	return nil
}

func (e *BlockExplorer) updateHashs(txn *badger.Txn, height uint32) error {
	b, err := e.Kernel.Block(height)
	if err != nil {
		return err
	}
	h, err := e.Kernel.Provider().Hash(height)
	if err != nil {
		return err
	}
	value := util.Uint32ToBytes(height)

	if err := txn.Set(h[:], value); err != nil {
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

func typeToString(t transaction.Type) string {
	switch t {
	case transaction.Type(10):
		return "Transfer"
	case transaction.Type(18):
		return "Withdraw"
	case transaction.Type(19):
		return "Burn"
	case transaction.Type(20):
		return "CreateAccount"
	case transaction.Type(21):
		return "CreateMultiSigAccount"
	case transaction.Type(30):
		return "Assign"
	case transaction.Type(38):
		return "Deposit"
	case transaction.Type(41):
		return "OpenAccount"
	case transaction.Type(60):
		return "CreateFormulation"
	case transaction.Type(61):
		return "RevokeFormulation"
	case transaction.Type(70):
		return "SolidityCreateContract"
	case transaction.Type(71):
		return "SolidityCallContract"
	}

	return ""
}

func (e *BlockExplorer) startExplorer() {

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

func (e *BlockExplorer) TxDetailMap(tran *data.Transactor, height uint32, txIndex uint32) (map[string][]byte, error) {
	// "fleta.CreateAccount":           &txFee{CreateAccountTransctionType, amount.COIN.MulC(10)},
	// "fleta.Transfer":                &txFee{TransferTransctionType, amount.COIN.DivC(10)},
	// "fleta.Withdraw":                &txFee{WithdrawTransctionType, amount.COIN.DivC(10)},
	// "fleta.Burn":                    &txFee{BurnTransctionType, amount.COIN.DivC(10)},
	m := map[string][]byte{}

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
		m["err"] = []byte("현재 지원하지 않는 transaction 입니다.")
	}
	m["Type"] = []byte(name)

	h := cd.Header.Hash()
	m["Block Hash"] = []byte(h.String())

	tm := time.Unix(int64(cd.Header.Timestamp/uint64(time.Second)), 0)
	m["Block Timestamp"] = []byte(tm.Format("2006-01-02 15:04:05"))
	h = t.Hash()
	m["Tx Hash"] = []byte(h.String())
	tm = time.Unix(int64(t.Timestamp()/uint64(time.Second)), 0)
	m["Tx TimeStamp"] = []byte(tm.Format("2006-01-02 15:04:05"))
	m["Chain"] = []byte(t.ChainCoord().String())

	switch name {
	case "fleta.CreateAccount":
		tx := t.(*account_tx.CreateAccount)
		m["From"] = []byte(tx.From_.String())
		m["KeyHash"] = []byte(tx.KeyHash.String())
		m["Seq"] = []byte(fmt.Sprint(tx.Seq_))
	case "fleta.Transfer":
		tx := t.(*account_tx.Transfer)
		m["From"] = []byte(tx.From_.String())
		m["To"] = []byte(tx.To.String())
		m["Seq"] = []byte(fmt.Sprint(tx.Seq_))
		m["Amount"] = []byte(tx.Amount.String())

		dst := make([]byte, hex.EncodedLen(len(tx.Tag)))
		hex.Encode(dst, tx.Tag)
		m["Tag"] = []byte(fmt.Sprintf("%s", dst))
		m["TokenCoord"] = []byte(tx.TokenCoord.String())
	case "fleta.Withdraw":
		tx := t.(*account_tx.Withdraw)
		m["From"] = []byte(tx.From_.String())
		m["Seq"] = []byte(fmt.Sprint(tx.Seq_))
		m["VoutCount"] = []byte(strconv.Itoa(len(tx.Vout)))

		bs, err := json.Marshal(&tx.Vout)
		if err == nil {
			m["Vouts"] = bs
		}
	case "fleta.Burn":
		tx := t.(*account_tx.Burn)
		m["From"] = []byte(tx.From_.String())
		m["Amount"] = []byte(tx.Amount.String())
		m["Seq"] = []byte(fmt.Sprint(tx.Seq_))
		m["TokenCoord"] = []byte(tx.TokenCoord.String())
	}

	return m, nil
}

func (e *BlockExplorer) BlockDetailMap(height uint32) (map[string][]byte, error) {
	cd, err := e.Kernel.Provider().Data(height)
	if err != nil {
		return nil, err
	}
	b, err := e.Kernel.Block(height)
	if err != nil {
		return nil, err
	}

	tm := time.Unix(int64(cd.Header.Timestamp/uint64(time.Second)), 0)
	m := map[string][]byte{}
	m["Hash"] = []byte(cd.Header.Hash().String())
	m["ChainCoord"] = []byte(b.Header.ChainCoord.String())
	m["Height"] = []byte(strconv.Itoa(int(cd.Header.Height)))
	m["Version"] = []byte(strconv.Itoa(int(cd.Header.Version)))
	m["HashPrevBlock"] = []byte(cd.Header.PrevHash.String())
	m["HashLevelRoot"] = []byte(b.Header.LevelRootHash.String())
	m["Timestamp"] = []byte(tm.Format("2006-01-02 15:04:05"))
	m["FormulationAddress"] = []byte(b.Header.FormulationAddress.String())
	m["TimeoutCount"] = []byte(strconv.Itoa(int(b.Header.TimeoutCount)))
	m["Transactions"] = []byte(strconv.Itoa(len(b.Body.Transactions)))
	return m, nil
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
	// case "typesPerBlock.data":
	// 	e.printJSON(e.typePerBlock(), w)
	case "lastestBlocks.data":
		e.printJSON(e.lastestBlocks(), w)
	case "lastestTransactions.data":
		e.printJSON(e.lastestTransactions(), w)
	case "paginationBlocks.data":
		e.printJSON(e.paginationBlocks(r), w)
	case "paginationTxs.data":
		e.printJSON(e.paginationTxs(r), w)
	case "chainInfoTable.data":
		e.printJSON(e.chainInfoTable(), w)
	}
}
