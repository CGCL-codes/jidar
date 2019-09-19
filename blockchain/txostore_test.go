package blockchain

import (
	"fmt"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/database"
	"github.com/btcsuite/btcd/wire"
	"testing"
)

type params struct {
	*chaincfg.Params
	rpcPort string
}

var mainNetParams = params{
	Params:  &chaincfg.MainNetParams,
	rpcPort: "8334",
}

var activeNetParams = &mainNetParams

func TestDeserializeTxoEntry (t *testing.T) {
	//txHash := newHashFromStr("0000000000000105c04c63625b5530ccfdafdd40c954570ce40ab0f407350374")
	txStr := "80c2c4fbb13d015a8650846b31515ebdc8e423294d2ed74cf50277c472de5056"
	txHash := newHashFromStr(txStr)

	//outpoint := wire.OutPoint{
	//	Hash:	*txHash,
	//	Index:	0,
	//}

	var ops [2]*wire.OutPoint

	for i:=0; i< 2; i++ {
		ops[i] = & wire.OutPoint {
			Hash: *txHash,
			Index: uint32(i),
		}
	}

	var tes [2]*TxoEntry

	for i:=0; i< 2; i++ {
		tes[i] = & TxoEntry {
			BlockHeight: 147253,
			IndexInBlock: uint32(6+i),
		}
	}

	db, err := database.Open("ffldb", "/home/vagrant/.btcd/data/mainnet/blocks_ffldb", activeNetParams.Net)
	if err!=nil {
		t.Fatal(err)
	}

	for i:=0; i<2; i++ {
		var entry *TxoEntry
		err = db.View(func(dbTx database.Tx) error {
			var err error
			entry, err = dbFetchTxoEntry(dbTx, ops[i])
			return err
		})
		if err != nil {
			t.Fatal(err)
		}

		if (entry.IndexInBlock != tes[i].IndexInBlock) || (entry.BlockHeight != tes[i].BlockHeight) {
			t.Fatal(fmt.Sprintf("The %dth output in tx: %s is not stored correctly", i, txStr))
		}
		fmt.Println("Entry: ", *entry)
	}
}
