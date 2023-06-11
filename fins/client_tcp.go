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

const DEFAULT_TCP_RESPONSE_TIMEOUT = 20 // ms

//var HandShakeChan chan bool = make(chan bool, 1)

// TcpClient Omron FINS TCP client
type TcpClient struct {
	conn *net.TCPConn
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

// NewTcpClient creates a new Omron FINS TCP client
func NewTcpClient(localAddr, plcAddr TCPAddress) (*TcpClient, error) {

	c := new(TcpClient)
	c.dst = plcAddr.FinsAddress
	c.src = localAddr.FinsAddress
	c.responseTimeoutMs = DEFAULT_TCP_RESPONSE_TIMEOUT
	c.byteOrder = binary.BigEndian

	conn, err := net.DialTCP("tcp", localAddr.TcpAddress, plcAddr.TcpAddress)
	if err != nil {
		return nil, err
	}
	c.conn = conn
	c.handShake()

	c.resp = make([]chan response, 256) //storage for all responses, sid is byte - only 256 values
	go c.listenLoop()
	return c, nil
}

// Set byte order
// Default value: binary.BigEndian
func (c *TcpClient) SetByteOrder(o binary.ByteOrder) {
	c.byteOrder = o
}

// Set response timeout duration (ms).
// Default value: 20ms.
// A timeout of zero can be used to block indefinitely.
func (c *TcpClient) SetTimeoutMs(t uint) {
	c.responseTimeoutMs = time.Duration(t)
}

// Close Closes an Omron FINS connection
func (c *TcpClient) Close() {
	c.closed = true
	c.conn.Close()
}

func (c *TcpClient) ReadInt16(memoryArea byte, address uint16, readCount uint16, isByteSwap bool, isWordSWap bool) ([]byte, int16, error) {

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

func (c *TcpClient) ReadUint16(memoryArea byte, address uint16, readCount uint16, isByteSwap bool, isWordSWap bool) ([]byte, uint16, error) {

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

func (c *TcpClient) ReadInt32(memoryArea byte, address uint16, readCount uint16, isByteSwap bool, isWordSWap bool) ([]byte, int32, error) {

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

func (c *TcpClient) ReadUint32(memoryArea byte, address uint16, readCount uint16, isByteSwap bool, isWordSWap bool) ([]byte, uint32, error) {

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

func (c *TcpClient) ReadFloat32(memoryArea byte, address uint16, readCount uint16, isByteSwap bool, isWordSWap bool) ([]byte, float32, error) {

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
func (c *TcpClient) ReadWords(memoryArea byte, address uint16, readCount uint16) ([]uint16, error) {

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
func (c *TcpClient) ReadBytes(memoryArea byte, address uint16, readCount uint16) ([]byte, error) {

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
func (c *TcpClient) ReadString(memoryArea byte, address uint16, readCount uint16) (string, error) {

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
func (c *TcpClient) ReadBits(memoryArea byte, address uint16, bitOffset byte, readCount uint16) ([]bool, error) {

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
func (c *TcpClient) ReadClock() (*time.Time, error) {

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

// 写入uint16类型
func (c *TcpClient) WriteUint16(memoryArea byte, address uint16, data uint16, isByteSwap bool, isWordSWap bool) error {
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

// 写入int16类型
func (c *TcpClient) WriteInt16(memoryArea byte, address uint16, data int16, isByteSwap bool, isWordSWap bool) error {
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

// 写入uint32
func (c *TcpClient) WriteUint32(memoryArea byte, address uint16, data uint32, isByteSwap bool, isWordSWap bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	x := int32(data)
	bytesBuffer := bytes.NewBuffer([]byte{})
	binary.Write(bytesBuffer, binary.BigEndian, x)
	bts := swap32BitDataBytes(bytesBuffer.Bytes(), isByteSwap, isWordSWap)

	command := writeCommand(memAddr(memoryArea, address), 2, bts)
	return checkResponse(c.sendCommand(command, COMMAND_TYPE_WRITE))
}

// 写入int32
func (c *TcpClient) WriteInt32(memoryArea byte, address uint16, data int32, isByteSwap bool, isWordSWap bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	bytesBuffer := bytes.NewBuffer([]byte{})
	binary.Write(bytesBuffer, binary.BigEndian, data)
	bts := swap32BitDataBytes(bytesBuffer.Bytes(), isByteSwap, isWordSWap)

	command := writeCommand(memAddr(memoryArea, address), 2, bts)
	return checkResponse(c.sendCommand(command, COMMAND_TYPE_WRITE))
}

func (c *TcpClient) WriteFloat32(memoryArea byte, address uint16, data float32, isByteSwap bool, isWordSWap bool) error {
	bits := math.Float32bits(data)
	bytes := make([]byte, 4)
	binary.BigEndian.PutUint32(bytes, bits)

	command := writeCommand(memAddr(memoryArea, address), 2, bytes)
	return checkResponse(c.sendCommand(command, COMMAND_TYPE_WRITE))

}

// WriteWords Writes words to the PLC data area
func (c *TcpClient) WriteWords(memoryArea byte, address uint16, data []uint16) error {

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
func (c *TcpClient) WriteString(memoryArea byte, address uint16, s string) error {

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
func (c *TcpClient) WriteBytes(memoryArea byte, address uint16, b []byte) error {

	c.mu.Lock()
	defer c.mu.Unlock()

	// if checkIsWordMemoryArea(memoryArea) == false {
	// 	return IncompatibleMemoryAreaError{memoryArea}
	// }
	command := writeCommand(memAddr(memoryArea, address), uint16(len(b)), b)
	return checkResponse(c.sendCommand(command, COMMAND_TYPE_WRITE))
}

// WriteBits Writes bits to the PLC data area
func (c *TcpClient) WriteBits(memoryArea byte, address uint16, bitOffset byte, data []bool) error {

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

func (c *TcpClient) nextHeader() *Header {
	sid := c.incrementSid()
	header := defaultCommandHeader(c.src, c.dst, sid)
	return &header
}

func (c *TcpClient) incrementSid() byte {
	c.Lock() //thread-safe sid incrementation
	c.sid++
	sid := c.sid
	c.Unlock()
	c.resp[sid] = make(chan response) //clearing cell of storage for new response
	return sid
}

func (c *TcpClient) handShake() error {
	cmd := []byte{0x46, 0x49, 0x4E, 0x53, 0x00, 0x00, 0x00, 0x0C, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

	_, err := (*c.conn).Write(cmd)
	if err != nil {
		return err
	}
	return nil
}

func (c *TcpClient) packTcpWriteCommand(cmd uint32, payload []byte) []byte {
	length := len(payload)
	buf := make([]byte, 16+length)

	//copy(buf, "FINS")
	buf[0] = 0x46 //FINS
	buf[1] = 0x49
	buf[2] = 0x4e
	buf[3] = 0x53

	//长度
	WriteUint32ToBytes(buf[4:], uint32(length+8))

	//命令码 读写时为2
	WriteUint32ToBytes(buf[8:], uint32(cmd))

	//错误码
	WriteUint32ToBytes(buf[12:], 0)

	//附加数据
	copy(buf[16:], payload)

	return buf
}

func (c *TcpClient) packTcpReadCommand(cmd uint32, payload []byte) []byte {
	length := len(payload)
	buf := make([]byte, 16+length)

	//copy(buf, "FINS")
	buf[0] = 0x46 //FINS
	buf[1] = 0x49
	buf[2] = 0x4e
	buf[3] = 0x53

	//长度
	WriteUint32ToBytes(buf[4:], uint32(length+8))

	//命令码 读写时为2
	WriteUint32ToBytes(buf[8:], uint32(cmd))

	//错误码
	WriteUint32ToBytes(buf[12:], 0)

	//附加数据
	copy(buf[16:], payload)

	return buf
}

func (c *TcpClient) sendCommand(command []byte, commandType uint8) (*response, error) {
	header := c.nextHeader()
	bts := encodeHeader(*header)
	bts = append(bts, command...)

	var cmd []byte
	if commandType == COMMAND_TYPE_READ {
		cmd = c.packTcpReadCommand(2, bts)
	} else {
		cmd = c.packTcpWriteCommand(2, bts)
	}

	for _, cc := range cmd {
		fmt.Printf("%0x ", cc)
	}
	_, err := (*c.conn).Write(cmd)
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

func (c *TcpClient) listenLoop() {
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
			// 头16字节：FINS + 长度 + 命令 + 错误码
			status := ParseUint32(buf[12:])
			if status != 0 {
				log.Printf("TCP状态错误: %v ", status)
				continue
			}

			length := ParseUint32(buf[4:])
			// 判断剩余长度
			if int(length)+8 < n {
				log.Printf("TCP长度错误: %v ", length)
				continue
			}

			command := ParseUint32(buf[8:])

			fmt.Println("Receive:", buf[0:length+8])

			//判断命令码： 握手命令 or 读写命令
			if command == 0 || command == 1 { //握手
				serverNode := buf[23:]
				c.dst.node = serverNode[0]
				serverNode = buf[19:23]
				c.src.node = serverNode[0]
				// fmt.Println("c.dst.node:", c.dst.node)
				// fmt.Println("c.src.node:", c.src.node)
				//HandShakeChan <- true
			} else if command == 2 { //读写
				ans := decodeResponse(buf[16:n])
				fmt.Printf("ans = %v\n", ans)
				fmt.Printf("ans.header.serviceID = %v\n", ans.header.serviceID)
				c.resp[ans.header.serviceID] <- ans
			}

		} else {
			log.Println("cannot read response: ", buf)
		}
	}
}
