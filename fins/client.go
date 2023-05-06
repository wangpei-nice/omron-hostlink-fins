package fins

import (
	"encoding/binary"
	"fmt"
	"time"
)

type Client interface {
	SetByteOrder(o binary.ByteOrder)
	SetTimeoutMs(t uint)
	Close()
	ReadInt16(memoryArea byte, address uint16, readCount uint16, isByteSwap bool, isWordSWap bool) ([]byte, int16, error)
	ReadUint16(memoryArea byte, address uint16, readCount uint16, isByteSwap bool, isWordSWap bool) ([]byte, uint16, error)
	ReadInt32(memoryArea byte, address uint16, readCount uint16, isByteSwap bool, isWordSWap bool) ([]byte, int32, error)
	ReadUint32(memoryArea byte, address uint16, readCount uint16, isByteSwap bool, isWordSWap bool) ([]byte, uint32, error)
	ReadFloat32(memoryArea byte, address uint16, readCount uint16, isByteSwap bool, isWordSWap bool) ([]byte, float32, error)
	ReadWords(memoryArea byte, address uint16, readCount uint16) ([]uint16, error)
	ReadBytes(memoryArea byte, address uint16, readCount uint16) ([]byte, error)
	ReadString(memoryArea byte, address uint16, readCount uint16) (string, error)
	ReadBits(memoryArea byte, address uint16, bitOffset byte, readCount uint16) ([]bool, error)
	ReadClock() (*time.Time, error)
	WriteUint16(memoryArea byte, address uint16, data uint16, isByteSwap bool, isWordSWap bool) error
	WriteInt16(memoryArea byte, address uint16, data int16, isByteSwap bool, isWordSWap bool) error
	WriteUint32(memoryArea byte, address uint16, data uint32, isByteSwap bool, isWordSWap bool) error
	WriteInt32(memoryArea byte, address uint16, data int32, isByteSwap bool, isWordSWap bool) error
	WriteFloat32(memoryArea byte, address uint16, data float32, isByteSwap bool, isWordSWap bool) error
	WriteWords(memoryArea byte, address uint16, data []uint16) error
	WriteString(memoryArea byte, address uint16, s string) error
	WriteBytes(memoryArea byte, address uint16, b []byte) error
	WriteBits(memoryArea byte, address uint16, bitOffset byte, data []bool) error
	// SetBit(memoryArea byte, address uint16, bitOffset byte) error
	// ResetBit(memoryArea byte, address uint16, bitOffset byte) error
	// ToggleBit(memoryArea byte, address uint16, bitOffset byte) error
	// bitTwiddle(memoryArea byte, address uint16, bitOffset byte, value byte) error
	nextHeader() *Header
	incrementSid() byte
	sendCommand(command []byte, commandType uint8) (*response, error)
	listenLoop()
}

func checkIsWordMemoryArea(memoryArea byte) bool {
	if memoryArea == MemoryAreaDMWord ||
		memoryArea == MemoryAreaARWord ||
		memoryArea == MemoryAreaHRWord ||
		memoryArea == MemoryAreaWRWord {
		return true
	}
	return false
}

func checkIsBitMemoryArea(memoryArea byte) bool {
	if memoryArea == MemoryAreaDMBit ||
		memoryArea == MemoryAreaARBit ||
		memoryArea == MemoryAreaHRBit ||
		memoryArea == MemoryAreaWRBit {
		return true
	}
	return false
}

func checkResponse(r *response, e error) error {
	if e != nil {
		return e
	}
	if r.endCode != EndCodeNormalCompletion {
		return fmt.Errorf("error reported by destination, end code 0x%x", r.endCode)
	}
	return nil
}
