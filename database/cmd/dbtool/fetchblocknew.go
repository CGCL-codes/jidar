package main

import (
	"bytes"
	"encoding/hex"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
	"github.com/btcsuite/btcd/wire"
	"time"
	"errors"
)

type fetchBlockNewCmd struct{}


var (
	// fetchBlockCfg defines the configuration options for the command.
	fetchBlockNewCfg = fetchBlockNewCmd{}
)

// Execute is the main entry point for the command.  It's invoked by the parser.
func (cmd *fetchBlockNewCmd) Execute(args []string) error {
	// Setup the global config options and ensure they are valid.
	if err := setupGlobalConfig(); err != nil {
		return err
	}

	if len(args) < 1 {
		return errors.New("required block hash parameter not specified")
	}

	blockHash, err := chainhash.NewHashFromStr(args[0])

	if err != nil {
		return err
	}

	// Load the block database.
	db, err := loadBlockDB()
	if err != nil {
		return err
	}
	defer db.Close()

	var blockBytes []byte

	errDB := db.View(func(tx database.Tx) error {
		log.Infof("Fetching block %s", blockHash)
		startTime := time.Now()
		blockBytes, err = tx.FetchBlockNew(blockHash)
		if err != nil {
			return err
		}
		log.Infof("Loaded block in %v", time.Since(startTime))
		log.Infof("Block Hex: %s", hex.EncodeToString(blockBytes))
		return nil
	})

	msgBlockNew := new(wire.MsgBlockNew)
	err = msgBlockNew.BtcDecode(bytes.NewReader(blockBytes), 0, wire.WitnessEncoding)
	if err != nil {
		return err
	}

	for i, tx := range msgBlockNew.Transactions {
		log.Infof("The %d th transaction", i)
		for j, in :=range tx.TxInNew {
			log.Infof("The %d th in: %v", j, *in)
		}
		for j, out :=range tx.TxOut {
			log.Infof("The %d th out: %v", j, *out)
		}
	}


	return errDB
}

// Usage overrides the usage display for the command.
func (cmd *fetchBlockNewCmd) Usage() string {
	return "<block-hash>"
}
