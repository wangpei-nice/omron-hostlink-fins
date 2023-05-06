package fins

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"net"
	"sync"
	"time"
)

const DEFAULT_UDP_RESPONSE_TIMEOUT = 20 // ms

// UdpClient Omron FINS client
type UdpClient struct {
	conn *net.UDPConn
	resp []chan response
	sync.Mutex
	dst               finsAddress
	src               finsAddress
	sid               byte
	closed            bool
	responseTimeoutMs time.Duration
	byteOrder         binary.ByteOrder
	mu                sync.Mutex
}

// NewUdpClient creates a new Omron FINS client
func NewUdpClient(localAddr, plcAddr UDPAddress) (*UdpClient, error) {

	c := new(UdpClient)
	c.dst = plcAddr.FinsAddress
	c.src = localAddr.FinsAddress
	c.responseTimeoutMs = DEFAULT_UDP_RESPONSE_TIMEOUT
	c.byteOrder = binary.BigEndian

	conn, err := net.DialUDP("udp", localAddr.UdpAddress, plcAddr.UdpAddress)
	if err != nil {
		return nil, err
	}
	c.conn = conn

	c.resp = make([]chan response, 256) //storage for all responses, sid is byte - only 256 values
	go c.listenLoop()
	return c, nil
}

// Set byte order
// Default value: binary.BigEndian
func (c *UdpClient) SetByteOrder(o binary.ByteOrder) {
	c.byteOrder = o
}

// Set response timeout duration (ms).
// Default value: 20ms.
// A timeout of zero can be used to block indefinitely.
func (c *UdpClient) SetTimeoutMs(t uint) {
	c.responseTimeoutMs = time.Duration(t)
}

// Close Closes an Omron FINS connection
func (c *UdpClient) Close() {
	c.closed = true
	c.conn.Close()
}

func (c *UdpClient) ReadInt16(memoryArea byte, address uint16, readCount uint16, isByteSwap bool, isWordSWap bool) ([]byte, int16, error) {

	c.mu.Lock()
	defer c.mu.Unlock()

	command := readCommand(memAddr(memoryArea, address), readCount)
	r, e := c.sendCommand(command, COMMAND_TYPE_READ)
	e = checkResponse(r, e)
	if e != nil {
		return nil, 0, e
	}

	tmpResult := make([]byte, 2)
	tmpResult[0] = r.data[0]
	tmpResult[1] = r.data[1]

	data := int16(binary.BigEndian.Uint16(swap16BitDataBytes(r.data[0:2], isByteSwap)))
	return tmpResult, data, nil
}

func (c *UdpClient) ReadUint16(memoryArea byte, address uint16, readCount uint16, isByteSwap bool, isWordSWap bool) ([]byte, uint16, error) {

	c.mu.Lock()
	defer c.mu.Unlock()

	command := readCommand(memAddr(memoryArea, address), readCount)
	r, e := c.sendCommand(command, COMMAND_TYPE_READ)
	e = checkResponse(r, e)
	if e != nil {
		return nil, 0, e
	}
	tmpResult := make([]byte, 2)
	tmpResult[0] = r.data[0]
	tmpResult[1] = r.data[1]

	data := binary.BigEndian.Uint16(swap16BitDataBytes(r.data[0:2], isByteSwap))
	return tmpResult, data, nil

}

func (c *UdpClient) ReadInt32(memoryArea byte, address uint16, readCount uint16, isByteSwap bool, isWordSWap bool) ([]byte, int32, error) {

	c.mu.Lock()
	defer c.mu.Unlock()

	command := readCommand(memAddr(memoryArea, address), readCount*2)
	r, e := c.sendCommand(command, COMMAND_TYPE_READ)
	e = checkResponse(r, e)
	if e != nil {
		return nil, 0, e
	}

	tmpResult := make([]byte, 4)
	tmpResult[0] = r.data[0]
	tmpResult[1] = r.data[1]
	tmpResult[2] = r.data[2]
	tmpResult[3] = r.data[3]

	data := int32(binary.BigEndian.Uint32(swap32BitDataBytes(r.data[0:4], isByteSwap, isWordSWap)))
	return tmpResult, data, nil
}

func (c *UdpClient) ReadUint32(memoryArea byte, address uint16, readCount uint16, isByteSwap bool, isWordSWap bool) ([]byte, uint32, error) {

	c.mu.Lock()
	defer c.mu.Unlock()

	command := readCommand(memAddr(memoryArea, address), readCount*2)
	r, e := c.sendCommand(command, COMMAND_TYPE_READ)
	e = checkResponse(r, e)
	if e != nil {
		return nil, 0, e
	}
	// data := make([]uint32, readCount, readCount)
	// for i := 0; i < int(readCount); i++ {
	// 	data[i] = binary.BigEndian.Uint32(swap32BitDataBytes(r.data[i*4:i*4+4], isByteSwap, isWordSWap))
	// }

	data := binary.BigEndian.Uint32(swap32BitDataBytes(r.data[0:4], isByteSwap, isWordSWap))

	tmpResult := make([]byte, 4)
	tmpResult[0] = r.data[0]
	tmpResult[1] = r.data[1]
	tmpResult[2] = r.data[2]
	tmpResult[3] = r.data[3]

	return tmpResult, data, nil
}

func (c *UdpClient) ReadFloat32(memoryArea byte, address uint16, readCount uint16, isByteSwap bool, isWordSWap bool) ([]byte, float32, error) {

	c.mu.Lock()
	defer c.mu.Unlock()

	command := readCommand(memAddr(memoryArea, address), readCount*4)
	r, e := c.sendCommand(command, COMMAND_TYPE_READ)
	e = checkResponse(r, e)
	if e != nil {
		return nil, 0, e
	}
	// data := make([]float32, readCount, readCount)
	// for i := 0; i < int(readCount); i++ {
	// 	// raw := binary.BigEndian.Uint32(r.data[i*4 : i*4+4])
	// 	raw := binary.LittleEndian.Uint32(r.data[i*4 : i*4+4])

	// 	data[i] = math.Float32frombits(raw)
	// 	fmt.Println("dataBytes=", data[i])
	// }

	tmpResult := make([]byte, 4)
	tmpResult[0] = r.data[0]
	tmpResult[1] = r.data[1]
	tmpResult[2] = r.data[2]
	tmpResult[3] = r.data[3]

	raw := binary.BigEndian.Uint32(r.data[0:4])
	data := math.Float32frombits(raw)

	return tmpResult, data, nil
}

// ReadWords Reads words from the PLC data area
func (c *UdpClient) ReadWords(memoryArea byte, address uint16, readCount uint16) ([]uint16, error) {

	c.mu.Lock()
	defer c.mu.Unlock()

	if checkIsWordMemoryArea(memoryArea) == false {
		return nil, IncompatibleMemoryAreaError{memoryArea}
	}
	command := readCommand(memAddr(memoryArea, address), readCount)
	r, e := c.sendCommand(command, COMMAND_TYPE_READ)
	e = checkResponse(r, e)
	if e != nil {
		return nil, e
	}

	data := make([]uint16, readCount, readCount)
	for i := 0; i < int(readCount); i++ {
		data[i] = c.byteOrder.Uint16(r.data[i*2 : i*2+2])
	}

	return data, nil
}

// ReadBytes Reads bytes from the PLC data area
func (c *UdpClient) ReadBytes(memoryArea byte, address uint16, readCount uint16) ([]byte, error) {

	// c.mu.Lock()
	// defer c.mu.Unlock()

	if checkIsWordMemoryArea(memoryArea) == false {
		return nil, IncompatibleMemoryAreaError{memoryArea}
	}
	command := readCommand(memAddr(memoryArea, address), readCount)
	r, e := c.sendCommand(command, COMMAND_TYPE_READ)
	e = checkResponse(r, e)
	if e != nil {
		return nil, e
	}

	return r.data, nil
}

// ReadString Reads a string from the PLC data area
func (c *UdpClient) ReadString(memoryArea byte, address uint16, readCount uint16) (string, error) {

	c.mu.Lock()
	defer c.mu.Unlock()

	data, e := c.ReadBytes(memoryArea, address, readCount)
	if e != nil {
		return "", e
	}
	n := bytes.IndexByte(data, 0)
	if n == -1 {
		n = len(data)
	}
	return string(data[:n]), nil
}

// ReadBits Reads bits from the PLC data area
func (c *UdpClient) ReadBits(memoryArea byte, address uint16, bitOffset byte, readCount uint16) ([]bool, error) {

	c.mu.Lock()
	defer c.mu.Unlock()

	// if checkIsBitMemoryArea(memoryArea) == false {
	// 	return nil, IncompatibleMemoryAreaError{memoryArea}
	// }
	command := readCommand(memAddrWithBitOffset(memoryArea, address, bitOffset), readCount)
	r, e := c.sendCommand(command, COMMAND_TYPE_READ)
	e = checkResponse(r, e)
	if e != nil {
		return nil, e
	}

	data := make([]bool, readCount, readCount)
	for i := 0; i < int(readCount); i++ {
		data[i] = r.data[i]&0x01 > 0
	}

	return data, nil
}

// ReadClock Reads the PLC clock
func (c *UdpClient) ReadClock() (*time.Time, error) {

	c.mu.Lock()
	defer c.mu.Unlock()

	r, e := c.sendCommand(clockReadCommand(), COMMAND_TYPE_READ)
	e = checkResponse(r, e)
	if e != nil {
		return nil, e
	}
	year, _ := decodeBCD(r.data[0:1])
	if year < 50 {
		year += 2000
	} else {
		year += 1900
	}
	month, _ := decodeBCD(r.data[1:2])
	day, _ := decodeBCD(r.data[2:3])
	hour, _ := decodeBCD(r.data[3:4])
	minute, _ := decodeBCD(r.data[4:5])
	second, _ := decodeBCD(r.data[5:6])

	t := time.Date(
		int(year), time.Month(month), int(day), int(hour), int(minute), int(second),
		0, // nanosecond
		time.Local,
	)
	return &t, nil
}

//写入uint16类型
func (c *UdpClient) WriteUint16(memoryArea byte, address uint16, data uint16, isByteSwap bool, isWordSWap bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	bts := make([]byte, 2, 2)
	if isByteSwap {
		binary.LittleEndian.PutUint16(bts, data)
	} else {
		binary.BigEndian.PutUint16(bts, data)
	}

	command := writeCommand(memAddr(memoryArea, address), 1, bts)
	return checkResponse(c.sendCommand(command, COMMAND_TYPE_WRITE))
}

//写入int16类型
func (c *UdpClient) WriteInt16(memoryArea byte, address uint16, data int16, isByteSwap bool, isWordSWap bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	bts := make([]byte, 2, 2)
	if isByteSwap {
		binary.LittleEndian.PutUint16(bts, uint16(data))
	} else {
		binary.LittleEndian.PutUint16(bts, uint16(data))
	}
	command := writeCommand(memAddr(memoryArea, address), 1, bts)
	return checkResponse(c.sendCommand(command, COMMAND_TYPE_WRITE))
}

//写入uint32
func (c *UdpClient) WriteUint32(memoryArea byte, address uint16, data uint32, isByteSwap bool, isWordSWap bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	x := int32(data)
	bytesBuffer := bytes.NewBuffer([]byte{})
	binary.Write(bytesBuffer, binary.BigEndian, x)
	bts := swap32BitDataBytes(bytesBuffer.Bytes(), isByteSwap, isWordSWap)

	command := writeCommand(memAddr(memoryArea, address), 2, bts)
	return checkResponse(c.sendCommand(command, COMMAND_TYPE_WRITE))
}

//写入int32
func (c *UdpClient) WriteInt32(memoryArea byte, address uint16, data int32, isByteSwap bool, isWordSWap bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	bytesBuffer := bytes.NewBuffer([]byte{})
	binary.Write(bytesBuffer, binary.BigEndian, data)
	bts := swap32BitDataBytes(bytesBuffer.Bytes(), isByteSwap, isWordSWap)

	command := writeCommand(memAddr(memoryArea, address), 2, bts)
	return checkResponse(c.sendCommand(command, COMMAND_TYPE_WRITE))
}

func (c *UdpClient) WriteFloat32(memoryArea byte, address uint16, data float32, isByteSwap bool, isWordSWap bool) error {
	bits := math.Float32bits(data)
	bytes := make([]byte, 4)
	binary.BigEndian.PutUint32(bytes, bits)

	command := writeCommand(memAddr(memoryArea, address), 2, bytes)
	return checkResponse(c.sendCommand(command, COMMAND_TYPE_WRITE))

}

// WriteWords Writes words to the PLC data area
func (c *UdpClient) WriteWords(memoryArea byte, address uint16, data []uint16) error {

	c.mu.Lock()
	defer c.mu.Unlock()

	if checkIsWordMemoryArea(memoryArea) == false {
		return IncompatibleMemoryAreaError{memoryArea}
	}
	l := uint16(len(data))
	bts := make([]byte, 2*l, 2*l)
	for i := 0; i < int(l); i++ {
		c.byteOrder.PutUint16(bts[i*2:i*2+2], data[i])
	}
	command := writeCommand(memAddr(memoryArea, address), l, bts)

	return checkResponse(c.sendCommand(command, COMMAND_TYPE_WRITE))
}

// WriteString Writes a string to the PLC data area
func (c *UdpClient) WriteString(memoryArea byte, address uint16, s string) error {

	c.mu.Lock()
	defer c.mu.Unlock()

	// if checkIsWordMemoryArea(memoryArea) == false {
	// 	return IncompatibleMemoryAreaError{memoryArea}
	// }
	bts := make([]byte, 2*len(s), 2*len(s))
	copy(bts, s)

	command := writeCommand(memAddr(memoryArea, address), uint16((len(s)+1)/2), bts) //TODO: test on real PLC

	return checkResponse(c.sendCommand(command, COMMAND_TYPE_WRITE))
}

// WriteBytes Writes bytes array to the PLC data area
func (c *UdpClient) WriteBytes(memoryArea byte, address uint16, b []byte) error {

	c.mu.Lock()
	defer c.mu.Unlock()

	// if checkIsWordMemoryArea(memoryArea) == false {
	// 	return IncompatibleMemoryAreaError{memoryArea}
	// }
	command := writeCommand(memAddr(memoryArea, address), uint16(len(b)), b)
	return checkResponse(c.sendCommand(command, COMMAND_TYPE_WRITE))
}

// WriteBits Writes bits to the PLC data area
func (c *UdpClient) WriteBits(memoryArea byte, address uint16, bitOffset byte, data []bool) error {

	c.mu.Lock()
	defer c.mu.Unlock()

	// if checkIsBitMemoryArea(memoryArea) == false {
	// 	return IncompatibleMemoryAreaError{memoryArea}
	// }
	l := uint16(len(data))
	bts := make([]byte, 0, l)
	var d byte
	for i := 0; i < int(l); i++ {
		if data[i] {
			d = 0x01
		} else {
			d = 0x00
		}
		bts = append(bts, d)
	}
	command := writeCommand(memAddrWithBitOffset(memoryArea, address, bitOffset), l, bts)

	return checkResponse(c.sendCommand(command, COMMAND_TYPE_WRITE))
}

// // SetBit Sets a bit in the PLC data area
// func (c *UdpClient) SetBit(memoryArea byte, address uint16, bitOffset byte) error {
// 	return c.bitTwiddle(memoryArea, address, bitOffset, 0x01)
// }

// // ResetBit Resets a bit in the PLC data area
// func (c *UdpClient) ResetBit(memoryArea byte, address uint16, bitOffset byte) error {
// 	return c.bitTwiddle(memoryArea, address, bitOffset, 0x00)
// }

// // ToggleBit Toggles a bit in the PLC data area
// func (c *UdpClient) ToggleBit(memoryArea byte, address uint16, bitOffset byte) error {
// 	b, e := c.ReadBits(memoryArea, address, bitOffset, 1)
// 	if e != nil {
// 		return e
// 	}
// 	var t byte
// 	if b[0] {
// 		t = 0x00
// 	} else {
// 		t = 0x01
// 	}
// 	return c.bitTwiddle(memoryArea, address, bitOffset, t)
// }

// func (c *UdpClient) bitTwiddle(memoryArea byte, address uint16, bitOffset byte, value byte) error {
// 	if checkIsBitMemoryArea(memoryArea) == false {
// 		return IncompatibleMemoryAreaError{memoryArea}
// 	}
// 	mem := memoryAddress{memoryArea, address, bitOffset}
// 	command := writeCommand(mem, 1, []byte{value})

// 	return checkResponse(c.sendCommand(command))
// }

func (c *UdpClient) nextHeader() *Header {
	sid := c.incrementSid()
	header := defaultCommandHeader(c.src, c.dst, sid)
	return &header
}

func (c *UdpClient) incrementSid() byte {
	c.Lock() //thread-safe sid incrementation
	c.sid++
	sid := c.sid
	c.Unlock()
	c.resp[sid] = make(chan response) //clearing cell of storage for new response
	return sid
}

func (c *UdpClient) sendCommand(command []byte, commandType uint8) (*response, error) {
	header := c.nextHeader()
	bts := encodeHeader(*header)
	bts = append(bts, command...)
	_, err := (*c.conn).Write(bts)
	if err != nil {
		return nil, err
	}

	// if response timeout is zero, block indefinitely
	if c.responseTimeoutMs > 0 {
		select {
		case resp := <-c.resp[header.serviceID]:
			return &resp, nil
		case <-time.After(c.responseTimeoutMs * time.Millisecond):
			return nil, ResponseTimeoutError{c.responseTimeoutMs}
		}
	} else {
		resp := <-c.resp[header.serviceID]
		return &resp, nil
	}
}

func (c *UdpClient) listenLoop() {
	for {
		buf := make([]byte, 2048)
		n, err := bufio.NewReader(c.conn).Read(buf)
		if err != nil {
			// do not complain when connection is closed by user
			if !c.closed {
				// log.Fatal(err)
				log.Printf("连接已经 处于关闭状态:error:%v ", err)
			}
			break
		}

		if n > 0 {
			ans := decodeResponse(buf[:n])
			c.resp[ans.header.serviceID] <- ans

			fmt.Printf("ans = %v\n", ans)

		} else {
			log.Println("cannot read response: ", buf)
		}
	}
}

// func swap32BitDataBytes(dataBytes []byte, isByteSwap bool, isWordSwap bool) []byte {

// 	if !isByteSwap && !isWordSwap {
// 		return dataBytes
// 	}

// 	if len(dataBytes) < 4 {
// 		return dataBytes
// 	}

// 	var newDataBytes = make([]byte, len(dataBytes))

// 	if isByteSwap {
// 		newDataBytes[0] = dataBytes[1]
// 		newDataBytes[1] = dataBytes[0]
// 		newDataBytes[2] = dataBytes[3]
// 		newDataBytes[3] = dataBytes[2]
// 		// Copy new DataBytes to the original DataBytes which can combine the ByteSwap with the WordSwap operation.
// 		copy(dataBytes, newDataBytes)
// 	}
// 	if isWordSwap {
// 		newDataBytes[0] = dataBytes[2]
// 		newDataBytes[1] = dataBytes[3]
// 		newDataBytes[2] = dataBytes[0]
// 		newDataBytes[3] = dataBytes[1]
// 	}

// 	return newDataBytes
// }
