package main

import (
	"flag"
	"fmt"
	"log"
	"time"
)

func main() {
	var err error
	var address string
	var regType string
	var startAddrStr string
	var cnt uint
	var slaveId uint
	var valuesStr string
	var multiRead bool
	var multiReadTime uint
	// /dev/ttyS9:9600:8N1
	// 127.0.0.1:1502
	flag.StringVar(&address, "h", "", "modbus server address")
	flag.StringVar(&regType, "t", "3u", "register type")
	flag.StringVar(&startAddrStr, "r", "0", "start address")
	flag.UintVar(&cnt, "c", 1, "count")
	flag.UintVar(&slaveId, "a", 1, "slave id")
	flag.StringVar(&valuesStr, "v", "", "value")
	flag.BoolVar(&multiRead, "m", false, "multi read mode")
	flag.UintVar(&multiReadTime, "M", 1000, "multi read delay time(ms)")
	flag.Parse()

	if address == "" {
		showHelp()
		return
	}

	spec, err := parseRegisterSpec(regType)
	if err != nil {
		log.Println(err)
		return
	}
	startAddr, err := parseUint16Param("start address", startAddrStr)
	if err != nil {
		log.Println(err)
		return
	}
	slaveID, err := parseByteParam("slave id", slaveId)
	if err != nil {
		log.Println(err)
		return
	}

	if spec.isWrite() {
		err = validateWriteRequest(spec, startAddr, valuesStr)
	} else {
		err = validateReadCount(spec, cnt)
		if err == nil {
			err = validateAddressRange(startAddr, uint16(cnt))
		}
	}
	if err != nil {
		log.Println(err)
		return
	}

	client, err := NewMBClient(address, slaveID)
	if err != nil {
		log.Println(err)
		return
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Println("Close error:", err)
		}
	}()
	client.SetByteSequence(spec.littleEndian)

	if spec.isWrite() {
		if err := writeRegisters(client, spec, startAddr, valuesStr); err != nil {
			log.Println(err)
		}
		return
	}

	for {
		if err := readRegisters(client, spec, startAddr, uint16(cnt)); err != nil {
			log.Println(err)
			return
		}
		if !multiRead {
			break
		}
		time.Sleep(time.Duration(multiReadTime) * time.Millisecond)
	}
}

func validateWriteRequest(spec registerSpec, startAddr uint16, valuesStr string) error {
	values, err := parseWriteValues(spec, valuesStr)
	if err != nil {
		return err
	}
	var quantity uint
	switch spec.valueType {
	case valueUint32, valueInt32, valueFloat32:
		quantity = uint(len(values.u32) * 2)
		if spec.valueType == valueFloat32 {
			quantity = uint(len(values.f32) * 2)
		}
	case valueUint16, valueInt16:
		quantity = uint(len(values.u16))
	default:
		return fmt.Errorf("unsupported value type %q", spec.valueType)
	}
	if quantity == 0 {
		return fmt.Errorf("value is required for write")
	}
	if quantity > maxRegisterWriteQty {
		return fmt.Errorf("write register quantity must be <= %d", maxRegisterWriteQty)
	}
	return validateAddressRange(startAddr, uint16(quantity))
}

func writeRegisters(client *MBClient, spec registerSpec, startAddr uint16, valuesStr string) error {
	values, err := parseWriteValues(spec, valuesStr)
	if err != nil {
		return err
	}
	switch spec.valueType {
	case valueUint32, valueInt32:
		err = client.WriteMultiU32(startAddr, values.u32...)
	case valueFloat32:
		err = client.WriteFloats(startAddr, values.f32...)
	case valueUint16, valueInt16:
		if len(values.u16) == 1 && spec.writeMode == writeModeAuto {
			err = client.WriteU16(startAddr, values.u16[0])
		} else {
			err = client.WriteMultiU16(startAddr, values.u16...)
		}
	default:
		return fmt.Errorf("unsupported value type %q", spec.valueType)
	}
	if err != nil {
		return fmt.Errorf("write registers error: %w", err)
	}
	return nil
}

func readRegisters(client *MBClient, spec registerSpec, startAddr uint16, cnt uint16) error {
	var results []byte
	var err error
	switch spec.function {
	case functionHoldingRegs:
		results, err = client.client.ReadHoldingRegisters(startAddr, cnt)
	case functionCoils:
		results, err = client.client.ReadCoils(startAddr, cnt)
	case functionDiscreteInputs:
		results, err = client.client.ReadDiscreteInputs(startAddr, cnt)
	case functionInputRegs:
		results, err = client.client.ReadInputRegisters(startAddr, cnt)
	default:
		return fmt.Errorf("unsupported function code %q", spec.function)
	}
	if err != nil {
		return fmt.Errorf("read registers error: %w", err)
	}
	return logReadResults(spec, startAddr, cnt, results)
}

func showHelp() {
	fmt.Println("Modbus 工具 - 读写 Modbus 设备寄存器")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Println("  mb [选项]")
	fmt.Println()
	fmt.Println("选项:")
	fmt.Println("  -h string        Modbus服务器地址")
	fmt.Println("                   格式1（TCP/IP）: 127.0.0.1:502")
	fmt.Println("                   格式2（串口）: /dev/ttyS9:9600:8N1")
	fmt.Println()
	fmt.Println("  -t string        寄存器类型（默认：3u）")
	fmt.Println("                   格式: [功能码][数据类型][字节序][写操作]")
	fmt.Println("                   功能码: 1(线圈), 2(离散输入), 3(保持寄存器), 4(输入寄存器)")
	fmt.Println("                   数据类型: U(uint32), S(int32), u(uint16) s(int16), F(float32)")
	fmt.Println("                   字节序: L(小端字节序), 默认大端字节序")
	fmt.Println("                   写操作: 仅功能码3支持，W 固定使用 0x10，w 单个寄存器用 0x06、多个用 0x10")
	fmt.Println("                   示例:")
	fmt.Println("                     -t 3U     读取保持寄存器为 uint32")
	fmt.Println("                     -t 3UW    用 0x10 写入保持寄存器为 uint32")
	fmt.Println("                     -t 3uw    写入 uint16，单个值用 0x06，多个值用 0x10")
	fmt.Println("                     -t 4FL    读取输入寄存器为 float32（小端）")
	fmt.Println("                     -t 3SW    写入保持寄存器为 int32")
	fmt.Println()
	fmt.Println("  -r string        起始地址（默认：0）")
	fmt.Println("                   支持十进制和十六进制（0x开头）")
	fmt.Println("                   示例: -r 10, -r 0x0A")
	fmt.Println()
	fmt.Println("  -c uint          读取寄存器/线圈数量（默认：1）")
	fmt.Println("                   对于32位数据类型必须为偶数，一个值占用2个寄存器")
	fmt.Println()
	fmt.Println("  -a uint          从站ID/设备地址（默认：1）")
	fmt.Println()
	fmt.Println("  -v string        要写入的值")
	fmt.Println("                   支持十进制和十六进制（0x开头）")
	fmt.Println("                   多个值用逗号分隔")
	fmt.Println("                   示例: -v 3.14, -v 0x1234, -v 10,20,30")
	fmt.Println()
	fmt.Println("  -m               重复读取模式, 默认读取一次")
	fmt.Println("  -M uint          重复读取间隔时间(毫秒), 默认1000ms")
}
