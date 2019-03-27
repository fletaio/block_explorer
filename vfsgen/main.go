package main

import (
	"log"
	"os"

	"github.com/fletaio/block_explorer"

	"github.com/shurcooL/vfsgen"
)

func main() {
	err := vfsgen.Generate(blockexplorer.Assets, vfsgen.Options{
		PackageName:  "blockexplorer",
		BuildTags:    "!dev",
		VariableName: "Assets",
	})
	if err != nil {
		log.Fatal(err)
	}

	oldLocation := "./assets_vfsdata.go"
	newLocation := "../assets_vfsdata.go"
	err = os.Rename(oldLocation, newLocation)
	if err != nil {
		log.Fatal(err)
	}
}
