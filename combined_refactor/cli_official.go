package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type cliOfficialDependencies struct {
	Scan   func(context.Context, *appSession, int, int, int, int, string)
	Detail func(context.Context, *appSession, string, int, int, string, ...bool)
	Speed  func(context.Context, *appSession, []TestResult, int, int, float64) []TestResult
}

func parseCLIDatacenters(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	centers, seen := []string{}, map[string]bool{}
	for _, part := range strings.Split(value, ",") {
		code, ok := normalizeDatacenterCode(part)
		if !ok {
			return nil, fmt.Errorf("-offdc 数据中心代码无效: %q；多个用逗号分隔，如 HKG,NRT", part)
		}
		if !seen[code] {
			centers = append(centers, code)
			seen[code] = true
		}
	}
	return centers, nil
}

func runOfficialCLIMeasurements(cfg *cliConfig, session *appSession, deps cliOfficialDependencies) ([]ScanResult, []TestResult, error) {
	centers, err := parseCLIDatacenters(cfg.dc)
	if err != nil {
		return nil, nil, err
	}
	scanMode := cfg.scanMode
	if scanMode == "" {
		scanMode = scanModeTCPing
	}
	var taskErr error
	if err := session.runTaskSync(func(ctx context.Context, s *appSession) {
		deps.Scan(ctx, s, cfg.ipType, cfg.threads, cfg.port, cfg.delay, scanMode)
		taskErr = ctx.Err()
	}); err != nil {
		return nil, nil, cliTaskError(err)
	}
	if taskErr != nil {
		return nil, nil, fmt.Errorf("CLI 扫描已取消: %w", taskErr)
	}
	session.scanMutex.Lock()
	scans := append([]ScanResult(nil), session.scanResults...)
	session.scanMutex.Unlock()
	if len(scans) == 0 {
		return nil, nil, errors.New("官方模式未发现有效 IP")
	}
	if len(centers) == 0 {
		dc := pickBestDataCenter(scans)
		if dc == "" {
			fmt.Println("[official] 无法确定数据中心，仅输出扫描结果")
			return scans, nil, nil
		}
		centers = []string{dc}
		fmt.Printf("[official] 自动选择数据中心: %s\n", dc)
	}
	found := map[string]bool{}
	for _, row := range scans {
		found[row.DataCenter] = true
	}
	available := []string{}
	for _, dc := range centers {
		if found[dc] {
			available = append(available, dc)
		} else {
			fmt.Printf("[official] 未发现所选数据中心 %s，跳过\n", dc)
		}
	}
	if len(available) == 0 {
		return scans, nil, errors.New("扫描结果中没有所选数据中心")
	}
	fmt.Printf("[official] 详细测试数据中心: %s；全部完成后合并测速\n", strings.Join(available, ", "))
	session.testMutex.Lock()
	session.testResults = nil
	session.testMutex.Unlock()
	if err := session.runTaskSync(func(ctx context.Context, s *appSession) {
		for _, dc := range available {
			if ctx.Err() != nil {
				break
			}
			deps.Detail(ctx, s, dc, cfg.port, cfg.delay, scanMode, true)
		}
		taskErr = ctx.Err()
	}); err != nil {
		return scans, nil, cliTaskError(err)
	}
	if taskErr != nil {
		return scans, nil, fmt.Errorf("CLI 详细测试已取消: %w", taskErr)
	}
	session.testMutex.Lock()
	results := append([]TestResult(nil), session.testResults...)
	session.testMutex.Unlock()
	if cfg.speedLimit <= 0 {
		fmt.Println("[official] 官方测速已关闭（-offspeedlimit 0）")
	} else if len(results) == 0 {
		fmt.Println("[official] 没有可用的详细测试结果，跳过测速")
	} else {
		sortOfficialTestResults(results)
		setCLIProgress(session, "speed", 0, cfg.speedLimit)
		fmt.Printf("[official] 开始合并测速：达标上限=%d，阈值=%.2fMB/s\n", cfg.speedLimit, cfg.speedMin)
		if err := session.runTaskSync(func(ctx context.Context, s *appSession) {
			results = deps.Speed(ctx, s, results, cfg.port, cfg.speedLimit, cfg.speedMin)
			taskErr = ctx.Err()
		}); err != nil {
			return scans, nil, cliTaskError(err)
		}
		if taskErr != nil {
			return scans, nil, fmt.Errorf("CLI 测速已取消: %w", taskErr)
		}
	}
	return scans, results, nil
}
