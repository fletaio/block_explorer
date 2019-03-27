package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/fletaio/block_explorer"

	"github.com/dgraph-io/badger"
	"github.com/fletaio/cmd/closer"
	"github.com/fletaio/core/block"
	"github.com/fletaio/core/node"
	"github.com/fletaio/core/reward"
	"github.com/fletaio/framework/config"
	"github.com/fletaio/framework/peer"
	"github.com/fletaio/framework/router"
	"github.com/fletaio/framework/router/evilnode"
	"github.com/fletaio/framework/rpc"

	"github.com/fletaio/core/data"
	"github.com/fletaio/core/kernel"

	"github.com/fletaio/common"
)

// Config is a configuration for the cmd
type Config struct {
	SeedNodes    []string
	ObserverKeys []string
	Port         int
	APIPort      int
	ExplorerPort int
	StoreRoot    string
	ForceRecover bool
}

func main() {
	var cfg Config
	if err := config.LoadFile("./config.toml", &cfg); err != nil {
		panic(err)
	}
	if len(cfg.StoreRoot) == 0 {
		cfg.StoreRoot = "./data"
	}

	ObserverKeyMap := map[common.PublicHash]bool{}
	for _, k := range cfg.ObserverKeys {
		pubhash, err := common.ParsePublicHash(k)
		if err != nil {
			panic(err)
		}
		ObserverKeyMap[pubhash] = true
	}

	GenCoord := common.NewCoordinate(0, 0)
	act := data.NewAccounter(GenCoord)
	tran := data.NewTransactor(GenCoord)
	if err := initChainComponent(act, tran); err != nil {
		panic(err)
	}

	GenesisContextData, err := initGenesisContextData(act, tran)
	if err != nil {
		panic(err)
	}

	cm := closer.NewManager()
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		<-sigc
		cm.CloseAll()
	}()
	defer cm.CloseAll()

	var ks *kernel.Store
	if s, err := kernel.NewStore(cfg.StoreRoot+"/kernel", BlockchainVersion, act, tran, cfg.ForceRecover); err != nil {
		if cfg.ForceRecover || err != badger.ErrTruncateNeeded {
			panic(err)
		} else {
			fmt.Println(err)
			fmt.Println("Do you want to recover database(it can be failed)? [y/n]")
			var answer string
			fmt.Scanf("%s", &answer)
			if strings.ToLower(answer) == "y" {
				if s, err := kernel.NewStore(cfg.StoreRoot+"/kernel", BlockchainVersion, act, tran, true); err != nil {
					panic(err)
				} else {
					ks = s
				}
			} else {
				os.Exit(1)
			}
		}
	} else {
		ks = s
	}
	cm.Add("kernel.Store", ks)

	rd := &reward.TestNetRewarder{}
	kn, err := kernel.NewKernel(&kernel.Config{
		ChainCoord:              GenCoord,
		ObserverKeyMap:          ObserverKeyMap,
		MaxBlocksPerFormulator:  8,
		MaxTransactionsPerBlock: 5000,
	}, ks, rd, GenesisContextData)
	if err != nil {
		panic(err)
	}
	cm.RemoveAll()
	cm.Add("kernel.Kernel", kn)

	ndcfg := &node.Config{
		ChainCoord: GenCoord,
		SeedNodes:  cfg.SeedNodes,
		Router: router.Config{
			Network: "tcp",
			Port:    cfg.Port,
			EvilNodeConfig: evilnode.Config{
				StorePath: cfg.StoreRoot + "/router",
			},
		},
		Peer: peer.Config{
			StorePath: cfg.StoreRoot + "/peers",
		},
	}
	nd, err := node.NewNode(ndcfg, kn)
	if err != nil {
		panic(err)
	}
	cm.RemoveAll()
	cm.Add("cmd.Node", nd)

	go nd.Run()

	rm := rpc.NewManager()
	cm.RemoveAll()
	cm.Add("rpc.Manager", rm)
	cm.Add("cmd.Node", nd)
	kn.AddEventHandler(rm)

	defer func() {
		cm.CloseAll()
		if err := recover(); err != nil {
			kn.DebugLog("Panic", err)
			panic(err)
		}
	}()

	// Chain
	rm.Add("Version", func(kn *kernel.Kernel, ID interface{}, arg *rpc.Argument) (interface{}, error) {
		return kn.Provider().Version(), nil
	})
	rm.Add("Height", func(kn *kernel.Kernel, ID interface{}, arg *rpc.Argument) (interface{}, error) {
		return kn.Provider().Height(), nil
	})
	rm.Add("LastHash", func(kn *kernel.Kernel, ID interface{}, arg *rpc.Argument) (interface{}, error) {
		return kn.Provider().LastHash(), nil
	})
	rm.Add("Hash", func(kn *kernel.Kernel, ID interface{}, arg *rpc.Argument) (interface{}, error) {
		if arg.Len() < 1 {
			return nil, rpc.ErrInvalidArgument
		}
		height, err := arg.Uint32(0)
		if err != nil {
			return nil, err
		}
		h, err := kn.Provider().Hash(height)
		if err != nil {
			return nil, err
		}
		return h, nil
	})
	rm.Add("Header", func(kn *kernel.Kernel, ID interface{}, arg *rpc.Argument) (interface{}, error) {
		if arg.Len() < 1 {
			return nil, rpc.ErrInvalidArgument
		}
		height, err := arg.Uint32(0)
		if err != nil {
			return nil, err
		}
		h, err := kn.Provider().Header(height)
		if err != nil {
			return nil, err
		}
		return h, nil
	})
	rm.Add("Block", func(kn *kernel.Kernel, ID interface{}, arg *rpc.Argument) (interface{}, error) {
		if arg.Len() < 1 {
			return nil, rpc.ErrInvalidArgument
		}
		height, err := arg.Uint32(0)
		if err != nil {
			return nil, err
		}
		cd, err := kn.Provider().Data(height)
		if err != nil {
			return nil, err
		}
		b := &block.Block{
			Header: cd.Header.(*block.Header),
			Body:   cd.Body.(*block.Body),
		}
		return b, nil
	})

	go func() {
		if err := rm.Run(kn, ":"+strconv.Itoa(cfg.APIPort)); err != nil {
			if http.ErrServerClosed != err {
				panic(err)
			}
		}
	}()

	be, err := blockexplorer.NewBlockExplorer("./explorer_data", kn, "./webfiles")
	if err != nil {
		panic(err)
	}
	go be.StartExplorer(cfg.ExplorerPort)

	cm.Wait()
}
