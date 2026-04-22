package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/goburrow/modbus"
)

type MBClient struct {
	handler      modbus.ClientHandler
	client       modbus.Client
	byteSequence bool
}

func NewMBClient(address string, slaveId byte) (*MBClient, error) {
	var handler modbus.ClientHandler
	segments := strings.Split(address, ":")
	if len(segments) == 3 {
		h, err := getRTUClientHandler(address, slaveId)
		if err != nil {
			return nil, err
		}
		handler = h
	} else {
		h, err := getTCPClientHandler(address, slaveId)
		if err != nil {
			return nil, err
		}
		handler = h
	}

	return &MBClient{
		handler: handler,
		client:  modbus.NewClient(handler),
	}, nil
}

func getRTUClientHandler(address string, slaveId byte) (*modbus.RTUClientHandler, error) {
	device, baudRate, dataBits, parity, stopBits, err := parseRTUAddress(address)
	if err != nil {
		return nil, err
	}

	handler := modbus.NewRTUClientHandler(device)
	handler.BaudRate = baudRate
	handler.DataBits = dataBits
	handler.Parity = parity
	handler.StopBits = stopBits
	handler.SlaveId = slaveId
	handler.Logger = log.Default()

	err = handler.Connect()
	if err != nil {
		return nil, err
	}
	return handler, nil
}

func parseRTUAddress(address string) (string, int, int, string, int, error) {
	segs := strings.Split(address, ":")
	if len(segs) != 3 {
		return "", 0, 0, "", 0, fmt.Errorf("rtu address must use device:baud:mode format")
	}
	if segs[0] == "" {
		return "", 0, 0, "", 0, fmt.Errorf("rtu device path is required")
	}
	baudRate, err := strconv.Atoi(segs[1])
	if err != nil {
		return "", 0, 0, "", 0, fmt.Errorf("baud rate parse error: %w", err)
	}
	if baudRate <= 0 {
		return "", 0, 0, "", 0, fmt.Errorf("baud rate must be greater than 0")
	}

	mode := strings.ToUpper(segs[2])
	if len(mode) != 3 {
		return "", 0, 0, "", 0, fmt.Errorf("serial mode must use format like 8N1")
	}

	dataBits, err := strconv.Atoi(mode[0:1])
	if err != nil || dataBits < 5 || dataBits > 8 {
		return "", 0, 0, "", 0, fmt.Errorf("data bits must be 5, 6, 7 or 8")
	}

	parity := mode[1:2]
	switch parity {
	case "N", "E", "O":
	default:
		return "", 0, 0, "", 0, fmt.Errorf("parity must be N, E or O")
	}

	stopBits, err := strconv.Atoi(mode[2:3])
	if err != nil || (stopBits != 1 && stopBits != 2) {
		return "", 0, 0, "", 0, fmt.Errorf("stop bits must be 1 or 2")
	}

	return segs[0], baudRate, dataBits, parity, stopBits, nil
}

func getTCPClientHandler(address string, slaveId byte) (*modbus.TCPClientHandler, error) {
	handler := modbus.NewTCPClientHandler(address)

	handler.Logger = log.Default()
	handler.SlaveId = slaveId
	handler.Timeout = 10 * time.Second
	err := handler.Connect()
	if err != nil {
		return nil, err
	}
	return handler, nil
}

func (mb *MBClient) SetByteSequence(byteSequence bool) {
	mb.byteSequence = byteSequence
}

func (mb *MBClient) WriteMultiU32(addr uint16, values ...uint32) error {
	bytes := make([]byte, 4*len(values))
	for i, v := range values {
		copy(bytes[i*4:], mb.U32ToBytes(v))
	}
	log.Printf("WriteMultiU32 addr: 0x%02X:%d  value: % X\n", addr, addr, bytes)
	_, err := mb.client.WriteMultipleRegisters(addr, uint16(2*len(values)), bytes)
	return err
}

func (mb *MBClient) WriteFloats(addr uint16, values ...float32) error {
	bytes := make([]byte, 4*len(values))
	for i, v := range values {
		bits := math.Float32bits(v)
		copy(bytes[i*4:], mb.U32ToBytes(bits))
	}
	log.Printf("WriteFloats addr: 0x%02X:%d  value: % X\n", addr, addr, bytes)
	_, err := mb.client.WriteMultipleRegisters(addr, uint16(2*len(values)), bytes)
	return err
}
func (mb *MBClient) WriteU16(addr uint16, value uint16) error {
	if mb.byteSequence {
		value = value<<8 | value>>8
	}
	_, err := mb.client.WriteSingleRegister(addr, value)
	return err
}

func (mb *MBClient) WriteMultiU16(addr uint16, values ...uint16) error {
	bytes := make([]byte, 2*len(values))
	for i, v := range values {
		if mb.byteSequence {
			binary.LittleEndian.PutUint16(bytes[i*2:], v)
		} else {
			binary.BigEndian.PutUint16(bytes[i*2:], v)
		}
	}
	_, err := mb.client.WriteMultipleRegisters(addr, uint16(len(values)), bytes)
	return err
}

func (mb *MBClient) U32ToBytes(value uint32) []byte {
	bytes := make([]byte, 4)
	if mb.byteSequence {
		var data = make([]byte, 4)
		binary.BigEndian.PutUint32(data, value)
		bytes[0] = data[2]
		bytes[1] = data[3]
		bytes[2] = data[0]
		bytes[3] = data[1]
	} else {
		binary.BigEndian.PutUint32(bytes, value)
	}
	return bytes
}

func (mb *MBClient) Close() error {
	type closer interface {
		Close() error
	}
	if c, ok := mb.handler.(closer); ok {
		return c.Close()
	}
	return nil
}
