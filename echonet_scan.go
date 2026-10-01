package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"sort"
	"time"
)

const echonetMulticastAddr = "224.0.23.0:3610"

type ECHONETScanner struct {
	addr        string
	tid         uint16
	readTimeout time.Duration
}

type ECHONETNode struct {
	Addr string
	EOJ  [3]byte
}

type ECHONETObject struct {
	EOJ           [3]byte
	Name          string
	GetProperties []byte
}

func NewECHONETScanner(addr string) *ECHONETScanner {
	return &ECHONETScanner{addr: net.JoinHostPort(addr, "3610"), tid: uint16(time.Now().UnixNano()), readTimeout: 1200 * time.Millisecond}
}

func DiscoverECHONETNodes(ctx context.Context) ([]ECHONETNode, error) {
	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	deadline := time.Now().Add(3 * time.Second)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	_ = conn.SetDeadline(deadline)

	addr, err := net.ResolveUDPAddr("udp4", echonetMulticastAddr)
	if err != nil {
		return nil, err
	}
	tid := uint16(time.Now().UnixNano())
	req := []byte{
		0x10, 0x81, byte(tid >> 8), byte(tid),
		0x05, 0xff, 0x01,
		0x0e, 0xf0, 0x01,
		0x62,
		0x01,
		0xd6, 0x00,
	}
	if _, err := conn.WriteTo(req, addr); err != nil {
		return nil, err
	}

	nodesByAddr := map[string]ECHONETNode{}
	buf := make([]byte, 1500)
	for {
		n, remote, err := conn.ReadFrom(buf)
		if err != nil {
			if len(nodesByAddr) > 0 {
				break
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break
			}
			return nil, err
		}
		udpAddr, ok := remote.(*net.UDPAddr)
		if !ok {
			continue
		}
		data, err := echonetPropertyData(buf[:n], 0xd6)
		if err != nil || len(data) < 4 {
			continue
		}
		nodesByAddr[udpAddr.IP.String()] = ECHONETNode{Addr: udpAddr.IP.String(), EOJ: [3]byte{data[1], data[2], data[3]}}
	}

	nodes := make([]ECHONETNode, 0, len(nodesByAddr))
	for _, node := range nodesByAddr {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Addr < nodes[j].Addr })
	return nodes, nil
}

func PrintECHONETDiscovery(ctx context.Context, w io.Writer) error {
	nodes, err := DiscoverECHONETNodes(ctx)
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		fmt.Fprintln(w, "no nodes found")
		return nil
	}
	for _, node := range nodes {
		fmt.Fprintf(w, "%s  %s  %s\n", node.Addr, formatEOJ(node.EOJ), echonetClassName(node.EOJ))
	}
	return nil
}

func (s *ECHONETScanner) Scan(ctx context.Context) ([]ECHONETObject, error) {
	objects, err := s.readInstanceList(ctx)
	if err != nil {
		objects = commonECHONETObjects()
	}
	for i := range objects {
		props, err := s.readPropertyMap(ctx, objects[i].EOJ)
		if err != nil {
			continue
		}
		objects[i].GetProperties = props
	}
	return objects, nil
}

func (s *ECHONETScanner) Print(ctx context.Context, w io.Writer) error {
	objects, err := s.Scan(ctx)
	if err != nil {
		return err
	}
	if len(objects) == 0 {
		fmt.Fprintln(w, "no objects found")
		return nil
	}
	for _, object := range objects {
		fmt.Fprintf(w, "%s  %s\n", formatEOJ(object.EOJ), object.Name)
		if len(object.GetProperties) == 0 {
			fmt.Fprintln(w, "  get: unknown")
			continue
		}
		fmt.Fprintf(w, "  get: %s\n", formatEPCList(object.GetProperties))
	}
	return nil
}

func (s *ECHONETScanner) readInstanceList(ctx context.Context) ([]ECHONETObject, error) {
	data, err := s.readRawProperty(ctx, [3]byte{0x0e, 0xf0, 0x01}, 0xd6)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("empty instance list")
	}
	count := int(data[0])
	objects := make([]ECHONETObject, 0, count)
	for i := 0; i < count; i++ {
		pos := 1 + i*3
		if pos+3 > len(data) {
			return nil, fmt.Errorf("truncated instance list")
		}
		eoj := [3]byte{data[pos], data[pos+1], data[pos+2]}
		objects = append(objects, ECHONETObject{EOJ: eoj, Name: echonetClassName(eoj)})
	}
	return objects, nil
}

func (s *ECHONETScanner) readPropertyMap(ctx context.Context, eoj [3]byte) ([]byte, error) {
	data, err := s.readRawProperty(ctx, eoj, 0x9f)
	if err != nil {
		return nil, err
	}
	return parseECHONETPropertyMap(data)
}

func (s *ECHONETScanner) readRawProperty(ctx context.Context, eoj [3]byte, epc byte) ([]byte, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "udp4", s.addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	deadline := time.Now().Add(s.readTimeout)
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
		0x01,
		epc, 0x00,
	}
	if _, err := conn.Write(req); err != nil {
		return nil, err
	}
	buf := make([]byte, 1500)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}
	return echonetPropertyData(buf[:n], epc)
}

func parseECHONETPropertyMap(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty property map")
	}
	count := int(data[0])
	props := []byte{}
	if count <= 15 {
		if len(data) < count+1 {
			return nil, fmt.Errorf("truncated property map")
		}
		props = append(props, data[1:1+count]...)
	} else {
		if len(data) < 17 {
			return nil, fmt.Errorf("truncated bitmap property map")
		}
		for group, bits := range data[1:17] {
			for bit := 0; bit < 8; bit++ {
				if bits&(1<<bit) != 0 {
					props = append(props, byte(0x80+group+bit*0x10))
				}
			}
		}
	}
	sort.Slice(props, func(i, j int) bool { return props[i] < props[j] })
	return props, nil
}

func formatEOJ(eoj [3]byte) string {
	return fmt.Sprintf("%02x%02x%02x", eoj[0], eoj[1], eoj[2])
}

func formatEPCList(props []byte) string {
	encoded := make([]byte, 0, len(props)*2)
	for i, prop := range props {
		if i > 0 {
			encoded = append(encoded, ' ')
		}
		encoded = hex.AppendEncode(encoded, []byte{prop})
	}
	return string(encoded)
}

func echonetClassName(eoj [3]byte) string {
	class := uint16(eoj[0])<<8 | uint16(eoj[1])
	switch class {
	case 0x0279:
		return "太陽光発電"
	case 0x027d:
		return "蓄電池"
	case 0x0287:
		return "分電盤メータリング"
	case 0x0288:
		return "低圧スマート電力量メータ"
	case 0x0ef0:
		return "ノードプロファイル"
	default:
		return "unknown"
	}
}

func commonECHONETObjects() []ECHONETObject {
	classes := [][3]byte{
		{0x02, 0x79, 0x01},
		{0x02, 0x79, 0x02},
		{0x02, 0x7d, 0x01},
		{0x02, 0x7d, 0x02},
		{0x02, 0x87, 0x01},
		{0x02, 0x88, 0x01},
		{0x0e, 0xf0, 0x01},
	}
	objects := make([]ECHONETObject, 0, len(classes))
	for _, eoj := range classes {
		objects = append(objects, ECHONETObject{EOJ: eoj, Name: echonetClassName(eoj)})
	}
	return objects
}

type RawECHONETPacket struct {
	Remote string
	Data   []byte
}

func DumpRawECHONETDiscovery(ctx context.Context, w io.Writer) error {
	packets, err := collectRawECHONET(ctx, "", [3]byte{0x0e, 0xf0, 0x01}, 0xd6)
	if err != nil {
		return err
	}
	printRawECHONETPackets(w, packets)
	return nil
}

func DumpRawECHONETTarget(ctx context.Context, w io.Writer, addr string) error {
	targets := []struct {
		label string
		eoj   [3]byte
		epc   byte
	}{
		{label: "node instance list", eoj: [3]byte{0x0e, 0xf0, 0x01}, epc: 0xd6},
		{label: "node get property map", eoj: [3]byte{0x0e, 0xf0, 0x01}, epc: 0x9f},
		{label: "solar get property map", eoj: [3]byte{0x02, 0x79, 0x01}, epc: 0x9f},
		{label: "battery get property map", eoj: [3]byte{0x02, 0x7d, 0x01}, epc: 0x9f},
		{label: "meter get property map", eoj: [3]byte{0x02, 0x88, 0x01}, epc: 0x9f},
	}
	for _, target := range targets {
		fmt.Fprintf(w, "# %s %s EPC %02x\n", target.label, formatEOJ(target.eoj), target.epc)
		packets, err := collectRawECHONET(ctx, addr, target.eoj, target.epc)
		if err != nil {
			fmt.Fprintf(w, "error: %v\n", err)
			continue
		}
		printRawECHONETPackets(w, packets)
	}
	return nil
}

func collectRawECHONET(ctx context.Context, addr string, eoj [3]byte, epc byte) ([]RawECHONETPacket, error) {
	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	deadline := time.Now().Add(1200 * time.Millisecond)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	_ = conn.SetDeadline(deadline)

	target := echonetMulticastAddr
	if addr != "" {
		target = net.JoinHostPort(addr, "3610")
	}
	udpAddr, err := net.ResolveUDPAddr("udp4", target)
	if err != nil {
		return nil, err
	}
	tid := uint16(time.Now().UnixNano())
	req := []byte{
		0x10, 0x81, byte(tid >> 8), byte(tid),
		0x05, 0xff, 0x01,
		eoj[0], eoj[1], eoj[2],
		0x62,
		0x01,
		epc, 0x00,
	}
	if _, err := conn.WriteTo(req, udpAddr); err != nil {
		return nil, err
	}

	packets := []RawECHONETPacket{}
	buf := make([]byte, 1500)
	for {
		n, remote, err := conn.ReadFrom(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break
			}
			return nil, err
		}
		data := make([]byte, n)
		copy(data, buf[:n])
		packets = append(packets, RawECHONETPacket{Remote: remote.String(), Data: data})
	}
	return packets, nil
}

func printRawECHONETPackets(w io.Writer, packets []RawECHONETPacket) {
	if len(packets) == 0 {
		fmt.Fprintln(w, "no response")
		return
	}
	for _, packet := range packets {
		fmt.Fprintf(w, "%s  %s\n", packet.Remote, hex.EncodeToString(packet.Data))
	}
}
