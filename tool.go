package main

import (
	"encoding/binary"
	"github.com/goburrow/modbus"
	"github.com/tarm/serial"
	"log"
	"math"
	"strconv"
	"strings"
	"time"
)

type MBClient struct {
	handler      modbus.ClientHandler
	client       modbus.Client
	byteSequence bool
}

func NewMBClient(address string, slaveId byte) *MBClient {
	var handler modbus.ClientHandler
	segments := strings.Split(address, ":")
	if len(segments) == 3 {
		// 串口
		h := getRTUClientHandler(address, slaveId)
		if h == nil {
			return nil
		}
		handler = h
	} else {
		// tcp
		h := getTCPClientHandler(address, slaveId)
		if h == nil {
			return nil
		}
		handler = h
	}

	return &MBClient{
		handler: handler,
		client:  modbus.NewClient(handler),
	}
}

func getRTUClientHandler(address string, slaveId byte) *modbus.RTUClientHandler {
	// /dev/ttyS9:9600:8N1
	segs := strings.Split(address, ":")
	handler := modbus.NewRTUClientHandler(segs[0])
	baudRate, err := strconv.Atoi(segs[1])
	if err != nil {
		log.Printf("%v\n", err)
		return nil
	}
	handler.BaudRate = baudRate
	handler.DataBits = 8
	handler.Parity = string(serial.ParityNone)
	handler.SlaveId = slaveId

	handler.Logger = log.Default()

	err = handler.Connect()
	if err != nil {
		log.Printf("%v\n", err)
		return nil
	}
	return handler
}

func getTCPClientHandler(address string, slaveId byte) *modbus.TCPClientHandler {
	// 127.0.0.1:502
	handler := modbus.NewTCPClientHandler(address)

	handler.Logger = log.Default()
	handler.SlaveId = slaveId
	handler.Timeout = 10 * time.Second
	err := handler.Connect()
	if err != nil {
		log.Printf("%v\n", err)
		return nil
	}
	return handler
}

func (mb *MBClient) SetByteSequence(byteSequence bool) {
	mb.byteSequence = byteSequence
}

func (mb *MBClient) WriteMultiU32(addr uint16, values ...float64) error {
	bytes := make([]byte, 4*len(values))
	for i, v := range values {
		copy(bytes[i*4:], mb.U32ToBytes(uint32(v)))
	}
	log.Printf("WriteMultiU32 addr: 0x%02X:%d  value: % X\n", addr, addr, bytes)
	_, err := mb.client.WriteMultipleRegisters(addr, uint16(2*len(values)), bytes)
	return err
}

func (mb *MBClient) WriteFloats(addr uint16, values ...float64) error {
	bytes := make([]byte, 4*len(values))
	for i, v := range values {
		bits := math.Float32bits(float32(v))
		copy(bytes[i*4:], mb.U32ToBytes(bits))
	}
	log.Printf("WriteFloats addr: 0x%02X:%d  value: % X\n", addr, addr, bytes)
	_, err := mb.client.WriteMultipleRegisters(addr, uint16(2*len(values)), bytes)
	return err
}
func (mb *MBClient) WriteU16(addr uint16, value uint16) error {
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

func (mb *MBClient) Close() {
}
