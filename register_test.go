package main

import (
	"reflect"
	"testing"

	"github.com/goburrow/modbus"
)

func TestParseRegisterSpecValid(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want registerSpec
	}{
		{
			name: "holding uint32 write",
			raw:  "3UW",
			want: registerSpec{function: functionHoldingRegs, valueType: valueUint32, writeMode: writeModeMultiple},
		},
		{
			name: "holding uint16 auto write",
			raw:  "3uw",
			want: registerSpec{function: functionHoldingRegs, valueType: valueUint16, writeMode: writeModeAuto},
		},
		{
			name: "input float32 word swapped",
			raw:  "4FL",
			want: registerSpec{function: functionInputRegs, valueType: valueFloat32, littleEndian: true},
		},
		{
			name: "coil shorthand",
			raw:  "1",
			want: registerSpec{function: functionCoils, valueType: valueUint16},
		},
		{
			name: "discrete explicit uint16",
			raw:  "2u",
			want: registerSpec{function: functionDiscreteInputs, valueType: valueUint16},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRegisterSpec(tt.raw)
			if err != nil {
				t.Fatalf("parseRegisterSpec() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("parseRegisterSpec() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseRegisterSpecInvalid(t *testing.T) {
	tests := []string{
		"",
		"3",
		"3Q",
		"3UWL",
		"3uWw",
		"3uwL",
		"4UW",
		"1W",
		"1UL",
	}

	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			if _, err := parseRegisterSpec(raw); err == nil {
				t.Fatalf("parseRegisterSpec(%q) expected error", raw)
			}
		})
	}
}

func TestValidateReadCountRejectsUnsafeCounts(t *testing.T) {
	uint32Spec := registerSpec{function: functionHoldingRegs, valueType: valueUint32}
	if err := validateReadCount(uint32Spec, 1); err == nil {
		t.Fatalf("validateReadCount() expected odd uint32 count error")
	}
	if err := validateReadCount(uint32Spec, 2); err != nil {
		t.Fatalf("validateReadCount() error = %v", err)
	}

	coilSpec := registerSpec{function: functionCoils, valueType: valueUint16}
	if err := validateReadCount(coilSpec, maxCoilReadCount+1); err == nil {
		t.Fatalf("validateReadCount() expected coil max count error")
	}
}

func TestParseParamsRejectTruncation(t *testing.T) {
	if _, err := parseUint16Param("start address", "0x10000"); err == nil {
		t.Fatalf("parseUint16Param() expected overflow error")
	}
	if _, err := parseByteParam("slave id", 256); err == nil {
		t.Fatalf("parseByteParam() expected overflow error")
	}
}

func TestParseWriteValuesValidatesIntegerTypes(t *testing.T) {
	uint16Spec := registerSpec{function: functionHoldingRegs, valueType: valueUint16, writeMode: writeModeMultiple}
	if _, err := parseWriteValues(uint16Spec, "1.5"); err == nil {
		t.Fatalf("parseWriteValues() expected fractional uint16 error")
	}
	if _, err := parseWriteValues(uint16Spec, "0x10000"); err == nil {
		t.Fatalf("parseWriteValues() expected uint16 overflow error")
	}

	int16Spec := registerSpec{function: functionHoldingRegs, valueType: valueInt16, writeMode: writeModeMultiple}
	got, err := parseWriteValues(int16Spec, "-1,0xFFFF")
	if err != nil {
		t.Fatalf("parseWriteValues() error = %v", err)
	}
	if !reflect.DeepEqual(got.u16, []uint16{0xFFFF, 0xFFFF}) {
		t.Fatalf("parseWriteValues() = %#v, want two 0xFFFF values", got.u16)
	}
}

func TestWriteModeSelectsFunctionCode(t *testing.T) {
	tests := []struct {
		name       string
		spec       registerSpec
		values     string
		wantSingle int
		wantMulti  int
	}{
		{
			name:       "uppercase W single uint16 uses multiple registers",
			spec:       registerSpec{function: functionHoldingRegs, valueType: valueUint16, writeMode: writeModeMultiple},
			values:     "1",
			wantSingle: 0,
			wantMulti:  1,
		},
		{
			name:       "lowercase w single uint16 keeps single register logic",
			spec:       registerSpec{function: functionHoldingRegs, valueType: valueUint16, writeMode: writeModeAuto},
			values:     "1",
			wantSingle: 1,
			wantMulti:  0,
		},
		{
			name:       "lowercase w multiple uint16 uses multiple registers",
			spec:       registerSpec{function: functionHoldingRegs, valueType: valueUint16, writeMode: writeModeAuto},
			values:     "1,2",
			wantSingle: 0,
			wantMulti:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeModbusClient{}
			err := writeRegisters(&MBClient{client: client}, tt.spec, 1, tt.values)
			if err != nil {
				t.Fatalf("writeRegisters() error = %v", err)
			}
			if client.singleWrites != tt.wantSingle || client.multiWrites != tt.wantMulti {
				t.Fatalf("singleWrites=%d multiWrites=%d, want %d/%d", client.singleWrites, client.multiWrites, tt.wantSingle, tt.wantMulti)
			}
		})
	}
}

func TestValidateAddressRangeRejectsOverflow(t *testing.T) {
	if err := validateAddressRange(0xFFFF, 2); err == nil {
		t.Fatalf("validateAddressRange() expected overflow error")
	}
	if err := validateAddressRange(0xFFFE, 2); err != nil {
		t.Fatalf("validateAddressRange() error = %v", err)
	}
}

type fakeModbusClient struct {
	singleWrites int
	multiWrites  int
}

func (f *fakeModbusClient) ReadCoils(address, quantity uint16) ([]byte, error) {
	return nil, nil
}

func (f *fakeModbusClient) ReadDiscreteInputs(address, quantity uint16) ([]byte, error) {
	return nil, nil
}

func (f *fakeModbusClient) WriteSingleCoil(address, value uint16) ([]byte, error) {
	return nil, nil
}

func (f *fakeModbusClient) WriteMultipleCoils(address, quantity uint16, value []byte) ([]byte, error) {
	return nil, nil
}

func (f *fakeModbusClient) ReadInputRegisters(address, quantity uint16) ([]byte, error) {
	return nil, nil
}

func (f *fakeModbusClient) ReadHoldingRegisters(address, quantity uint16) ([]byte, error) {
	return nil, nil
}

func (f *fakeModbusClient) WriteSingleRegister(address, value uint16) ([]byte, error) {
	f.singleWrites++
	return nil, nil
}

func (f *fakeModbusClient) WriteMultipleRegisters(address, quantity uint16, value []byte) ([]byte, error) {
	f.multiWrites++
	return nil, nil
}

func (f *fakeModbusClient) ReadWriteMultipleRegisters(readAddress, readQuantity, writeAddress, writeQuantity uint16, value []byte) ([]byte, error) {
	return nil, nil
}

func (f *fakeModbusClient) MaskWriteRegister(address, andMask, orMask uint16) ([]byte, error) {
	return nil, nil
}

func (f *fakeModbusClient) ReadFIFOQueue(address uint16) ([]byte, error) {
	return nil, nil
}

func TestWordSwappedU32RoundTrip(t *testing.T) {
	client := &MBClient{byteSequence: true}
	bytes := client.U32ToBytes(0x11223344)
	wantBytes := []byte{0x33, 0x44, 0x11, 0x22}
	if !reflect.DeepEqual(bytes, wantBytes) {
		t.Fatalf("U32ToBytes() = % X, want % X", bytes, wantBytes)
	}

	got := decodeU32(bytes, true)
	if got != 0x11223344 {
		t.Fatalf("decodeU32() = 0x%08X, want 0x11223344", got)
	}
}

func TestDecodeU16LittleEndian(t *testing.T) {
	got := decodeU16([]byte{0x34, 0x12}, true)
	if got != 0x1234 {
		t.Fatalf("decodeU16() = 0x%04X, want 0x1234", got)
	}
}

func TestParseRTUAddress(t *testing.T) {
	device, baudRate, dataBits, parity, stopBits, err := parseRTUAddress("/dev/ttyS9:9600:7E2")
	if err != nil {
		t.Fatalf("parseRTUAddress() error = %v", err)
	}
	if device != "/dev/ttyS9" || baudRate != 9600 || dataBits != 7 || parity != "E" || stopBits != 2 {
		t.Fatalf("parseRTUAddress() = %q,%d,%d,%q,%d", device, baudRate, dataBits, parity, stopBits)
	}

	if _, _, _, _, _, err := parseRTUAddress("/dev/ttyS9:9600:9N1"); err == nil {
		t.Fatalf("parseRTUAddress() expected invalid data bits error")
	}
}

func TestModbusHandlersImplementClose(t *testing.T) {
	type closer interface {
		Close() error
	}
	if _, ok := interface{}(modbus.NewTCPClientHandler("127.0.0.1:502")).(closer); !ok {
		t.Fatalf("TCPClientHandler must implement Close")
	}
	if _, ok := interface{}(modbus.NewRTUClientHandler("/dev/ttyS0")).(closer); !ok {
		t.Fatalf("RTUClientHandler must implement Close")
	}
}
