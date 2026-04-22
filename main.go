package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"
)

func stringParse(str string) (uint, []float64) {
	if str == "" {
		return 0, nil
	}
	// 先看下是否是单个值
	if !strings.Contains(str, ",") {
		// 单个值
		// 是否为16进制
		if strings.HasPrefix(str, "0x") || strings.HasPrefix(str, "0X") {
			v, err := strconv.ParseUint(str[2:], 16, 64)
			if err != nil {
				log.Println("ParseUint error: ", err)
				return 0, nil
			}
			return 1, []float64{float64(v)}
		} else {
			v, err := strconv.ParseFloat(str, 64)
			if err != nil {
				log.Println("ParseFloat error: ", err)
				return 0, nil
			}
			return 1, []float64{v}
		}
	} else {
		// 多个值
		parts := strings.Split(str, ",")
		var values []float64
		for _, s := range parts {
			// 是否为16进制
			if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
				v, err := strconv.ParseUint(s[2:], 16, 64)
				if err != nil {
					log.Println("ParseUint error: ", err)
					return 0, nil
				}
				values = append(values, float64(v))
			} else {
				v, err := strconv.ParseFloat(s, 64)
				if err != nil {
					log.Println("ParseFloat error: ", err)
					return 0, nil
				}
				values = append(values, v)
			}
		}
		return uint(len(values)), values
	}
}

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

	// 构建客户端
	client := NewMBClient(address, byte(slaveId))
	if client == nil {
		return
	}
	if strings.Contains(regType, "L") {
		client.SetByteSequence(true)
	}

	startAddr, err := strconv.ParseUint(startAddrStr, 0, 0)
	if err != nil {
		fmt.Printf("%v", err)
		return
	}

	var registerType = "3"
	if regType[0] == '4' {
		registerType = "4"
	} else if regType[0] == '1' {
		registerType = "1"
	} else if regType[0] == '2' {
		registerType = "2"
	}

	if strings.Contains(regType, "W") {
		varCnt, values := stringParse(valuesStr)
		if varCnt == 0 {
			log.Fatalf("value string: %s parse error\n", valuesStr)
			return
		}
		if strings.Contains(regType, "F") {
			// 要写入浮点数
			err = client.WriteFloats(uint16(startAddr), values...)
			if err != nil {
				log.Println("WriteMultipleRegisters error: ", err)
				return
			}
		} else if strings.Contains(regType, "U") || strings.Contains(regType, "S") {
			// 要写入U32或者S32
			err = client.WriteMultiU32(uint16(startAddr), values...)
			if err != nil {
				log.Println("WriteMultipleRegisters error: ", err)
				return
			}
		} else {
			if varCnt == 1 {
				// 写单个寄存器
				v := uint16(values[0])
				err = client.WriteU16(uint16(startAddr), v)
				if err != nil {
					log.Println("WriteSingleRegister error: ", err)
					return
				}
				return
			} else {
				datas := make([]uint16, varCnt)
				for i, v := range values {
					datas[i] = uint16(v)
				}
				err = client.WriteMultiU16(uint16(startAddr), datas...)
				if err != nil {
					log.Println("WriteMultipleRegisters error: ", err)
					return
				}
			}
		}
	} else {
		for {
			var results []byte
			if registerType == "3" {
				results, err = client.client.ReadHoldingRegisters(uint16(startAddr), uint16(cnt))
			} else if registerType == "1" {
				results, err = client.client.ReadCoils(uint16(startAddr), uint16(cnt))
			} else if registerType == "2" {
				results, err = client.client.ReadDiscreteInputs(uint16(startAddr), uint16(cnt))
			} else {
				results, err = client.client.ReadInputRegisters(uint16(startAddr), uint16(cnt))
			}

			if err != nil {
				log.Println("ReadHoldingRegisters error: ", err)
				return
			}

			var dataType = regType[1:]
			// 功能码 1(线圈) 和 2(离散输入) 返回的是位数据，每字节 8 个位
			if registerType == "1" || registerType == "2" {
				var j uint64 = 0
				for i := 0; i < len(results) && j < uint64(cnt); i++ {
					for b := 0; b < 8 && j < uint64(cnt); b++ {
						value := (results[i] >> b) & 1
						log.Printf("addr: 0x%02X:%d  value: %d (%s)", startAddr+j, startAddr+j, value, map[byte]string{0: "OFF", 1: "ON"}[value])
						j++
					}
				}
			} else if dataType == "U" {
				// uint32
				var j uint64 = 0
				for i := 0; i < len(results); i += 4 {
					value := binary.BigEndian.Uint32(results[i:])
					log.Printf("addr: 0x%02X:%d  value: 0x%08X : %d", startAddr+j, startAddr+j, value, value)
					j += 2
				}
			} else if dataType == "UL" {
				// uint32 little endian
				var j uint64 = 0
				for i := 0; i < len(results); i += 4 {
					l := binary.LittleEndian.Uint16(results[i:])
					h := binary.LittleEndian.Uint16(results[i+2:])
					value := uint32(h)<<16 | uint32(l)
					log.Printf("addr: 0x%02X:%d  value: 0x%08X : %d", startAddr+j, startAddr+j, value, value)
					j += 2
				}
			} else if dataType == "S" {
				// int32
				var j uint64 = 0
				for i := 0; i < len(results); i += 4 {
					value := int32(binary.BigEndian.Uint32(results[i:]))
					log.Printf("addr: 0x%02X:%d  value: 0x%08X : %d", startAddr+j, startAddr+j, value, value)
					j += 2
				}
			} else if dataType == "SL" {
				// int32 little endian
				var j uint64 = 0
				for i := 0; i < len(results); i += 4 {
					l := binary.LittleEndian.Uint16(results[i:])
					h := binary.LittleEndian.Uint16(results[i+2:])
					value := int32(uint32(h)<<16 | uint32(l))
					log.Printf("addr: 0x%02X:%d  value: 0x%08X : %d", startAddr+j, startAddr+j, value, value)
					j += 2
				}
			} else if dataType == "s" {
				// int16
				var j uint64 = 0
				for i := 0; i < len(results); i += 2 {
					value := int16(binary.BigEndian.Uint16(results[i:]))
					log.Printf("addr: 0x%02X:%d  value: %02X : %d", startAddr+j, startAddr+j, value, value)
					j++
				}
			} else if dataType == "F" {
				var j uint64 = 0
				for i := 0; i < len(results); i += 4 {
					value := math.Float32frombits(binary.BigEndian.Uint32(results[i:]))
					log.Printf("addr: 0x%02X:%d  value: %f", startAddr+j, startAddr+j, value)
					j += 2
				}
			} else if dataType == "FL" {
				var j uint64 = 0
				for i := 0; i < len(results); i += 4 {
					l := binary.BigEndian.Uint16(results[i:])
					h := binary.BigEndian.Uint16(results[i+2:])
					var data = uint32(h)<<16 | uint32(l)
					f := math.Float32frombits(data)
					fmt.Printf("Addr: 0x%02x:%d, value: %g\n", startAddr+j, startAddr+j, f)
					j += 2
				}
			} else {
				var j uint64 = 0
				for i := 0; i < len(results); i += 2 {
					value := binary.BigEndian.Uint16(results[i:])
					log.Printf("addr: 0x%02X:%d  value: 0x%02X : %d", startAddr+j, startAddr+j, value, value)
					j++
				}
			}
			if !multiRead {
				break
			} else {
				select {
				case <-time.After(time.Duration(multiReadTime) * time.Millisecond):
				}
			}
		}
	}
}

// 添加显示帮助的函数
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
	fmt.Println("                   格式: [功能码][数据类型][字节序]")
	fmt.Println("                   功能码: 1(线圈), 2(离散输入), 3(保持寄存器), 4(输入寄存器)")
	fmt.Println("                   数据类型: U(uint32), S(int32), u(uint16) s(int16), F(float32)")
	fmt.Println("                   字节序: L(小端字节序), 默认大端字节序")
	fmt.Println("                   写操作: 在类型后加 W")
	fmt.Println("                   示例:")
	fmt.Println("                     -t 3U     读取保持寄存器为 uint32")
	fmt.Println("                     -t 3UW    写入保持寄存器为 uint32")
	fmt.Println("                     -t 4FL    读取输入寄存器为 float32（小端）")
	fmt.Println("                     -t 3SW    写入保持寄存器为 int32")
	fmt.Println()
	fmt.Println("  -r string        起始地址（默认：0）")
	fmt.Println("                   支持十进制和十六进制（0x开头）")
	fmt.Println("                   示例: -r 10, -r 0x0A")
	fmt.Println()
	fmt.Println("  -c uint          读取数量（默认：1）")
	fmt.Println("                   对于32位数据类型，一个值占用2个寄存器")
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
