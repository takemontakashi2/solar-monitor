package main

import "fmt"

func echonetPropertyName(eoj [3]byte, epc byte) string {
	class := uint16(eoj[0])<<8 | uint16(eoj[1])
	common := map[byte]string{
		0x80: "動作状態",
		0x81: "設置場所",
		0x82: "規格Version情報",
		0x83: "識別番号",
		0x86: "メーカー異常コード",
		0x88: "異常発生状態",
		0x89: "異常内容",
		0x8a: "メーカーコード",
		0x8b: "事業場コード",
		0x8d: "製造番号",
		0x93: "遠隔操作設定",
		0x97: "現在時刻設定",
		0x98: "現在年月日設定",
	}
	if name, ok := common[epc]; ok {
		return name
	}
	switch class {
	case 0x0279:
		switch epc {
		case 0xe0:
			return "瞬時発電電力計測値"
		case 0xe1:
			return "積算発電電力量計測値"
		}
	case 0x027d:
		switch epc {
		case 0xa1:
			return "AC実効容量(放電)"
		case 0xa2:
			return "AC充電可能容量"
		case 0xa3:
			return "AC放電可能容量"
		case 0xa4:
			return "AC充電可能量"
		case 0xa5:
			return "AC放電可能量"
		case 0xcf:
			return "運転動作状態"
		case 0xda:
			return "運転モード設定"
		case 0xdb:
			return "系統連系状態"
		case 0xe2:
			return "蓄電残量1"
		case 0xe3:
			return "蓄電残量2"
		case 0xe4:
			return "蓄電残量3"
		case 0xe6:
			return "蓄電池タイプ"
		case 0xeb:
			return "充電電力設定値"
		case 0xec:
			return "放電電力設定値"
		}
	}
	return ""
}

func describeECHONETValue(eoj [3]byte, epc byte, data []byte) string {
	if len(data) == 0 {
		return ""
	}
	class := uint16(eoj[0])<<8 | uint16(eoj[1])
	value := unsignedBE(data)
	if epc == 0x80 {
		switch data[0] {
		case 0x30:
			return "ON"
		case 0x31:
			return "OFF"
		}
	}
	if class == 0x027d {
		switch epc {
		case 0xcf, 0xda:
			switch data[0] {
			case 0x42:
				return "充電"
			case 0x43:
				return "放電"
			case 0x44:
				return "待機"
			case 0x46:
				return "自動"
			}
		case 0xe4:
			return fmt.Sprintf("%d%%", value)
		case 0xeb, 0xec:
			return fmt.Sprintf("%d W", value)
		}
	}
	if class == 0x0279 {
		switch epc {
		case 0xe0:
			return fmt.Sprintf("%d W", value)
		case 0xe1:
			return fmt.Sprintf("%.3f kWh", float64(value)*0.001)
		}
	}
	return ""
}
