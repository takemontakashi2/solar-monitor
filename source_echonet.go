package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

type ECHONETSource struct {
	addr            string
	fields          map[string]ECHONETProperty
	tid             uint16
	fullScan        bool
	initialScanDone bool
}

type ECHONETProperty struct {
	EOJ   [3]byte
	EPC   byte
	Scale float64
}

func NewECHONETSource(addr string, fields map[string]ECHONETProperty, fullScan bool) *ECHONETSource {
	return &ECHONETSource{addr: net.JoinHostPort(addr, "3610"), fields: fields, tid: uint16(time.Now().UnixNano()), fullScan: fullScan}
}

func (s *ECHONETSource) Read(ctx context.Context) (Sample, error) {
	now := time.Now()
	result := Sample{Time: now, Source: "echonet"}
	useFullScan := s.fullScan || !s.initialScanDone
	properties, err := s.readAllProperties(ctx, now, useFullScan)
	if err != nil {
		return Sample{}, err
	}
	result.Properties = properties
	s.initialScanDone = true
	for _, prop := range properties {
		s.applyKnownProperty(&result, prop)
	}
	return result, nil
}

func (s *ECHONETSource) readAllProperties(ctx context.Context, t time.Time, fullScan bool) ([]SampleProperty, error) {
	conn, udpAddr, err := s.openConn()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	objects, err := s.readObjects(ctx, conn, udpAddr)
	if err != nil {
		return nil, err
	}
	props := []SampleProperty{}
	for _, object := range objects {
		candidates := echonetReadCandidates(object, fullScan)
		if len(candidates) == 0 {
			propertyMap, err := s.readPropertyMap(ctx, conn, udpAddr, object)
			if err != nil {
				continue
			}
			candidates = propertyMap
		}
		for _, batch := range propertyBatches(candidates, 1) {
			values, err := s.readRawProperties(ctx, conn, udpAddr, object, batch)
			if err != nil {
				continue
			}
			for _, epc := range batch {
				data, ok := values[epc]
				if !ok {
					continue
				}
				props = append(props, makeSampleProperty(t, object, epc, data))
			}
		}
	}
	return props, nil
}

func echonetReadCandidates(eoj [3]byte, fullScan bool) []byte {
	class := uint16(eoj[0])<<8 | uint16(eoj[1])
	common := []byte{0x80, 0x81, 0x82, 0x83, 0x86, 0x88, 0x89, 0x8a, 0x8b, 0x8d, 0x97, 0x98}
	switch class {
	case 0x0279:
		normal := append([]byte{}, common...)
		normal = append(normal, 0xe0, 0xe1, 0xe8)
		if !fullScan {
			return uniqueBytes(normal)
		}
		full := append(normal, 0x93, 0xa0, 0xa2, 0xb0, 0xb1, 0xb2, 0xb4, 0xc1, 0xc2, 0xc3, 0xc4, 0xd0, 0xd1, 0xf1, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8)
		return uniqueBytes(full)
	case 0x027d:
		normal := append([]byte{}, common...)
		normal = append(normal, 0xd3, 0xd6, 0xd8, 0xe3, 0xe4, 0xcf, 0xda, 0xdb, 0xeb, 0xec)
		if !fullScan {
			return uniqueBytes(normal)
		}
		full := append(normal, 0x93, 0xa0, 0xa1, 0xa2, 0xa3, 0xa4, 0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xab, 0xc1, 0xc2, 0xc7, 0xd0, 0xd1, 0xd2, 0xe5, 0xe6, 0xf0, 0xf1, 0xf3, 0xf4, 0xf5, 0xf6)
		return uniqueBytes(full)
	case 0x05ff:
		normal := append([]byte{}, common...)
		normal = append(normal, 0x8c, 0xf1, 0xf2, 0xf3, 0xf4, 0xf5)
		return uniqueBytes(normal)
	default:
		return nil
	}
}

func uniqueBytes(values []byte) []byte {
	seen := map[byte]struct{}{}
	out := make([]byte, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (s *ECHONETSource) openConn() (net.PacketConn, *net.UDPAddr, error) {
	conn, err := net.ListenPacket("udp4", ":3610")
	if err != nil {
		return nil, nil, err
	}
	udpAddr, err := net.ResolveUDPAddr("udp4", s.addr)
	if err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	return conn, udpAddr, nil
}

func (s *ECHONETSource) readObjects(ctx context.Context, conn net.PacketConn, addr *net.UDPAddr) ([][3]byte, error) {
	data, err := s.readRawProperty(ctx, conn, addr, [3]byte{0x0e, 0xf0, 0x01}, 0xd6)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("empty instance list")
	}
	objects := make([][3]byte, 0, int(data[0]))
	for i := 0; i < int(data[0]); i++ {
		pos := 1 + i*3
		if pos+3 > len(data) {
			return nil, fmt.Errorf("truncated instance list")
		}
		objects = append(objects, [3]byte{data[pos], data[pos+1], data[pos+2]})
	}
	return objects, nil
}

func (s *ECHONETSource) readPropertyMap(ctx context.Context, conn net.PacketConn, addr *net.UDPAddr, eoj [3]byte) ([]byte, error) {
	data, err := s.readRawProperty(ctx, conn, addr, eoj, 0x9f)
	if err != nil {
		return nil, err
	}
	return parseECHONETPropertyMap(data)
}

func (s *ECHONETSource) readRawProperty(ctx context.Context, conn net.PacketConn, addr *net.UDPAddr, eoj [3]byte, epc byte) ([]byte, error) {
	values, err := s.readRawProperties(ctx, conn, addr, eoj, []byte{epc})
	if err != nil {
		return nil, err
	}
	data, ok := values[epc]
	if !ok {
		return nil, fmt.Errorf("EPC 0x%02x not found", epc)
	}
	return data, nil
}

func (s *ECHONETSource) readRawProperties(ctx context.Context, conn net.PacketConn, addr *net.UDPAddr, eoj [3]byte, epcs []byte) (map[byte][]byte, error) {
	deadline := time.Now().Add(1200 * time.Millisecond)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	_ = conn.SetDeadline(deadline)

	s.tid++
	req := []byte{
		0x10, 0x81, byte(s.tid >> 8), byte(s.tid),
		0x05, 0xff, 0x01,
		eoj[0], eoj[1], eoj[2],
		0x62,
		byte(len(epcs)),
	}
	for _, epc := range epcs {
		req = append(req, epc, 0x00)
	}
	if _, err := conn.WriteTo(req, addr); err != nil {
		return nil, err
	}
	buf := make([]byte, 1500)
	for {
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			return nil, err
		}
		values, err := echonetPropertiesDataFor(buf[:n], eoj)
		if err == nil {
			return values, nil
		}
	}
}

func (s *ECHONETSource) applyKnownProperty(sample *Sample, prop SampleProperty) {
	if prop.Float == nil {
		return
	}
	key := strings.ToLower(prop.EOJ + ":" + prop.EPC)
	for name, configured := range s.fields {
		if strings.ToLower(formatEOJ(configured.EOJ)+":"+fmt.Sprintf("%02x", configured.EPC)) != key {
			continue
		}
		value := *prop.Float * configured.Scale
		switch name {
		case "pv":
			sample.PVWatts = floatPtr(value)
		case "load":
			sample.LoadWatts = floatPtr(value)
		case "grid":
			sample.GridWatts = floatPtr(value)
		case "today":
			sample.TodayKWh = floatPtr(value)
		}
	}
}

func makeSampleProperty(t time.Time, eoj [3]byte, epc byte, data []byte) SampleProperty {
	unsigned := unsignedBE(data)
	signed := signedBE(data)
	float := echonetNumericValue(eoj, epc, data)
	return SampleProperty{
		Time:        t,
		EOJ:         formatEOJ(eoj),
		EPC:         fmt.Sprintf("%02x", epc),
		Name:        echonetPropertyName(eoj, epc),
		Raw:         hex.EncodeToString(data),
		Unsigned:    &unsigned,
		Signed:      &signed,
		Float:       &float,
		Description: describeECHONETValue(eoj, epc, data),
	}
}

func echonetNumericValue(eoj [3]byte, epc byte, data []byte) float64 {
	class := uint16(eoj[0])<<8 | uint16(eoj[1])
	switch class {
	case 0x027d:
		if epc == 0xd3 {
			return float64(signedBE(data))
		}
	case 0x0288:
		if epc == 0xe7 {
			return float64(signedBE(data))
		}
	}
	return float64(unsignedBE(data))
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

func propertyBatches(props []byte, size int) [][]byte {
	filtered := []byte{}
	for _, prop := range props {
		if prop == 0x9d || prop == 0x9e || prop == 0x9f {
			continue
		}
		filtered = append(filtered, prop)
	}
	batches := [][]byte{}
	for len(filtered) > 0 {
		n := size
		if len(filtered) < n {
			n = len(filtered)
		}
		batch := make([]byte, n)
		copy(batch, filtered[:n])
		batches = append(batches, batch)
		filtered = filtered[n:]
	}
	return batches
}

func echonetPropertiesData(packet []byte) (map[byte][]byte, error) {
	if len(packet) < 12 || packet[0] != 0x10 || packet[1] != 0x81 {
		return nil, errors.New("invalid ECHONET Lite packet")
	}
	if packet[10] != 0x72 {
		return nil, fmt.Errorf("unexpected ESV 0x%02x", packet[10])
	}
	opc := int(packet[11])
	pos := 12
	values := map[byte][]byte{}
	for i := 0; i < opc; i++ {
		if pos+2 > len(packet) {
			return nil, errors.New("truncated property header")
		}
		epc := packet[pos]
		pdc := int(packet[pos+1])
		pos += 2
		if pos+pdc > len(packet) {
			return nil, errors.New("truncated property data")
		}
		data := make([]byte, pdc)
		copy(data, packet[pos:pos+pdc])
		values[epc] = data
		pos += pdc
	}
	return values, nil
}

func echonetPropertiesDataFor(packet []byte, eoj [3]byte) (map[byte][]byte, error) {
	if len(packet) < 12 {
		return nil, errors.New("invalid ECHONET Lite packet")
	}
	if packet[4] != eoj[0] || packet[5] != eoj[1] || packet[6] != eoj[2] {
		return nil, fmt.Errorf("unexpected SEOJ %02x%02x%02x", packet[4], packet[5], packet[6])
	}
	return echonetPropertiesData(packet)
}
