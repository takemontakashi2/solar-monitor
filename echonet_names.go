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
		names := map[byte]string{
			0xa0: "出力制御設定1",
			0xa1: "出力制御設定2",
			0xa2: "余剰買取制御機能設定",
			0xb0: "出力制御スケジュール",
			0xb1: "次回アクセス日時",
			0xb2: "余剰買取制御機能タイプ",
			0xb3: "出力変化時間設定値",
			0xb4: "上限クリップ設定値",
			0xc0: "運転力率設定値",
			0xc1: "FIT契約タイプ",
			0xc2: "自家消費タイプ",
			0xc3: "設備認定容量",
			0xc4: "換算係数",
			0xd0: "系統連系状態",
			0xd1: "出力抑制状態",
			0xe0: "瞬時発電電力計測値",
			0xe1: "積算発電電力量計測値",
			0xe3: "積算売電電力量計測値",
			0xe5: "発電電力制限設定1",
			0xe6: "発電電力制限設定2",
			0xe7: "売電電力制限設定",
			0xe8: "定格発電電力値(系統連系時)",
			0xe9: "定格発電電力値(独立時)",
		}
		if name, ok := names[epc]; ok {
			return name
		}
	case 0x027d:
		names := map[byte]string{
			0xa0: "AC実効容量 (充電)",
			0xa1: "AC実効容量 (放電)",
			0xa2: "AC充電可能容量",
			0xa3: "AC放電可能容量",
			0xa4: "AC充電可能量",
			0xa5: "AC放電可能量",
			0xa6: "AC充電上限設定",
			0xa7: "AC放電下限設定",
			0xa8: "AC積算充電電力量計測値",
			0xa9: "AC積算放電電力量計測値",
			0xaa: "AC充電量設定値",
			0xab: "AC放電量設定値",
			0xc1: "充電方式",
			0xc2: "放電方式",
			0xc7: "AC定格電力量",
			0xc8: "最小最大充電電力値",
			0xc9: "最小最大放電電力値",
			0xca: "最小最大充電電流値",
			0xcb: "最小最大放電電流値",
			0xcc: "再連系許可設定",
			0xcd: "運転許可設定",
			0xce: "自立運転許可設定",
			0xcf: "運転動作状態",
			0xd0: "定格電力量",
			0xd1: "定格容量",
			0xd2: "定格電圧",
			0xd3: "瞬時充放電電力計測値",
			0xd4: "瞬時充放電電流計測値",
			0xd5: "瞬時充放電電圧計測値",
			0xd6: "積算放電電力量計測値",
			0xd8: "積算充電電力量計測値",
			0xda: "運転モード設定",
			0xdb: "系統連系状態",
			0xdc: "最小最大充電電力値 (独立時)",
			0xdd: "最小最大放電電力値 (独立時)",
			0xde: "最小最大充電電流値 (独立時)",
			0xdf: "最小最大放電電流値 (独立時)",
			0xe0: "充放電量設定値1",
			0xe1: "充放電量設定値2",
			0xe2: "蓄電残量1",
			0xe3: "蓄電残量2",
			0xe4: "蓄電残量3",
			0xe5: "劣化状態",
			0xe6: "蓄電池タイプ",
			0xe7: "充電量設定値1",
			0xe8: "放電量設定値1",
			0xe9: "充電量設定値2",
			0xea: "放電量設定値2",
			0xeb: "充電電力設定値",
			0xec: "放電電力設定値",
			0xed: "充電電流設定値",
			0xee: "放電電流設定値",
			0xef: "定格電圧 (独立時)",
		}
		if name, ok := names[epc]; ok {
			return name
		}
	case 0x0288:
		names := map[byte]string{
			0xc0: "Bルート識別番号",
			0xd0: "1分積算電力量計測値 (正方向、逆方向)",
			0xe0: "積算電力量計測値 (正方向)",
			0xe2: "積算電力量計測値履歴1 (正方向)",
			0xe3: "積算電力量計測値 (逆方向)",
			0xe4: "積算電力量計測値履歴1 (逆方向)",
			0xe7: "瞬時電力計測値",
			0xe8: "瞬時電流計測値",
			0xea: "定時積算電力量計測値 (正方向)",
			0xeb: "定時積算電力量計測値 (逆方向)",
			0xec: "積算電力量計測値履歴2 (正方向、逆方向)",
			0xee: "積算電力量計測値履歴3 (正方向、逆方向)",
		}
		if name, ok := names[epc]; ok {
			return name
		}
	case 0x05ff:
		names := map[byte]string{
			0xc0: "コントローラID",
			0xc1: "管理台数",
			0xc2: "機器情報リスト",
		}
		if name, ok := names[epc]; ok {
			return name
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
		case 0xd3:
			return fmt.Sprintf("%d W", signedBE(data))
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
	if class == 0x0288 {
		switch epc {
		case 0xe7:
			return fmt.Sprintf("%d W", signedBE(data))
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
