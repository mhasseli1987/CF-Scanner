package main

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

var cloudflareRanges = []string{
	"103.21.244.0/22",
	"103.22.200.0/22",
	"103.31.4.0/22",
	"104.16.0.0/13",
	"104.24.0.0/14",
	"108.162.192.0/18",
	"131.0.72.0/22",
	"141.101.64.0/18",
	"162.158.0.0/15",
	"172.64.0.0/13",
	"173.245.48.0/20",
	"188.114.96.0/20",
	"190.93.240.0/20",
	"197.234.240.0/22",
	"198.41.128.0/17",
}

type ScanResult struct {
	IP      string
	Latency time.Duration
}

func expandCIDR(cidr string) ([]net.IP, error) {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}
	var ips []net.IP
	for ip := ip.Mask(ipNet.Mask); ipNet.Contains(ip); incrementIP(ip) {
		tmp := make(net.IP, len(ip))
		copy(tmp, ip)
		ips = append(ips, tmp)
	}
	if len(ips) > 2 {
		ips = ips[1 : len(ips)-1]
	}
	return ips, nil
}

func incrementIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func testIP(ip string, timeout time.Duration) (time.Duration, bool) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", ip+":443")
	if err != nil {
		return 0, false
	}
	latency := time.Since(start)
	conn.Close()
	return latency, true
}

func main() {
	fmt.Println("╔══════════════════════════════════════════════════╗")
	fmt.Println("║       Cloudflare Clean IP Scanner v1.0          ║")
	fmt.Println("╠══════════════════════════════════════════════════╣")
	fmt.Println("║  Scanning Cloudflare IP ranges...               ║")
	fmt.Println("║  Target: 50 clean IPs with ping < 200ms        ║")
	fmt.Println("╚══════════════════════════════════════════════════╝")
	fmt.Println()

	var allIPs []net.IP
	fmt.Print("[*] Loading Cloudflare IP ranges... ")
	for _, cidr := range cloudflareRanges {
		ips, err := expandCIDR(cidr)
		if err != nil {
			continue
		}
		allIPs = append(allIPs, ips...)
	}
	fmt.Printf("Done! (%d IPs loaded)\n", len(allIPs))

	rand.Shuffle(len(allIPs), func(i, j int) {
		allIPs[i], allIPs[j] = allIPs[j], allIPs[i]
	})

	const (
		targetCount = 50
		maxWorkers  = 128
		maxTimeout  = 200 * time.Millisecond
		maxAttempts = 5000
	)

	var (
		results []ScanResult
		mu      sync.Mutex
		tested  int64
		found   int64
		wg      sync.WaitGroup
		done    int32
	)

	sem := make(chan struct{}, maxWorkers)

	fmt.Printf("[*] Scanning with %d concurrent workers...\n", maxWorkers)
	fmt.Printf("[*] Max latency: %v | Target: %d clean IPs\n\n", maxTimeout, targetCount)

	scanLimit := maxAttempts
	if len(allIPs) < scanLimit {
		scanLimit = len(allIPs)
	}

	startTime := time.Now()

	for i := 0; i < scanLimit; i++ {
		if atomic.LoadInt32(&done) == 1 {
			break
		}

		ip := allIPs[i].String()
		sem <- struct{}{}
		wg.Add(1)

		go func(ipStr string) {
			defer wg.Done()
			defer func() { <-sem }()

			if atomic.LoadInt32(&done) == 1 {
				return
			}

			latency, ok := testIP(ipStr, maxTimeout)
			current := atomic.AddInt64(&tested, 1)

			if ok && latency < maxTimeout {
				mu.Lock()
				results = append(results, ScanResult{IP: ipStr, Latency: latency})
				currentFound := int64(len(results))
				mu.Unlock()

				atomic.StoreInt64(&found, currentFound)
				fmt.Printf("  [+] Found: %-15s | Latency: %v\n", ipStr, latency.Round(time.Millisecond))

				if currentFound >= targetCount {
					atomic.StoreInt32(&done, 1)
				}
			}

			if current%100 == 0 {
				fmt.Printf("  [~] Progress: %d tested | %d found\n", current, atomic.LoadInt64(&found))
			}
		}(ip)
	}

	wg.Wait()
	elapsed := time.Since(startTime)

	fmt.Printf("\n[*] Scan completed in %v\n", elapsed.Round(time.Millisecond))
	fmt.Printf("[*] Tested: %d | Found: %d clean IPs\n\n", atomic.LoadInt64(&tested), len(results))

	if len(results) == 0 {
		fmt.Println("[!] No clean IPs found. Try again or check your internet connection.")
		fmt.Println("\nPress Enter to exit...")
		fmt.Scanln()
		return
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Latency < results[j].Latency
	})

	if len(results) > targetCount {
		results = results[:targetCount]
	}

	filename := fmt.Sprintf("clean_ips_%s.txt", time.Now().Format("2006-01-02_15-04-05"))
	file, err := os.Create(filename)
	if err != nil {
		fmt.Printf("[!] Error creating file: %v\n", err)
		fmt.Println("\nPress Enter to exit...")
		fmt.Scanln()
		return
	}
	defer file.Close()

	for _, r := range results {
		file.WriteString(r.IP + "\n")
	}

	fmt.Println("╔══════════════════════════════════════════════════╗")
	fmt.Println("║              RESULTS SUMMARY                     ║")
	fmt.Println("╠══════════════════════════════════════════════════╣")
	fmt.Printf("║  Clean IPs found: %-5d                          ║\n", len(results))
	fmt.Printf("║  Best latency:    %-10v                     ║\n", results[0].Latency.Round(time.Millisecond))
	fmt.Printf("║  Worst latency:   %-10v                     ║\n", results[len(results)-1].Latency.Round(time.Millisecond))
	fmt.Printf("║  Saved to: %-38s║\n", filename)
	fmt.Println("╚══════════════════════════════════════════════════╝")

	fmt.Println("\nTop 10 fastest IPs:")
	fmt.Println("─────────────────────────────────")
	limit := 10
	if len(results) < limit {
		limit = len(results)
	}
	for i := 0; i < limit; i++ {
		fmt.Printf("  %2d. %-15s  %v\n", i+1, results[i].IP, results[i].Latency.Round(time.Millisecond))
	}

	fmt.Println("\nPress Enter to exit...")
	fmt.Scanln()
}
