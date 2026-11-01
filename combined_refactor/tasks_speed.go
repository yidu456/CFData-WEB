package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func isTLSPort(port int) bool {
	return port == 443 || port == 2053 || port == 2083 || port == 2087 || port == 2096 || port == 8443
}

func runWindowedSpeedTest(ctx context.Context, ip string, port int, customURL string) (float64, string) {
	scheme := "http"
	if isTLSPort(port) {
		scheme = "https"
	}

	parsedURL, err := parseSpeedTestURL(customURL, scheme)
	if err != nil {
		return 0, "URL解析错误: " + err.Error()
	}

	transport := &http.Transport{
		DialContext: func(c context.Context, network, addr string) (net.Conn, error) {
			dialer := &net.Dialer{Timeout: 5 * time.Second}
			return dialer.DialContext(c, "tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
		},
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     tlsConfigWithRootCAs(parsedURL.Hostname()),
		DisableCompression:  true,
	}
	client := http.Client{
		Transport: wrapDebugTransport("official-speed", transport),
		Timeout:   15 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", parsedURL.String(), nil)
	if err != nil {
		return 0, "请求构造错误"
	}
	req.Host = parsedURL.Host
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept-Encoding", "identity")

	resp, err := client.Do(req)
	if err != nil {
		return 0, "连接错误"
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return 0, formatSpeedHTTPFailure(resp.StatusCode)
	}

	buf := make([]byte, 32*1024)
	var totalBytes int64

	timeout := time.After(6 * time.Second)
	measuredStart := time.Now()

	readerCtx, readerCancel := context.WithCancel(ctx)
	defer readerCancel()

	type readChunk struct {
		n   int
		err error
	}
	chunks := make(chan readChunk, 16)
	readerDone := make(chan struct{})
	safeGo("speed-reader", nil, func() {
		defer close(readerDone)
		for {
			n, err := resp.Body.Read(buf)
			select {
			case chunks <- readChunk{n: n, err: err}:
			case <-readerCtx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	})

	done := false
	canceled := false
	var readErr error
loop:
	for !done {
		select {
		case <-ctx.Done():
			canceled = true
			done = true
		case <-timeout:
			done = true
		case chunk := <-chunks:
			if chunk.n > 0 {
				totalBytes += int64(chunk.n)
			}
			if chunk.err != nil {
				if chunk.err != io.EOF {
					readErr = chunk.err
				}
				done = true
				break loop
			}
		}
	}

	readerCancel()
	resp.Body.Close()
	<-readerDone

	if canceled {
		return 0, "测速任务已终止"
	}
	if readErr != nil {
		return 0, "测速失败"
	}

	if totalBytes <= 0 {
		return 0, "0MB/s"
	}

	duration := time.Since(measuredStart).Seconds()
	if duration == 0 {
		duration = 1
	}
	return float64(totalBytes) / duration / 1024 / 1024, ""
}

func formatSpeedHTTPFailure(statusCode int) string {
	if statusCode == http.StatusTooManyRequests {
		return "测速失败（速率限制）"
	}
	return "测速失败"
}

func runSpeedTest(ctx context.Context, session *appSession, ip string, port int, customURL string) {
	session.sendWSMessage("log", fmt.Sprintf("开始对 IP %s 端口 %d 进行测速...", ip, port))

	speedMB, speedErr := runWindowedSpeedTest(ctx, ip, port, customURL)
	if ctx.Err() != nil {
		return
	}
	session.testMutex.Lock()
	var emitted *TestResult
	for i := range session.testResults {
		if session.testResults[i].IP == ip && session.testResults[i].Port == port {
			applyOfficialSpeedMeasurement(&session.testResults[i], speedMB, speedErr)
			copy := session.testResults[i]
			emitted = &copy
		}
	}
	session.testMutex.Unlock()
	if emitted == nil {
		session.scanMutex.Lock()
		for _, scan := range session.scanResults {
			if scan.IP == ip && scan.Port == port {
				row := TestResult{IP: ip, Port: port, DataCenter: scan.DataCenter, DCCountry: scan.DCCountry, Region: scan.Region, City: scan.City, RecordedScanMode: scan.RecordedScanMode, MinLatency: scan.TCPDuration, MaxLatency: scan.TCPDuration, AvgLatency: scan.TCPDuration}
				applyOfficialSpeedMeasurement(&row, speedMB, speedErr)
				emitted = &row
				break
			}
		}
		session.scanMutex.Unlock()
		if emitted != nil {
			session.testMutex.Lock()
			session.testResults = append(session.testResults, *emitted)
			session.testMutex.Unlock()
			session.sendWSMessage("test_result", *emitted)
		}
	}
	if speedErr != "" {
		if ctx.Err() != nil {
			return
		}
		session.sendWSMessage("speed_test_result", map[string]string{"ip": ip, "port": strconv.Itoa(port), "speed": speedErr})
		session.sendWSMessage("log", "测速失败: "+speedErr)
		return
	}
	if ctx.Err() != nil {
		return
	}
	speedStr := fmt.Sprintf("%.2fMB/s", speedMB)

	session.sendWSMessage("speed_test_result", map[string]string{"ip": ip, "port": strconv.Itoa(port), "speed": speedStr})
	session.sendWSMessage("log", fmt.Sprintf("IP %s 测速完成: %s", ip, speedStr))
}

const speedRateLimitMessage = "检测到触发速率限制，请稍后重试或更换网络环境"

func isSpeedRateLimited(speedErr string) bool {
	return strings.Contains(speedErr, "速率限制")
}

func applyOfficialSpeedMeasurement(result *TestResult, speedMB float64, speedErr string) {
	result.MeasuredSpeedMB, result.SpeedMeasured = 0, false
	if speedErr != "" {
		result.Speed = speedErr
		return
	}
	if !finiteNonnegative(speedMB) {
		result.Speed = "测速失败"
		return
	}
	result.Speed = fmt.Sprintf("%.2fMB/s", speedMB)
	result.MeasuredSpeedMB, result.SpeedMeasured = speedMB, true
}

func runOfficialSpeedTestsCore(ctx context.Context, results []TestResult, port int, limit int, speedMinMB float64, customURL string, onResult func(current, total, qualified int, result TestResult), onRateLimited func()) ([]TestResult, []TestResult) {
	results = officialSpeedCandidates(results, port)
	capacity := len(results)
	if limit > 0 && limit < capacity {
		capacity = limit
	}
	qualified := make([]TestResult, 0, capacity)
	interSpeedPause := func() bool {
		select {
		case <-ctx.Done():
			return false
		case <-time.After(1200 * time.Millisecond):
			return true
		}
	}
	consecutiveRateLimited := 0
	for i := range results {
		select {
		case <-ctx.Done():
			return results, qualified
		default:
		}
		if limit > 0 && len(qualified) >= limit {
			break
		}
		speedMB, speedErr := runWindowedSpeedTest(ctx, results[i].IP, port, customURL)
		applyOfficialSpeedMeasurement(&results[i], speedMB, speedErr)
		if speedErr != "" {
			if isSpeedRateLimited(speedErr) {
				consecutiveRateLimited++
			} else {
				consecutiveRateLimited = 0
			}
		} else {
			consecutiveRateLimited = 0
			if speedMB >= speedMinMB {
				qualified = append(qualified, results[i])
			}
		}
		if onResult != nil {
			if ctx.Err() != nil {
				return results, qualified
			}
			onResult(i+1, len(results), len(qualified), results[i])
		}
		if !interSpeedPause() {
			return results, qualified
		}
		if consecutiveRateLimited >= 3 {
			if onRateLimited != nil {
				onRateLimited()
			}
			break
		}
	}
	return results, qualified
}

func officialSpeedCandidates(results []TestResult, port int) []TestResult {
	candidates := make([]TestResult, 0, len(results))
	positions := make(map[string]int, len(results))
	for _, result := range results {
		if i, exists := positions[result.IP]; exists {
			if result.Port == port {
				candidates[i] = result
			}
			continue
		}
		// The common port override measures a new endpoint, not the source row.
		if result.Port != port {
			result.Port = port
			result.Speed, result.MeasuredSpeedMB, result.SpeedMeasured = "", 0, false
		}
		positions[result.IP] = len(candidates)
		candidates = append(candidates, result)
	}
	return candidates
}

func runOfficialSpeedBatch(ctx context.Context, session *appSession, port int, customURL string, speedLimit int, speedMin float64, fallbackResults []TestResult, skipTested bool, scopes ...[]string) {
	if port <= 0 {
		port = 443
	}
	if speedLimit <= 0 {
		session.sendWSMessage("log", "官方批量测速已关闭（测速上限 0）")
		session.sendWSMessage("official_speed_complete", map[string]interface{}{"qualified": 0, "limit": speedLimit, "rateLimited": false})
		return
	}
	if speedMin <= 0 {
		speedMin = 0.1
	}

	session.testMutex.Lock()
	pool := session.testResults
	if len(scopes) > 0 {
		selected := make(map[string]bool)
		for _, dc := range scopes[0] {
			selected[dc] = true
		}
		pool = nil
		known := make(map[string]bool)
		for _, row := range session.testResults {
			if selected[row.DataCenter] {
				pool = append(pool, row)
				known[net.JoinHostPort(row.IP, strconv.Itoa(row.Port))] = true
			}
		}
		session.scanMutex.Lock()
		for _, scan := range session.scanResults {
			if selected[scan.DataCenter] && !known[net.JoinHostPort(scan.IP, strconv.Itoa(scan.Port))] {
				pool = append(pool, TestResult{IP: scan.IP, Port: scan.Port, DataCenter: scan.DataCenter, DCCountry: scan.DCCountry, Region: scan.Region, City: scan.City, RecordedScanMode: scan.RecordedScanMode, MinLatency: scan.TCPDuration, MaxLatency: scan.TCPDuration, AvgLatency: scan.TCPDuration})
			}
		}
		session.scanMutex.Unlock()
	}
	results := officialSpeedCandidates(pool, port)
	session.testMutex.Unlock()
	if len(scopes) == 0 && len(results) == 0 && len(fallbackResults) > 0 {
		results = officialSpeedCandidates(fallbackResults, port)
		session.testMutex.Lock()
		session.testResults = append([]TestResult(nil), fallbackResults...)
		session.testMutex.Unlock()
	}
	if len(results) == 0 {
		session.sendWSMessage("log", "没有可用的详细测试结果，跳过官方批量测速")
		session.sendWSMessage("official_speed_complete", map[string]interface{}{"qualified": 0, "limit": speedLimit, "rateLimited": false})
		return
	}
	if skipTested {
		pending := results[:0]
		for _, result := range results {
			speedText := strings.TrimSpace(result.Speed)
			if speedText == "" || speedText == "未测速" {
				pending = append(pending, result)
			}
		}
		results = pending
		if len(results) == 0 {
			session.sendWSMessage("log", "没有未测速的详细测试结果，跳过继续测速")
			session.sendWSMessage("official_speed_complete", map[string]interface{}{"qualified": 0, "limit": speedLimit, "rateLimited": false})
			return
		}
	}
	sortOfficialTestResults(results)

	session.sendWSMessage("log", fmt.Sprintf("开始测速：%d 条记录，目标上限=%d，测速阈值=%.2fMB/s", len(results), speedLimit, speedMin))
	session.sendWSMessage("official_speed_progress", map[string]interface{}{"current": 0, "total": len(results), "qualified": 0, "limit": speedLimit})
	rateLimited := false
	updated, qualified := runOfficialSpeedTestsCore(ctx, results, port, speedLimit, speedMin, customURL, func(current, total, qualified int, result TestResult) {
		if len(scopes) > 0 {
			session.sendWSMessage("test_result", result)
		}
		session.sendWSMessage("speed_test_result", map[string]string{"ip": result.IP, "port": strconv.Itoa(result.Port), "speed": result.Speed})
		session.sendWSMessage("official_speed_progress", map[string]interface{}{"current": current, "total": total, "qualified": qualified, "limit": speedLimit})
	}, func() {
		rateLimited = true
		session.sendWSMessage("log", speedRateLimitMessage)
	})

	session.testMutex.Lock()
	positions := make(map[string]int, len(session.testResults))
	for idx, result := range session.testResults {
		positions[net.JoinHostPort(result.IP, strconv.Itoa(result.Port))] = idx
	}
	for _, result := range updated {
		if idx, exists := positions[net.JoinHostPort(result.IP, strconv.Itoa(result.Port))]; exists {
			session.testResults[idx].Speed = result.Speed
			session.testResults[idx].MeasuredSpeedMB = result.MeasuredSpeedMB
			session.testResults[idx].SpeedMeasured = result.SpeedMeasured
		} else if result.Speed != "" && result.Speed != "未测速" {
			positions[net.JoinHostPort(result.IP, strconv.Itoa(result.Port))] = len(session.testResults)
			session.testResults = append(session.testResults, result)
		}
	}
	qualificationPool := session.testResults
	if len(scopes) > 0 {
		selected := make(map[string]bool)
		for _, dc := range scopes[0] {
			selected[dc] = true
		}
		qualificationPool = nil
		for _, row := range session.testResults {
			if selected[row.DataCenter] {
				qualificationPool = append(qualificationPool, row)
			}
		}
	}
	totalQualified := countOfficialQualifiedResults(qualificationPool, speedMin)
	session.testMutex.Unlock()
	if ctx.Err() != nil {
		session.sendWSMessage("log", "官方批量测速已终止")
	} else if rateLimited {
		session.sendWSMessage("log", fmt.Sprintf("官方批量测速已因速率限制停止，达标 %d/%d，总达标 %d", len(qualified), speedLimit, totalQualified))
	} else {
		session.sendWSMessage("log", fmt.Sprintf("官方批量测速完成，达标 %d/%d，总达标 %d", len(qualified), speedLimit, totalQualified))
	}
	if ctx.Err() == nil {
		session.sendWSMessage("official_speed_complete", map[string]interface{}{"qualified": len(qualified), "totalQualified": totalQualified, "limit": speedLimit, "rateLimited": rateLimited})
	}
}

func countOfficialQualifiedResults(results []TestResult, speedMin float64) int {
	count := 0
	for _, result := range results {
		if speed, ok := parseSpeedMBForSort(result.Speed); ok && speed >= speedMin {
			count++
		}
	}
	return count
}
