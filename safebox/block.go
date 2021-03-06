/*
PASL - Personalized Accounts & Secure Ledger

Copyright (C) 2018 PASL Project

Greatly inspired by Kurt Rose's python implementation
https://gist.github.com/kurtbrose/4423605

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package safebox

import (
	"crypto/sha256"

	"github.com/pasl-project/pasl/accounter"
	"github.com/pasl-project/pasl/common"
	"github.com/pasl-project/pasl/crypto"
	"github.com/pasl-project/pasl/safebox/tx"
	"github.com/pasl-project/pasl/utils"
)

type SerializedBlockHeader struct {
	HeaderOnly      uint8
	Version         common.Version
	Index           uint32
	Miner           []byte
	Reward          uint64
	Fee             uint64
	Time            uint32
	Target          uint32
	Nonce           uint32
	Payload         []byte
	PrevSafeboxHash []byte
	OperationsHash  []byte
	Pow             []byte
}

type SerializedBlock struct {
	Header     SerializedBlockHeader
	Operations []tx.Tx
}

type BlockBase interface {
	GetIndex() uint32
	GetMiner() *crypto.Public
	GetReward() uint64
	GetFee() uint64
	GetHash() []byte
	GetVersion() common.Version
	GetTimestamp() uint32
	GetTarget() common.TargetBase
	GetNonce() uint32
	GetPayload() []byte
	GetPrevSafeBoxHash() []byte
	GetOperationsHash() []byte
	GetPow() []byte
	SerializeHeader(willAppendOperations bool) SerializedBlockHeader
	Serialize() SerializedBlock
	GetOperations() []tx.Tx
}

type BlockMetadata struct {
	Index           uint32
	Miner           []byte
	Version         common.Version
	Timestamp       uint32
	Target          uint32
	Nonce           uint32
	Payload         []byte
	PrevSafeBoxHash []byte
	Operations      []tx.Tx
}

type Block struct {
	Meta           *BlockMetadata
	Miner          *crypto.Public
	Target         common.TargetBase
	Operations     []tx.Tx
	OperationsHash [32]byte
	Fee            uint64
	Reward         uint64
	Hash           []byte
	Accounts       []accounter.Account
}

type blockHashBuffer struct {
	Index     uint32
	Accounts  []accounter.AccountHashBuffer
	Timestamp uint32
}

func GetOperationsHash(operations []tx.Tx) [32]byte {
	hash := sha256.Sum256([]byte(""))
	for _, it := range operations {
		h := sha256.New()
		it.SerializeUnderlying(h)
		hash = sha256.Sum256(h.Sum(hash[:]))
	}
	return hash
}

func NewBlock(meta *BlockMetadata) (BlockBase, error) {
	var fee uint64 = 0
	operations := make([]tx.Tx, len(meta.Operations))

	for index, it := range meta.Operations {
		operations[index] = it
		fee += operations[index].GetFee()
	}

	var miner *crypto.Public
	var err error
	if miner, err = crypto.NewPublic(meta.Miner); err != nil {
		return nil, err
	}

	block := &Block{
		Meta:           meta,
		Miner:          miner,
		Target:         common.NewTarget(meta.Target),
		Operations:     operations,
		OperationsHash: GetOperationsHash(operations),
		Fee:            fee,
		Reward:         getReward(meta.Index),
		Accounts:       make([]accounter.Account, 5),
	}
	var i uint32
	for i = 0; i < uint32(len(block.Accounts)); i++ {
		block.Accounts[i] = accounter.Account{
			Number:       i + block.GetIndex(),
			PublicKey:    *block.GetMiner(),
			Balance:      0,
			UpdatedIndex: block.GetIndex(),
			Operations:   0,
		}
	}

	block.Hash = block.GetHash()

	return block, nil
}

func (block *Block) GetAccountsSerialized() []accounter.AccountHashBuffer {
	var result []accounter.AccountHashBuffer = make([]accounter.AccountHashBuffer, len(block.Accounts))
	for i := 0; i < len(result); i++ {
		result[i] = block.Accounts[i].GetHashBuffer()
	}
	return result
}

func (block *Block) GetHash() []byte {
	buf := utils.Serialize(blockHashBuffer{
		Index:     block.GetIndex(),
		Accounts:  block.GetAccountsSerialized(),
		Timestamp: block.GetTimestamp(),
	})
	hash := sha256.New()
	hash.Write(buf)
	return hash.Sum(nil)
}

func (block *Block) GetIndex() uint32 {
	return block.Meta.Index
}

func (block *Block) GetMiner() *crypto.Public {
	return block.Miner
}

func (block *Block) GetReward() uint64 {
	return block.Reward
}

func (block *Block) GetFee() (fee uint64) {
	return block.Fee
}

func (block *Block) GetVersion() common.Version {
	return block.Meta.Version
}

func (block *Block) GetTimestamp() uint32 {
	return block.Meta.Timestamp
}

func (block *Block) GetTarget() common.TargetBase {
	return block.Target
}

func (block *Block) GetNonce() uint32 {
	return block.Meta.Nonce
}

func (block *Block) GetPayload() []byte {
	return block.Meta.Payload
}

func (block *Block) GetPrevSafeBoxHash() []byte {
	return block.Meta.PrevSafeBoxHash
}

func (block *Block) GetOperationsHash() []byte {
	return block.OperationsHash[:]
}

func (block *Block) GetPow() []byte {
	// TODO: fix
	return []byte("")
}

func (block *Block) SerializeHeader(willAppendOperations bool) SerializedBlockHeader {
	var headerOnly uint8
	if willAppendOperations {
		headerOnly = 2
	} else {
		headerOnly = 3
	}
	return SerializedBlockHeader{
		HeaderOnly: headerOnly,
		Version: common.Version{
			Major: 1,
			Minor: 1,
		},
		Index:           block.GetIndex(),
		Miner:           utils.Serialize(block.GetMiner()),
		Reward:          block.GetReward(),
		Fee:             block.GetFee(),
		Time:            block.GetTimestamp(),
		Target:          block.GetTarget().GetCompact(),
		Nonce:           block.GetNonce(),
		Payload:         block.GetPayload(),
		PrevSafeboxHash: block.GetPrevSafeBoxHash(),
		OperationsHash:  block.GetOperationsHash(),
		Pow:             block.GetPow(),
	}
}

func (this *Block) Serialize() SerializedBlock {
	return SerializedBlock{
		Header:     this.SerializeHeader(true),
		Operations: this.Operations,
	}
}

func (this *Block) GetOperations() []tx.Tx {
	return this.Operations
}
