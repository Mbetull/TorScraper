package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"golang.org/x/net/proxy"
)

var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
}

func getTorProxy() string {
	switch runtime.GOOS {
	case "windows":
		fmt.Println("[INFO] Windows algılandı → Tor Browser (9150)")
		return "127.0.0.1:9150"
	case "linux":
		fmt.Println("[INFO] Linux algılandı → Tor Service (9050)")
		return "127.0.0.1:9050"
	default:
		return "127.0.0.1:9050"
	}
}

func createTorClient(torProxy string) (*http.Client, error) {
	torDialer, err := proxy.SOCKS5("tcp", torProxy, nil, proxy.Direct)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return torDialer.Dial(network, addr)
		},
	}

	return &http.Client{
		Transport: transport,
		Timeout:   40 * time.Second,
	}, nil
}

func getRandomUserAgent() string {
	return userAgents[rand.Intn(len(userAgents))]
}

func fetchWithUserAgent(client *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", getRandomUserAgent())
	return client.Do(req)
}

func readTargets(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var targets []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		targets = append(targets, line)
	}

	err = scanner.Err()
	if err != nil {
		return nil, err
	}

	return targets, nil
}

func writeLog(line string) {
	f, _ := os.OpenFile("output/scan_report.log",
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	f.WriteString(line + "\n")
}

func saveHTML(index int, body io.Reader) error {
	file, err := os.Create(fmt.Sprintf("output/html/site_%d.html", index))
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, body)
	return err
}

func takeScreenshot(url string, index int, torProxy string) error {
	opts := append(
		chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ProxyServer("socks5://"+torProxy),
		chromedp.Flag("headless", true),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	var buf []byte
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Sleep(5*time.Second),
		chromedp.FullScreenshot(&buf, 90),
	)

	if err != nil {
		return err
	}

	return os.WriteFile(
		fmt.Sprintf("output/screenshots/site_%d.png", index),
		buf, 0644,
	)
}

func scanTarget(client *http.Client, url string, index int, torProxy string) {

	fmt.Println("[INFO] Scanning:", url)

	resp, err := fetchWithUserAgent(client, url)
	if err != nil {
		fmt.Println("[ERR] Failed:", url)
		writeLog("[DEAD] " + url)
		return
	}
	defer resp.Body.Close()

	saveHTML(index, resp.Body)
	writeLog("[LIVE] " + url)

	takeScreenshot(url, index, torProxy)

	fmt.Println("[OK] SUCCESS:", url)
}

func worker(jobs <-chan struct {
	index int
	url   string
}, client *http.Client, wg *sync.WaitGroup, torProxy string) {

	defer wg.Done()

	for job := range jobs {
		scanTarget(client, job.url, job.index, torProxy)
	}
}

func main() {

	fmt.Println("====================================")
	fmt.Println(" TOR SCRAPER BAŞLATILDI ")
	fmt.Println(" Tarih:", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println(" OS:", runtime.GOOS)
	fmt.Println("====================================")

	rand.Seed(time.Now().UnixNano())

	os.MkdirAll("output/html", 0755)
	os.MkdirAll("output/screenshots", 0755)

	torProxy := getTorProxy()

	client, err := createTorClient(torProxy)
	if err != nil {
		fmt.Println("[FATAL] Tor client oluşturulamadı:", err)
		return
	}

	targets, err := readTargets("targets.yaml")
	if err != nil {
		fmt.Println("[FATAL] targets.yaml okunamadı:", err)
		return
	}

	fmt.Println("[OK] Okunan hedef sayısı:", len(targets))

	const workerCount = 5
	jobs := make(chan struct {
		index int
		url   string
	})

	var wg sync.WaitGroup

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go worker(jobs, client, &wg, torProxy)
	}

	for i, target := range targets {
		jobs <- struct {
			index int
			url   string
		}{i + 1, target}
	}

	close(jobs)
	wg.Wait()

	fmt.Println("[DONE] Tarama tamamlandı")
}
