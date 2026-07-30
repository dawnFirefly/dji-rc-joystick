package rc

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"sync"
	"time"
)

// // // // // // // // // //

const (
	scanBatchSize    = 64
	scanDialTimeout  = 300 * time.Millisecond
	probeDialTimeout = 500 * time.Millisecond
	probeReadTimeout = 500 * time.Millisecond
	maxScanHosts     = 1022
	minScanPrefix    = 16

	maxKnownIPs      = 64
	maxScanBatchSize = 256
	maxHostLimit     = 65534
	maxProbeBytes    = 64 << 10

	discoverProgressInterval = 100 * time.Millisecond
)

var knownIPs = []string{
	"192.168.7.251",
	"192.168.7.253",
	"192.168.42.2",
	"192.168.137.2",
}

// //

// DiscoverConfigObj parameterizes the network scan. A nil config or any
// zero field falls back to the package defaults above.
type DiscoverConfigObj struct {
	// Port is the remote TCP port; zero uses DefaultPort.
	Port int
	// KnownIPs are probed before subnet scanning and are limited to 64 entries.
	KnownIPs []string
	// BatchSize limits concurrent subnet probes to 1..256.
	BatchSize int
	// ScanDialTimeout bounds each subnet connection attempt.
	ScanDialTimeout time.Duration
	// ProbeDialTimeout bounds each known-address connection attempt.
	ProbeDialTimeout time.Duration
	// ReadTimeout bounds protocol identification and optional info collection.
	ReadTimeout time.Duration
	// MaxHosts caps probes per subnet to 1..65534.
	MaxHosts int
	// MinPrefix accepts local IPv4 networks in the range 16..30.
	MinPrefix int
	// CollectInfo also queries each remote's model and firmware.
	CollectInfo bool
	// OnProgress receives serialized, rate-limited discovery progress.
	OnProgress func(DiscoverProgressObj)
}

// DiscoverProgressObj is one aggregate discovery progress snapshot.
type DiscoverProgressObj struct {
	// Known reports whether the known-address pass is active.
	Known bool
	// Network is the current subnet CIDR; empty during the known-address pass.
	Network string
	// Probed is the number of completed probes in the current pass.
	Probed int
	// Total is the number of probes planned for the current pass.
	Total int
	// Found is the total number of remotes found so far.
	Found int
}

// RemoteObj is a discovered DJI remote. Model and Firmware are populated only
// when DiscoverConfigObj.CollectInfo is set.
type RemoteObj struct {
	// IP is the discovered address.
	IP string
	// Model is populated when CollectInfo is enabled and a reply is decoded.
	Model string
	// Firmware is populated when CollectInfo is enabled and a reply is decoded.
	Firmware string
}

type scanNetObj struct {
	network string
	base    uint32 // network base address, host byte order
	hosts   int
	self    uint32 // our own address, skipped during the scan
}

type progressReporterObj struct {
	callback func(DiscoverProgressObj)
	mu       sync.Mutex
	last     time.Time
}

// //

func newProgressReporter(callback func(DiscoverProgressObj)) *progressReporterObj {
	if callback == nil {
		return nil
	}
	return &progressReporterObj{callback: callback}
}

func (obj *progressReporterObj) publish(progress DiscoverProgressObj, force bool) {
	if obj == nil {
		return
	}

	obj.mu.Lock()
	defer obj.mu.Unlock()
	now := time.Now()
	if !force && now.Sub(obj.last) < discoverProgressInterval {
		return
	}
	obj.last = now
	obj.callback(progress)
}

func (c *DiscoverConfigObj) normalized() (DiscoverConfigObj, error) {
	var config DiscoverConfigObj
	if c != nil {
		config = *c
	}

	port, err := normalizePort(config.Port)
	if err != nil {
		return DiscoverConfigObj{}, err
	}
	config.Port = port

	if len(config.KnownIPs) == 0 {
		config.KnownIPs = append([]string(nil), knownIPs...)
	} else {
		if len(config.KnownIPs) > maxKnownIPs {
			return DiscoverConfigObj{}, fmt.Errorf("rc: known IP count exceeds %d", maxKnownIPs)
		}
		config.KnownIPs = append([]string(nil), config.KnownIPs...)
		for _, ip := range config.KnownIPs {
			if net.ParseIP(ip) == nil {
				return DiscoverConfigObj{}, fmt.Errorf("rc: invalid known IP address %q", ip)
			}
		}
	}
	if config.BatchSize == 0 {
		config.BatchSize = scanBatchSize
	} else if config.BatchSize < 0 || config.BatchSize > maxScanBatchSize {
		return DiscoverConfigObj{}, fmt.Errorf("rc: scan batch size must be in 1..%d", maxScanBatchSize)
	}
	if config.ScanDialTimeout == 0 {
		config.ScanDialTimeout = scanDialTimeout
	} else if config.ScanDialTimeout < 0 {
		return DiscoverConfigObj{}, errors.New("rc: scan dial timeout is negative")
	}
	if config.ProbeDialTimeout == 0 {
		config.ProbeDialTimeout = probeDialTimeout
	} else if config.ProbeDialTimeout < 0 {
		return DiscoverConfigObj{}, errors.New("rc: probe dial timeout is negative")
	}
	if config.ReadTimeout == 0 {
		config.ReadTimeout = probeReadTimeout
	} else if config.ReadTimeout < 0 {
		return DiscoverConfigObj{}, errors.New("rc: probe read timeout is negative")
	}
	if config.MaxHosts == 0 {
		config.MaxHosts = maxScanHosts
	} else if config.MaxHosts < 0 || config.MaxHosts > maxHostLimit {
		return DiscoverConfigObj{}, fmt.Errorf("rc: maximum host count must be in 1..%d", maxHostLimit)
	}
	if config.MinPrefix == 0 {
		config.MinPrefix = minScanPrefix
	} else if config.MinPrefix < minScanPrefix || config.MinPrefix > 30 {
		return DiscoverConfigObj{}, fmt.Errorf("rc: minimum scan prefix must be in %d..30", minScanPrefix)
	}
	return config, nil
}

func ipString(addr uint32) string {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, addr)
	return ip.String()
}

// probe connects and confirms the DJI TCP transport magic. With CollectInfo
// it then sends one version query and parses the model/firmware from the
// reply. Returns whether the host is a DJI RC.
func probe(ctx context.Context, addr string, dialTimeout time.Duration, config DiscoverConfigObj) (model, firmware string, isRC bool) {
	dialer := net.Dialer{Timeout: dialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return "", "", false
	}
	defer func() { _ = conn.Close() }()
	stopCancel := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopCancel()

	_ = conn.SetReadDeadline(time.Now().Add(config.ReadTimeout))
	acc := make([]byte, 0, 1024)
	buf := make([]byte, 1024)
	sentVersion := false
	for {
		n, readErr := conn.Read(buf)
		if n > 0 {
			remaining := maxProbeBytes - len(acc)
			if n > remaining {
				n = remaining
			}
			acc = append(acc, buf[:n]...)
			if len(acc) >= 8 {
				if !bytes.Equal(acc[:4], tcpMagic[:]) {
					return "", "", false
				}
				isRC = true
				if !config.CollectInfo {
					return "", "", true
				}
				if !sentVersion {
					req := buildVersionRequest(1)
					if _, wErr := conn.Write(req[:]); wErr != nil {
						return "", "", true
					}
					sentVersion = true
				}
				if m, f, ok := findVersionReply(acc); ok {
					return m, f, true
				}
			}
			if len(acc) == maxProbeBytes {
				return "", "", isRC
			}
		}
		if readErr != nil {
			return "", "", isRC
		}
	}
}

func findVersionReply(data []byte) (model, firmware string, ok bool) {
	var (
		channels [maxChannels]int16
		payload  [rawPayloadMax]byte
		packets  [maxParsePackets]PacketObj
	)
	_, _, _, count := parseStream(data, &channels, &payload, &packets)
	for i := 0; i < count; i++ {
		p := packets[i]
		if p.IsResponse() && p.CmdSet == versionSet && p.CmdID == versionID {
			m, f := parseVersionReply(p.Payload[:p.Len])
			return m, f, true
		}
	}
	return "", "", false
}

// localNets collects scannable subnets: up, non-loopback, IPv4,
// not link-local, prefix not narrower than config.MinPrefix.
func localNets(config DiscoverConfigObj) []scanNetObj {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	var nets []scanNetObj
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, addrErr := iface.Addrs()
		if addrErr != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipNet.IP.To4()
			if ip4 == nil || (ip4[0] == 169 && ip4[1] == 254) {
				continue
			}
			ones, bits := ipNet.Mask.Size()
			if bits != 32 || ones < config.MinPrefix || ones > 30 {
				continue
			}

			hosts := 1<<(32-ones) - 2
			if hosts > config.MaxHosts {
				hosts = config.MaxHosts
			}
			self := binary.BigEndian.Uint32(ip4)
			base := self &^ (1<<(32-ones) - 1)
			nets = append(nets, scanNetObj{
				network: ipString(base) + "/" + strconv.Itoa(ones),
				base:    base,
				hosts:   hosts,
				self:    self,
			})
		}
	}
	return nets
}

// scanNet sweeps one whole subnet, high to low (DHCP usually assigns from
// the top of the range), at most config.BatchSize probes in flight, and
// collects every host that speaks the DJI protocol.
func scanNet(ctx context.Context, network scanNetObj, config DiscoverConfigObj,
	reporter *progressReporterObj, foundBase int) []RemoteObj {
	type hit struct {
		addr   uint32
		remote RemoteObj
	}
	var (
		mu     sync.Mutex
		found  []hit
		probed int
	)
	sem := make(chan struct{}, config.BatchSize)
	var wg sync.WaitGroup

	total := network.hosts
	selfHost := int(network.self - network.base)
	if selfHost >= 1 && selfHost <= network.hosts {
		total--
	}
	progress := DiscoverProgressObj{Network: network.network, Total: total, Found: foundBase}
	reporter.publish(progress, true)

scan:
	for host := network.hosts; host >= 1; host-- {
		if ctx.Err() != nil {
			break
		}
		addr32 := network.base + uint32(host)
		if addr32 == network.self {
			continue
		}

		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			break scan
		}
		wg.Add(1)
		go func(target uint32) {
			defer wg.Done()
			defer func() { <-sem }()

			ip := ipString(target)
			addr := net.JoinHostPort(ip, strconv.Itoa(config.Port))
			var remote RemoteObj
			foundRemote := false
			if model, firmware, ok := probe(ctx, addr, config.ScanDialTimeout, config); ok {
				remote = RemoteObj{IP: ip, Model: model, Firmware: firmware}
				foundRemote = true
			}

			mu.Lock()
			if foundRemote {
				found = append(found, hit{addr: target, remote: remote})
			}
			probed++
			progress.Probed = probed
			progress.Found = foundBase + len(found)
			reporter.publish(progress, false)
			mu.Unlock()
		}(addr32)
	}
	wg.Wait()

	mu.Lock()
	progress.Probed = probed
	progress.Found = foundBase + len(found)
	reporter.publish(progress, true)
	mu.Unlock()

	// Deterministic order: high to low, matching the scan direction.
	sort.Slice(found, func(i, j int) bool { return found[i].addr > found[j].addr })
	remotes := make([]RemoteObj, 0, len(found))
	for _, h := range found {
		remotes = append(remotes, h.remote)
	}
	return remotes
}

// //

// Discover probes known addresses, then bounded local IPv4 subnets. Results
// are deduplicated by IP. Cancellation returns the remotes accumulated so far
// together with the context error.
func Discover(ctx context.Context, config *DiscoverConfigObj) ([]RemoteObj, error) {
	if ctx == nil {
		return nil, errors.New("rc: context is nil")
	}

	resolved, err := config.normalized()
	if err != nil {
		return nil, err
	}

	var found []RemoteObj
	reporter := newProgressReporter(resolved.OnProgress)
	seen := make(map[string]struct{})
	add := func(remote RemoteObj) {
		if _, ok := seen[remote.IP]; !ok {
			seen[remote.IP] = struct{}{}
			found = append(found, remote)
		}
	}

	progress := DiscoverProgressObj{Known: true, Total: len(resolved.KnownIPs)}
	reporter.publish(progress, true)
	for index, ip := range resolved.KnownIPs {
		if ctx.Err() != nil {
			return found, ctx.Err()
		}
		addr := net.JoinHostPort(ip, strconv.Itoa(resolved.Port))
		if model, firmware, ok := probe(ctx, addr, resolved.ProbeDialTimeout, resolved); ok {
			add(RemoteObj{IP: ip, Model: model, Firmware: firmware})
		}
		progress.Probed = index + 1
		progress.Found = len(found)
		reporter.publish(progress, index+1 == len(resolved.KnownIPs))
	}

	for _, network := range localNets(resolved) {
		if ctx.Err() != nil {
			return found, ctx.Err()
		}
		for _, remote := range scanNet(ctx, network, resolved, reporter, len(found)) {
			add(remote)
		}
	}

	if ctx.Err() != nil {
		return found, ctx.Err()
	}
	return found, nil
}
