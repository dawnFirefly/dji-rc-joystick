package rc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"
)

// // // // // // // // // //

func serveStream(t *testing.T, addr string, data []byte) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Skipf("cannot listen on %s: %v", addr, err)
	}

	go func() {
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				_, _ = c.Write(data)
				time.Sleep(100 * time.Millisecond)
			}(conn)
		}
	}()
	return ln
}

func serveMagic(t *testing.T, addr string) net.Listener {
	t.Helper()
	frame := buildVersionQuery(1)
	return serveStream(t, addr, frame[:])
}

func scanConfig(t *testing.T, port int) DiscoverConfigObj {
	t.Helper()
	config, err := (&DiscoverConfigObj{Port: port}).normalized()
	if err != nil {
		t.Fatal(err)
	}
	return config
}

// //

func TestDiscoverConfigDefaults(t *testing.T) {
	config, err := (*DiscoverConfigObj)(nil).normalized()
	if err != nil {
		t.Fatal(err)
	}
	if config.Port != DefaultPort ||
		config.BatchSize != scanBatchSize ||
		config.ScanDialTimeout != scanDialTimeout ||
		config.ProbeDialTimeout != probeDialTimeout ||
		config.ReadTimeout != probeReadTimeout ||
		config.MaxHosts != maxScanHosts ||
		config.MinPrefix != minScanPrefix ||
		len(config.KnownIPs) != len(knownIPs) {
		t.Fatalf("nil config not fully defaulted: %+v", config)
	}

	custom, err := (&DiscoverConfigObj{Port: 5000, BatchSize: 8}).normalized()
	if err != nil {
		t.Fatal(err)
	}
	if custom.Port != 5000 || custom.BatchSize != 8 {
		t.Fatalf("explicit values lost: %+v", custom)
	}
	if custom.ScanDialTimeout != scanDialTimeout || custom.MaxHosts != maxScanHosts {
		t.Fatalf("zero fields not defaulted: %+v", custom)
	}

	if _, err := (&DiscoverConfigObj{Port: -1}).normalized(); err == nil {
		t.Fatal("invalid port accepted")
	}

	invalid := []DiscoverConfigObj{
		{BatchSize: -1},
		{BatchSize: maxScanBatchSize + 1},
		{ScanDialTimeout: -1},
		{ProbeDialTimeout: -1},
		{ReadTimeout: -1},
		{MaxHosts: -1},
		{MaxHosts: maxHostLimit + 1},
		{MinPrefix: minScanPrefix - 1},
		{MinPrefix: 31},
		{KnownIPs: []string{"not-an-ip"}},
		{KnownIPs: make([]string, maxKnownIPs+1)},
	}
	for i := range invalid {
		if _, err = invalid[i].normalized(); err == nil {
			t.Errorf("invalid discovery config %d accepted: %+v", i, invalid[i])
		}
	}
}

func TestDiscoverRejectsNilContext(t *testing.T) {
	var nilContext context.Context
	if _, err := Discover(nilContext, nil); err == nil {
		t.Fatal("Discover() accepted a nil context")
	}
}

func TestDiscoverCancellationStopsEstablishedProbe(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr == nil {
			accepted <- conn
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, discoverErr := Discover(ctx, &DiscoverConfigObj{
			Port:             ln.Addr().(*net.TCPAddr).Port,
			KnownIPs:         []string{"127.0.0.1"},
			ProbeDialTimeout: time.Second,
			ReadTimeout:      time.Hour,
		})
		result <- discoverErr
	}()

	var conn net.Conn
	select {
	case conn = <-accepted:
	case <-time.After(time.Second):
		cancel()
		t.Fatal("Discover() did not establish the known-address probe")
	}
	defer func() { _ = conn.Close() }()
	cancel()

	select {
	case err = <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Discover() cancellation error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Discover() did not stop an established probe on cancellation")
	}
}

func TestDiscoverReportsKnownAddressProgress(t *testing.T) {
	ln := serveMagic(t, "127.0.0.1:0")
	defer func() { _ = ln.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	var progress []DiscoverProgressObj
	remotes, err := Discover(ctx, &DiscoverConfigObj{
		Port:     ln.Addr().(*net.TCPAddr).Port,
		KnownIPs: []string{"127.0.0.1"},
		OnProgress: func(snapshot DiscoverProgressObj) {
			progress = append(progress, snapshot)
			if snapshot.Known && snapshot.Probed == snapshot.Total {
				cancel()
			}
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(progress) != 2 {
		t.Fatalf("progress snapshots = %v", progress)
	}
	if !progress[0].Known || progress[0].Probed != 0 || progress[0].Total != 1 {
		t.Fatalf("initial progress = %+v", progress[0])
	}
	if progress[1].Probed != 1 || progress[1].Found != 1 {
		t.Fatalf("final progress = %+v", progress[1])
	}
	if len(remotes) != 1 || remotes[0].IP != "127.0.0.1" {
		t.Fatalf("partial remotes = %+v", remotes)
	}
}

func TestScanNetCollectsAllRemotes(t *testing.T) {
	first := serveMagic(t, "127.0.0.1:0")
	defer func() { _ = first.Close() }()
	port := first.Addr().(*net.TCPAddr).Port

	// Whole 127/8 is local on Linux: distinct loopback IPs, same port.
	second := serveMagic(t, fmt.Sprintf("127.0.0.3:%d", port))
	defer func() { _ = second.Close() }()
	garbage := make([]byte, 16)
	dud := serveStream(t, fmt.Sprintf("127.0.0.2:%d", port), garbage)
	defer func() { _ = dud.Close() }()

	var progress []DiscoverProgressObj
	reporter := newProgressReporter(func(snapshot DiscoverProgressObj) {
		progress = append(progress, snapshot)
	})
	network := scanNetObj{network: "127.0.0.0/30", base: 0x7F000000, hosts: 3}
	remotes := scanNet(context.Background(), network, scanConfig(t, port), reporter, 5)

	want := []string{"127.0.0.3", "127.0.0.1"}
	if len(remotes) != len(want) {
		t.Fatalf("remotes = %v, want %v", remotes, want)
	}
	for i := range want {
		if remotes[i].IP != want[i] {
			t.Fatalf("remotes = %v, want %v", remotes, want)
		}
	}
	if len(progress) < 2 {
		t.Fatalf("progress snapshots = %v", progress)
	}
	firstProgress := progress[0]
	lastProgress := progress[len(progress)-1]
	if firstProgress.Network != network.network || firstProgress.Probed != 0 ||
		firstProgress.Total != 3 || firstProgress.Found != 5 {
		t.Fatalf("initial progress = %+v", firstProgress)
	}
	if lastProgress.Probed != 3 || lastProgress.Total != 3 || lastProgress.Found != 7 {
		t.Fatalf("final progress = %+v", lastProgress)
	}
}

func TestScanNetSkipsSelf(t *testing.T) {
	ln := serveMagic(t, "127.0.0.1:0")
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*net.TCPAddr).Port

	network := scanNetObj{base: 0x7F000000, hosts: 3, self: 0x7F000001}
	if remotes := scanNet(context.Background(), network, scanConfig(t, port), nil, 0); len(remotes) != 0 {
		t.Fatalf("own address must be skipped, got %v", remotes)
	}
}

// versionReplyPayload is a real RC 2 version-inquiry response body.
var versionReplyPayload = []byte{
	0x00, 0x01, 0x52, 0x43, 0x33, 0x33, 0x31, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x06, 0x00,
	0x00, 0x00, 0x2e, 0x00, 0x06, 0x0a, 0x01, 0x00, 0x00, 0x00,
}

func TestFindVersionReply(t *testing.T) {
	frame := buildFrame(6, 2, 0, 0x80, versionSet, versionID, versionReplyPayload)
	model, firmware, ok := findVersionReply(frame)
	if !ok || model != "RC331" || firmware != "10.06.00.46" {
		t.Fatalf("findVersionReply = %q/%q (ok=%t), want RC331/10.06.00.46", model, firmware, ok)
	}
	if _, _, ok := findVersionReply(buildRCFrame(1, centeredPayload())); ok {
		t.Fatal("a stick frame must not parse as a version reply")
	}
}

func TestProbeCollectsInfo(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()

	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		hello := buildVersionQuery(1) // any valid magic frame confirms it is an RC
		_, _ = conn.Write(hello[:])
		buf := make([]byte, 64)
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		if _, rErr := conn.Read(buf); rErr == nil { // the version request
			_, _ = conn.Write(buildFrame(6, 2, 0, 0x80, versionSet, versionID, versionReplyPayload))
		}
		time.Sleep(50 * time.Millisecond)
	}()

	config, _ := (&DiscoverConfigObj{Port: 1, CollectInfo: true}).normalized()
	model, firmware, ok := probe(context.Background(), ln.Addr().String(), time.Second, config)
	if !ok || model != "RC331" || firmware != "10.06.00.46" {
		t.Fatalf("probe = %q/%q (ok=%t), want RC331/10.06.00.46", model, firmware, ok)
	}
}
