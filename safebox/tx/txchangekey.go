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

package tx

import (
	"errors"
	"fmt"
	"io"

	"github.com/pasl-project/pasl/accounter"
	"github.com/pasl-project/pasl/crypto"
	"github.com/pasl-project/pasl/utils"
)

type ChangeKey struct {
	Source       uint32
	OperationId  uint32
	Fee          uint64
	Payload      []byte
	PublicKey    crypto.Public
	NewPublickey []byte
	Signature    crypto.SignatureSerialized
}

type changeKeyContext struct {
	Source    *accounter.Account
	NewPublic *crypto.Public
}

type changeKeyToSign struct {
	Source    uint32
	Operation uint32
	Fee       uint64
	Payload   utils.Serializable
	Public    crypto.PublicSerializedPlain
	NewPublic []byte
}

func (this *ChangeKey) GetFee() uint64 {
	return this.Fee
}

func (this *ChangeKey) Validate(getAccount func(number uint32) *accounter.Account) (context interface{}, err error) {
	source := getAccount(this.Source)
	if source == nil {
		return nil, fmt.Errorf("Source account %d not found", this.Source)
	}
	if source.Balance < this.Fee {
		return nil, errors.New("Insufficient balance")
	}

	public, err := crypto.NewPublic(this.NewPublickey)
	if err != nil {
		return nil, err
	}

	return &changeKeyContext{source, public}, nil
}

func (this *ChangeKey) Apply(index uint32, context interface{}) (map[uint32][]accounter.Micro, error) {
	result := make(map[uint32][]accounter.Micro)

	params := context.(*changeKeyContext)
	result[params.Source.Number] = params.Source.KeyChange(params.NewPublic, index)
	result[params.Source.Number] = append(result[params.Source.Number], params.Source.BalanceSub(this.Fee, index)...)
	return result, nil
}

func (this *ChangeKey) Serialize(w io.Writer) error {
	_, err := w.Write(utils.Serialize(this))
	return err
}

func (this *ChangeKey) getBufferToSign() []byte {
	return utils.Serialize(changeKeyToSign{
		Source:    this.Source,
		Operation: this.OperationId,
		Fee:       this.Fee,
		Payload: &utils.BytesWithoutLengthPrefix{
			Bytes: this.Payload,
		},
		Public:    this.PublicKey.SerializedPlain(),
		NewPublic: this.NewPublickey,
	})
}

func (this *ChangeKey) getSignature() *crypto.SignatureSerialized {
	return &this.Signature
}

func (this *ChangeKey) getSourceInfo() (number uint32, operationId uint32, publicKey *crypto.Public) {
	return this.Source, this.OperationId, &this.PublicKey
}
