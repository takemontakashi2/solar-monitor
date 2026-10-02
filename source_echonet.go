package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

type ECHONETSource struct {
	addr   string
	fields map[string]ECHONETProperty
	tid    uint16
}

type ECHONETProperty struct {
	EOJ   [3]byte
	EPC   byte
	Scale float64
}

func NewECHONETSource(addr string, fields map[string]ECHONETProperty) *ECHONETSource {
	return &ECHONETSource{addr: net.JoinHostPort(addr, "3610"), fields: fields, tid: uint16(time.Now().UnixNano())}
}

func (s *ECHONETSource) Read(ctx context.Context) (Sample, error) {
	result := Sample{Time: time.Now(), Source: "echonet"}
	for name, prop := range s.fields {
		value, err := s.readProperty(ctx, prop, name == "grid")
		if err != nil {
			return Sample{}, fmt.Errorf("%s: %w", name, err)
		}
		switch name {
		case "pv":
			result.PVWatts = value
		case "load":
			result.LoadWatts = value
		case "grid":
			result.GridWatts = value
		case "today":
			result.TodayKWh = value
		}
	}
	return result, nil
}

func (s *ECHONETSource) readProperty(ctx context.Context, prop ECHONETProperty, signed bool) (float64, error) {
	conn, err := net.ListenPacket("udp4", ":3610")
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	udpAddr, err := net.ResolveUDPAddr("udp4", s.addr)
	if err != nil {
		return 0, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	}

	s.tid++
	req := []byte{
		0x10, 0x81, byte(s.tid >> 8), byte(s.tid),
		0x05, 0xff, 0x01,
		prop.EOJ[0], prop.EOJ[1], prop.EOJ[2],
		0x62,
		0x01,
		prop.EPC, 0x00,
	}
	if _, err := conn.WriteTo(req, udpAddr); err != nil {
		return 0, err
	}
	buf := make([]byte, 1500)
	n, _, err := conn.ReadFrom(buf)
	if err != nil {
		return 0, err
	}
	data, err := echonetPropertyData(buf[:n], prop.EPC)
	if err != nil {
		return 0, err
	}
	if signed {
		return float64(signedBE(data)) * prop.Scale, nil
	}
	return float64(unsignedBE(data)) * prop.Scale, nil
}

func echonetPropertyData(packet []byte, epc byte) ([]byte, error) {
	if len(packet) < 14 || packet[0] != 0x10 || packet[1] != 0x81 {
		return nil, errors.New("invalid ECHONET Lite packet")
	}
	if packet[10] != 0x72 {
		return nil, fmt.Errorf("unexpected ESV 0x%02x", packet[10])
	}
	opc := int(packet[11])
	pos := 12
	for i := 0; i < opc; i++ {
		if pos+2 > len(packet) {
			return nil, errors.New("truncated property header")
		}
		gotEPC := packet[pos]
		pdc := int(packet[pos+1])
		pos += 2
		if pos+pdc > len(packet) {
			return nil, errors.New("truncated property data")
		}
		if gotEPC == epc {
			return packet[pos : pos+pdc], nil
		}
		pos += pdc
	}
	return nil, fmt.Errorf("EPC 0x%02x not found", epc)
}

func unsignedBE(data []byte) uint64 {
	var value uint64
	for _, b := range data {
		value = value<<8 | uint64(b)
	}
	return value
}

func signedBE(data []byte) int64 {
	if len(data) == 0 {
		return 0
	}
	value := int64(unsignedBE(data))
	bits := uint(len(data) * 8)
	sign := int64(1) << (bits - 1)
	if value&sign != 0 {
		value -= int64(1) << bits
	}
	return value
}

func parseECHONETFields(raw string) (map[string]ECHONETProperty, error) {
	fields := map[string]ECHONETProperty{}
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		name, rest, ok := strings.Cut(item, "=")
		if !ok {
			return nil, fmt.Errorf("field %q must be name=EOJ:EPC[:scale]", item)
		}
		parts := strings.Split(rest, ":")
		if len(parts) < 2 || len(parts) > 3 {
			return nil, fmt.Errorf("field %q must be name=EOJ:EPC[:scale]", item)
		}
		eoj, err := parseHexBytes(parts[0], 3)
		if err != nil {
			return nil, fmt.Errorf("%s EOJ: %w", name, err)
		}
		epc, err := parseHexBytes(parts[1], 1)
		if err != nil {
			return nil, fmt.Errorf("%s EPC: %w", name, err)
		}
		scale := 1.0
		if len(parts) == 3 {
			scale, err = strconv.ParseFloat(parts[2], 64)
			if err != nil {
				return nil, fmt.Errorf("%s scale: %w", name, err)
			}
		}
		fields[name] = ECHONETProperty{EOJ: [3]byte{eoj[0], eoj[1], eoj[2]}, EPC: epc[0], Scale: scale}
	}
	return fields, nil
}

func parseHexBytes(raw string, want int) ([]byte, error) {
	raw = strings.TrimPrefix(strings.ToLower(strings.ReplaceAll(raw, "0x", "")), "#")
	raw = strings.ReplaceAll(raw, "-", "")
	raw = strings.ReplaceAll(raw, " ", "")
	if len(raw) != want*2 {
		return nil, fmt.Errorf("want %d hex bytes", want)
	}
	out := make([]byte, want)
	for i := 0; i < want; i++ {
		n, err := strconv.ParseUint(raw[i*2:i*2+2], 16, 8)
		if err != nil {
			return nil, err
		}
		out[i] = byte(n)
	}
	return out, nil
}
