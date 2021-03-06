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

package utils

import (
	"bytes"
	"container/list"
	"encoding/binary"
	"io"
	"reflect"
)

type Serializable interface {
	Serialize(io.Writer) error
	Deserialize(io.Reader) error
}

type BytesWithoutLengthPrefix struct {
	Bytes []byte
}

func (this *BytesWithoutLengthPrefix) Serialize(w io.Writer) error {
	_, err := w.Write(this.Bytes)
	return err
}

func (this *BytesWithoutLengthPrefix) Deserialize(r io.Reader) error {
	_, err := r.Read(this.Bytes[:])
	return err
}

func SerializeBytes(data []byte) []byte {
	dataLen := len(data)
	serialized := make([]byte, 2+dataLen)
	serialized[0] = (byte)(dataLen & 0xFF)
	serialized[1] = (byte)(dataLen >> 8)
	copy(serialized[2:], data)
	return serialized
}

func DeserializeBytes(reader io.Reader) (data []byte, err error) {
	var size int16
	if err = binary.Read(reader, binary.LittleEndian, &size); err != nil {
		return
	}
	data = make([]byte, size)
	if err = binary.Read(reader, binary.LittleEndian, &data); err != nil {
		return nil, err
	}
	return
}

type pair struct {
	a interface{}
	b int
}

func strucWalker(struc interface{}, callback func(*reflect.Value)) {
	v := reflect.ValueOf(struc)
	if reflect.TypeOf(struc).Kind() == reflect.Ptr {
		v = v.Elem()
	}

	wayBack := list.New()
	wayBack.PushBack(pair{
		a: v,
		b: 0,
	})

	step := func(i int, v reflect.Value, el reflect.Value) bool {
		switch kind := el.Kind(); kind {
		case reflect.Struct:
			if el.CanAddr() {
				if _, ok := el.Addr().Interface().(Serializable); ok {
					callback(&el)
					break
				}
			}
			wayBack.PushBack(pair{
				a: v,
				b: i + 1,
			})
			wayBack.PushBack(pair{
				a: el,
				b: 0,
			})
			return false
		case reflect.Slice:
			switch el.Type().Elem().Kind() {
			case reflect.Uint8:
				callback(&el)
			default:
				callback(&el)
				wayBack.PushBack(pair{
					a: v,
					b: i + 1,
				})
				wayBack.PushBack(pair{
					a: el,
					b: 0,
				})
				return false
			}
		default:
			callback(&el)
		}
		return true
	}

	for wayBack.Len() > 0 {
		el := wayBack.Back()
		current := el.Value
		wayBack.Remove(el)

		v := current.(pair).a.(reflect.Value)

		switch kind := v.Kind(); kind {
		case reflect.Struct:
			if v.CanAddr() {
				if _, ok := v.Addr().Interface().(Serializable); ok {
					callback(&v)
					break
				}
			}
			total := v.NumField()
			for i := current.(pair).b; i < total; i++ {
				if !step(i, v, v.Field(i)) {
					break
				}
			}
		case reflect.Slice:
			total := v.Len()
			for i := current.(pair).b; i < total; i++ {
				if !step(i, v, v.Index(i)) {
					break
				}
			}
		default:
			callback(&v)
		}
	}
}

func Serialize(struc interface{}) []byte {
	serialized := &bytes.Buffer{}

	strucWalker(struc, func(value *reflect.Value) {
		switch kind := value.Kind(); kind {
		case reflect.Ptr:
			if err := value.Interface().(Serializable).Serialize(serialized); err != nil {
				Panicf("Custom type serialization failed: %v", err)
			}
		case reflect.Interface:
			if err := value.Interface().(Serializable).Serialize(serialized); err != nil {
				Panicf("Custom type serialization failed: %v", err)
			}
		case reflect.Struct:
			if err := value.Addr().Interface().(Serializable).Serialize(serialized); err != nil {
				Panicf("Custom type serialization failed: %v", err)
			}
		case reflect.Uint8:
			binary.Write(serialized, binary.LittleEndian, uint8(value.Uint()))
		case reflect.Uint16:
			binary.Write(serialized, binary.LittleEndian, uint16(value.Uint()))
		case reflect.Uint32:
			binary.Write(serialized, binary.LittleEndian, uint32(value.Uint()))
		case reflect.Uint64:
			binary.Write(serialized, binary.LittleEndian, uint64(value.Uint()))
		case reflect.String:
			value := value.String()
			binary.Write(serialized, binary.LittleEndian, uint16(len(value)))
			binary.Write(serialized, binary.LittleEndian, []byte(value))
		case reflect.Slice:
			switch value.Type().Elem().Kind() {
			case reflect.Uint8:
				value := value.Bytes()
				binary.Write(serialized, binary.LittleEndian, uint16(len(value)))
				binary.Write(serialized, binary.LittleEndian, value)
			default:
				binary.Write(serialized, binary.LittleEndian, uint32(value.Len()))
			}
		default:
			Panicf("Unimplemented %v", kind)
		}
	})

	return serialized.Bytes()
}

func Deserialize(struc interface{}, r io.Reader) error {
	strucWalker(struc, func(value *reflect.Value) {
		switch kind := value.Kind(); kind {
		case reflect.Ptr:
			if err := value.Interface().(Serializable).Deserialize(r); err != nil {
				Panicf("Custom type deserialization failed: %v", err)
			}
		case reflect.Struct:
			if err := value.Addr().Interface().(Serializable).Deserialize(r); err != nil {
				Panicf("Custom type deserialization failed: %v", err)
			}
		case reflect.Uint8:
			var val uint8
			binary.Read(r, binary.LittleEndian, &val)
			value.SetUint(uint64(val))
		case reflect.Uint16:
			var val uint16
			binary.Read(r, binary.LittleEndian, &val)
			value.SetUint(uint64(val))
		case reflect.Uint32:
			var val uint32
			binary.Read(r, binary.LittleEndian, &val)
			value.SetUint(uint64(val))
		case reflect.Uint64:
			var val uint64
			binary.Read(r, binary.LittleEndian, &val)
			value.SetUint(val)
		case reflect.String:
			var len uint16
			binary.Read(r, binary.LittleEndian, &len)
			var str []byte = make([]byte, len)
			binary.Read(r, binary.LittleEndian, &str)
			value.SetString(string(str))
		case reflect.Slice:
			switch kind := value.Type().Elem().Kind(); kind {
			case reflect.Uint8:
				var len uint16
				binary.Read(r, binary.LittleEndian, &len)
				var data []byte = make([]byte, len)
				binary.Read(r, binary.LittleEndian, &data)
				value.SetBytes(data)
			default:
				var len uint32
				binary.Read(r, binary.LittleEndian, &len)
				value.Set(reflect.MakeSlice(value.Type(), int(len), int(len)))
			}
		default:
			Panicf("Unimplemented %v", kind)
		}
	})
	return nil
}
