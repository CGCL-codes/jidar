// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/wire"
	"time"

	"github.com/btcsuite/btcd/database"
	"github.com/btcsuite/btcutil"
)

func (b *BlockChain) CreateMsgBlockNew(msgBlock *wire.MsgBlock, blockHeight uint32) (*wire.MsgBlockNew, error) {
	msgBlockNew := new(wire.MsgBlockNew)
	msgBlockNew.Header = msgBlock.Header
	msgBlockNew.Transactions = make([]*wire.MsgTxNew, len(msgBlock.Transactions))
	var entry *TxoEntry
	var err error
	for i, tx := range msgBlock.Transactions {
		txNew := new(wire.MsgTxNew)
		txNew.Version = tx.Version
		txNew.LockTime = tx.LockTime
		txNew.TxOut = tx.TxOut
		txNew.TxInNew = make([]*wire.TxInNew, len(tx.TxIn))
		for j, in := range tx.TxIn {
			inNew := new(wire.TxInNew)
			inNew.PreviousOutPoint = in.PreviousOutPoint
			inNew.SignatureScript = in.SignatureScript
			inNew.Witness = in.Witness
			inNew.Sequence = in.Sequence
			if blockHeight != 0 {
				// Continue for the genesis block, since leveldb is not created
				entry, err = b.FetchTxoEntry(&in.PreviousOutPoint)
				if err != nil {
					return nil, err
				}
			}

			if entry == nil {
				entry = new(TxoEntry)
				if i != 0 {
					// if entry == nil && not the coinbase, check if the previousOutput spends an output in this block
					valid := false
					pi := 0
					for ; pi < i; pi++ {
						if msgBlock.Transactions[pi].TxHash() == in.PreviousOutPoint.Hash {
							valid = true
							log.Trace(fmt.Sprintf("--------------- find it , fuck it, blockNum: %d, txpos: %d, "+
								"inpos: %d, reference txpos: %d", blockHeight, i, j, pi))
							break
						}
					}
					if valid {
						entry.BlockHeight = blockHeight
						txoCount := 0
						for k := 0; k < pi; k++ {
							txoCount += len(msgBlock.Transactions[k].TxOut)
						}
						entry.IndexInBlock = uint32(txoCount) + in.PreviousOutPoint.Index
					} else {
						return nil, errors.New(fmt.Sprintf("Cannot find the PreviousOutPoint: ",
							in.PreviousOutPoint.String()))
					}
				} else {
					// if it is a coinbase, set BlockHeight and IndexInBlock as -1
					entry.IndexInBlock = 0
					entry.BlockHeight = 0
				}
			}
			inNew.BlockHeight = entry.BlockHeight
			inNew.IndexInBlock = entry.IndexInBlock
			txNew.TxInNew[j] = inNew
		}
		msgBlockNew.Transactions[i] = txNew
	}
	return msgBlockNew, nil
}

// maybeAcceptBlock potentially accepts a block into the block chain and, if
// accepted, returns whether or not it is on the main chain.  It performs
// several validation checks which depend on its position within the block chain
// before adding it.  The block is expected to have already gone through
// ProcessBlock before calling this function with it.
//
// The flags are also passed to checkBlockContext and connectBestChain.  See
// their documentation for how the flags modify their behavior.
//
// This function MUST be called with the chain state lock held (for writes).
func (b *BlockChain) maybeAcceptBlock(block *btcutil.Block, flags BehaviorFlags) (bool, time.Duration, error) {
	// The height of this block is one more than the referenced previous
	// block.
	prevHash := &block.MsgBlock().Header.PrevBlock
	prevNode := b.index.LookupNode(prevHash)
	if prevNode == nil {
		str := fmt.Sprintf("previous block %s is unknown", prevHash)
		return false, 0, ruleError(ErrPreviousBlockUnknown, str)
	} else if b.index.NodeStatus(prevNode).KnownInvalid() {
		str := fmt.Sprintf("previous block %s is known to be invalid", prevHash)
		return false, 0, ruleError(ErrInvalidAncestorBlock, str)
	}

	blockHeight := prevNode.height + 1
	block.SetHeight(blockHeight)

	// fill the msgBlockNew in block
	msgBlock := block.MsgBlock()
	msgBlockNew, err := b.CreateMsgBlockNew(msgBlock, uint32(blockHeight))
	if err!=nil {
		return false, 0, err
	}
	block.SetMsgBlockNew(msgBlockNew)

	// The block must pass all of the validation rules which depend on the
	// position of the block within the block chain.
	err = b.checkBlockContext(block, prevNode, flags)
	if err != nil {
		return false, 0, err
	}

	// Insert the block into the database if it's not already there.  Even
	// though it is possible the block will ultimately fail to connect, it
	// has already passed all proof-of-work and validity tests which means
	// it would be prohibitively expensive for an attacker to fill up the
	// disk with a bunch of blocks that fail to connect.  This is necessary
	// since it allows block download to be decoupled from the much more
	// expensive connection logic.  It also has some other nice properties
	// such as making blocks that never become part of the main chain or
	// blocks that fail to connect available for further analysis.
	err = b.db.Update(func(dbTx database.Tx) error {
		return dbStoreBlock(dbTx, block)
	})
	if err != nil {
		return false, 0, err
	}

	// Create a new block node for the block and add it to the node index. Even
	// if the block ultimately gets connected to the main chain, it starts out
	// on a side chain.
	blockHeader := &block.MsgBlock().Header
	newNode := newBlockNode(blockHeader, prevNode)
	newNode.status = statusDataStored

	b.index.AddNode(newNode)
	err = b.index.flushToDB()
	if err != nil {
		return false, 0, err
	}

	// Connect the passed block to the chain while respecting proper chain
	// selection according to the chain with the most proof of work.  This
	// also handles validation of the transaction scripts.
	isMainChain, duration, err := b.connectBestChain(newNode, block, flags)
	if err != nil {
		return false, 0, err
	}

	// Notify the caller that the new block was accepted into the block
	// chain.  The caller would typically want to react by relaying the
	// inventory to other peers.
	b.chainLock.Unlock()
	b.sendNotification(NTBlockAccepted, block)
	b.chainLock.Lock()

	return isMainChain, duration, nil
}
