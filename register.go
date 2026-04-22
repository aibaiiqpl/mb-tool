package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
)

const (
	functionCoils          byte = '1'
	functionDiscreteInputs byte = '2'
	functionHoldingRegs    byte = '3'
	functionInputRegs      byte = '4'

	valueUint32  byte = 'U'
	valueInt32   byte = 'S'
	valueUint16  byte = 'u'
	valueInt16   byte = 's'
	valueFloat32 byte = 'F'

	maxCoilReadCount     uint = 2000
	maxRegisterReadCount uint = 125
	maxRegisterWriteQty  uint = 123
)

type writeMode uint8

const (
	writeModeNone writeMode = iota
	writeModeAuto
	writeModeMultiple
)

type registerSpec struct {
	function     byte
	valueType    byte
	littleEndian bool
	writeMode    writeMode
}

type writeValues struct {
	u16 []uint16
	u32 []uint32
	f32 []float32
}

func parseRegisterSpec(raw string) (registerSpec, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return registerSpec{}, fmt.Errorf("register type is empty")
	}

	spec := registerSpec{function: raw[0]}
	switch spec.function {
	case functionCoils, functionDiscreteInputs:
		return parseBitRegisterSpec(raw, spec)
	case functionHoldingRegs, functionInputRegs:
		return parseWordRegisterSpec(raw, spec)
	default:
		return registerSpec{}, fmt.Errorf("unsupported function code %q", raw[0])
	}
}

func parseBitRegisterSpec(raw string, spec registerSpec) (registerSpec, error) {
	if len(raw) == 1 {
		spec.valueType = valueUint16
		return spec, nil
	}
	if raw == string([]byte{spec.function, valueUint16}) {
		spec.valueType = valueUint16
		return spec, nil
	}
	return registerSpec{}, fmt.Errorf("function %c only supports read type %cu", spec.function, spec.function)
}

func parseWordRegisterSpec(raw string, spec registerSpec) (registerSpec, error) {
	if len(raw) < 2 {
		return registerSpec{}, fmt.Errorf("register type %q missing value type", raw)
	}

	spec.valueType = raw[1]
	switch spec.valueType {
	case valueUint32, valueInt32, valueUint16, valueInt16, valueFloat32:
	default:
		return registerSpec{}, fmt.Errorf("unsupported value type %q", raw[1])
	}

	for i := 2; i < len(raw); i++ {
		switch raw[i] {
		case 'L':
			if spec.littleEndian {
				return registerSpec{}, fmt.Errorf("register type %q repeats byte order flag L", raw)
			}
			if spec.isWrite() {
				return registerSpec{}, fmt.Errorf("byte order flag L must appear before write flag")
			}
			spec.littleEndian = true
		case 'W':
			if spec.isWrite() {
				return registerSpec{}, fmt.Errorf("register type %q repeats write flag", raw)
			}
			spec.writeMode = writeModeMultiple
		case 'w':
			if spec.isWrite() {
				return registerSpec{}, fmt.Errorf("register type %q repeats write flag", raw)
			}
			spec.writeMode = writeModeAuto
		default:
			return registerSpec{}, fmt.Errorf("unsupported register type flag %q", raw[i])
		}
	}
	if spec.isWrite() && spec.function != functionHoldingRegs {
		return registerSpec{}, fmt.Errorf("write only supports holding registers function 3")
	}
	return spec, nil
}

func (s registerSpec) isWrite() bool {
	return s.writeMode != writeModeNone
}

func (s registerSpec) isBitRead() bool {
	return s.function == functionCoils || s.function == functionDiscreteInputs
}

func (s registerSpec) is32Bit() bool {
	return s.valueType == valueUint32 || s.valueType == valueInt32 || s.valueType == valueFloat32
}

func (s registerSpec) registersPerValue() uint16 {
	if s.is32Bit() {
		return 2
	}
	return 1
}

func validateReadCount(spec registerSpec, cnt uint) error {
	if cnt == 0 {
		return fmt.Errorf("count must be greater than 0")
	}
	if spec.isBitRead() {
		if cnt > maxCoilReadCount {
			return fmt.Errorf("coil/input count must be <= %d", maxCoilReadCount)
		}
		return nil
	}
	if cnt > maxRegisterReadCount {
		return fmt.Errorf("register count must be <= %d", maxRegisterReadCount)
	}
	if spec.is32Bit() && cnt%2 != 0 {
		return fmt.Errorf("32-bit value types require an even register count")
	}
	return nil
}

func validateAddressRange(start uint16, quantity uint16) error {
	if quantity == 0 {
		return fmt.Errorf("quantity must be greater than 0")
	}
	if uint32(start)+uint32(quantity)-1 > math.MaxUint16 {
		return fmt.Errorf("address range 0x%X + %d exceeds 0xFFFF", start, quantity)
	}
	return nil
}

func parseUint16Param(name string, raw string) (uint16, error) {
	value, err := strconv.ParseUint(raw, 0, 32)
	if err != nil {
		return 0, fmt.Errorf("%s parse error: %w", name, err)
	}
	if value > math.MaxUint16 {
		return 0, fmt.Errorf("%s must be <= 0xFFFF", name)
	}
	return uint16(value), nil
}

func parseByteParam(name string, value uint) (byte, error) {
	if value > math.MaxUint8 {
		return 0, fmt.Errorf("%s must be <= 255", name)
	}
	return byte(value), nil
}

func parseWriteValues(spec registerSpec, raw string) (writeValues, error) {
	tokens, err := splitValueList(raw)
	if err != nil {
		return writeValues{}, err
	}

	switch spec.valueType {
	case valueUint32:
		values := make([]uint32, 0, len(tokens))
		for _, token := range tokens {
			value, err := strconv.ParseUint(token, 0, 32)
			if err != nil {
				return writeValues{}, fmt.Errorf("uint32 value %q parse error: %w", token, err)
			}
			values = append(values, uint32(value))
		}
		return writeValues{u32: values}, nil
	case valueInt32:
		values := make([]uint32, 0, len(tokens))
		for _, token := range tokens {
			value, err := parseSigned32Bits(token)
			if err != nil {
				return writeValues{}, err
			}
			values = append(values, value)
		}
		return writeValues{u32: values}, nil
	case valueUint16:
		values := make([]uint16, 0, len(tokens))
		for _, token := range tokens {
			value, err := strconv.ParseUint(token, 0, 16)
			if err != nil {
				return writeValues{}, fmt.Errorf("uint16 value %q parse error: %w", token, err)
			}
			values = append(values, uint16(value))
		}
		return writeValues{u16: values}, nil
	case valueInt16:
		values := make([]uint16, 0, len(tokens))
		for _, token := range tokens {
			value, err := parseSigned16Bits(token)
			if err != nil {
				return writeValues{}, err
			}
			values = append(values, value)
		}
		return writeValues{u16: values}, nil
	case valueFloat32:
		values := make([]float32, 0, len(tokens))
		for _, token := range tokens {
			value, err := strconv.ParseFloat(token, 32)
			if err != nil {
				return writeValues{}, fmt.Errorf("float32 value %q parse error: %w", token, err)
			}
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return writeValues{}, fmt.Errorf("float32 value %q must be finite", token)
			}
			values = append(values, float32(value))
		}
		return writeValues{f32: values}, nil
	default:
		return writeValues{}, fmt.Errorf("unsupported value type %q", spec.valueType)
	}
}

func splitValueList(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("value is required for write")
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			return nil, fmt.Errorf("value list contains an empty item")
		}
		values = append(values, value)
	}
	return values, nil
}

func parseSigned32Bits(token string) (uint32, error) {
	if hasHexPrefix(token) {
		value, err := strconv.ParseUint(token, 0, 32)
		if err != nil {
			return 0, fmt.Errorf("int32 value %q parse error: %w", token, err)
		}
		return uint32(value), nil
	}
	value, err := strconv.ParseInt(token, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("int32 value %q parse error: %w", token, err)
	}
	return uint32(int32(value)), nil
}

func parseSigned16Bits(token string) (uint16, error) {
	if hasHexPrefix(token) {
		value, err := strconv.ParseUint(token, 0, 16)
		if err != nil {
			return 0, fmt.Errorf("int16 value %q parse error: %w", token, err)
		}
		return uint16(value), nil
	}
	value, err := strconv.ParseInt(token, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("int16 value %q parse error: %w", token, err)
	}
	return uint16(int16(value)), nil
}

func hasHexPrefix(token string) bool {
	return strings.HasPrefix(token, "0x") || strings.HasPrefix(token, "0X")
}

func decodeU32(data []byte, littleEndian bool) uint32 {
	if !littleEndian {
		return binary.BigEndian.Uint32(data)
	}
	low := binary.BigEndian.Uint16(data[0:2])
	high := binary.BigEndian.Uint16(data[2:4])
	return uint32(high)<<16 | uint32(low)
}

func decodeU16(data []byte, littleEndian bool) uint16 {
	if littleEndian {
		return binary.LittleEndian.Uint16(data)
	}
	return binary.BigEndian.Uint16(data)
}

func logReadResults(spec registerSpec, startAddr uint16, cnt uint16, results []byte) error {
	if spec.isBitRead() {
		var offset uint16
		for i := 0; i < len(results) && offset < cnt; i++ {
			for bit := 0; bit < 8 && offset < cnt; bit++ {
				value := (results[i] >> bit) & 1
				log.Printf("addr: 0x%02X:%d  value: %d (%s)", uint32(startAddr)+uint32(offset), uint32(startAddr)+uint32(offset), value, map[byte]string{0: "OFF", 1: "ON"}[value])
				offset++
			}
		}
		return nil
	}

	expectedBytes := int(cnt) * 2
	if len(results) != expectedBytes {
		return fmt.Errorf("response byte count %d does not match expected %d", len(results), expectedBytes)
	}

	registerStep := spec.registersPerValue()
	var offset uint16
	for i := 0; i < len(results); i += int(registerStep) * 2 {
		addr := uint32(startAddr) + uint32(offset)
		switch spec.valueType {
		case valueUint32:
			value := decodeU32(results[i:i+4], spec.littleEndian)
			log.Printf("addr: 0x%02X:%d  value: 0x%08X : %d", addr, addr, value, value)
		case valueInt32:
			value := int32(decodeU32(results[i:i+4], spec.littleEndian))
			log.Printf("addr: 0x%02X:%d  value: 0x%08X : %d", addr, addr, uint32(value), value)
		case valueUint16:
			value := decodeU16(results[i:i+2], spec.littleEndian)
			log.Printf("addr: 0x%02X:%d  value: 0x%04X : %d", addr, addr, value, value)
		case valueInt16:
			value := int16(decodeU16(results[i:i+2], spec.littleEndian))
			log.Printf("addr: 0x%02X:%d  value: 0x%04X : %d", addr, addr, uint16(value), value)
		case valueFloat32:
			value := math.Float32frombits(decodeU32(results[i:i+4], spec.littleEndian))
			log.Printf("addr: 0x%02X:%d  value: %g", addr, addr, value)
		default:
			return fmt.Errorf("unsupported value type %q", spec.valueType)
		}
		offset += registerStep
	}
	return nil
}
