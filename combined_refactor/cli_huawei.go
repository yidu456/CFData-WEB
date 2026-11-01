package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"strings"
	"time"
)

type cliHuaweiConfig struct {
	Enabled   bool   `json:"enabled"`
	Zone      string `json:"zone"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Line      string `json:"line"`
	Mode      string `json:"mode"`
	StartRow  int    `json:"startRow"`
	EndRow    int    `json:"endRow"`
	Confirmed bool   `json:"-"`
}

var cliHuaweiFlags = []cliFlagInfo{
	{name: "huawei", description: "官方 CLI 测速完成后预览并更新华为 DNS；仅使用合格结果，默认需要本次确认", defaultValue: "false"},
	{name: "hwzone", description: "华为公网区域域名，如 example.com（不是区域 ID）", defaultValue: ""},
	{name: "hwname", description: "已有 A/AAAA 记录完整域名，如 edge.example.com", defaultValue: ""},
	{name: "hwtype", description: "DNS 记录类型 A 或 AAAA，须与 offiptype 对应", defaultValue: "A"},
	{name: "hwline", description: "已有 DNS 记录线路，必须准确匹配", defaultValue: "default_view"},
	{name: "hwmode", description: "DNS IP 更新方式：overwrite 覆盖 / append 追加", defaultValue: "overwrite"},
	{name: "hwstartrow", description: "合格结果全局排序、地址族筛选及去重后的起始排名；独立于导出范围", defaultValue: "1"},
	{name: "hwendrow", description: "DNS 结束排名（含）；0 至末尾；测速达标上限仍由 offspeedlimit 控制", defaultValue: "10"},
	{name: "hwconfirm", description: "明确授权本次运行按预览更新 DNS；仅本次有效，不保存至配置；无此参数则询问确认", defaultValue: "false"},
}

func defaultCLIHuaweiConfig() cliHuaweiConfig {
	return cliHuaweiConfig{Type: "A", Line: "default_view", Mode: "overwrite", StartRow: 1, EndRow: 10}
}

func registerCLIHuaweiFlags(c *cliHuaweiConfig) {
	*c = defaultCLIHuaweiConfig()
	flag.BoolVar(&c.Enabled, "huawei", false, cliHuaweiFlags[0].description)
	flag.StringVar(&c.Zone, "hwzone", "", cliHuaweiFlags[1].description)
	flag.StringVar(&c.Name, "hwname", "", cliHuaweiFlags[2].description)
	flag.StringVar(&c.Type, "hwtype", c.Type, cliHuaweiFlags[3].description)
	flag.StringVar(&c.Line, "hwline", c.Line, cliHuaweiFlags[4].description)
	flag.StringVar(&c.Mode, "hwmode", c.Mode, cliHuaweiFlags[5].description)
	flag.IntVar(&c.StartRow, "hwstartrow", c.StartRow, cliHuaweiFlags[6].description)
	flag.IntVar(&c.EndRow, "hwendrow", c.EndRow, cliHuaweiFlags[7].description)
	flag.BoolVar(&c.Confirmed, "hwconfirm", false, cliHuaweiFlags[8].description)
}

func applyCLIHuaweiConfig(dst *cliHuaweiConfig, file cliHuaweiConfig, provided map[string]bool) {
	if !provided["huawei"] {
		dst.Enabled = file.Enabled
	}
	if !provided["hwzone"] {
		dst.Zone = file.Zone
	}
	if !provided["hwname"] {
		dst.Name = file.Name
	}
	if !provided["hwtype"] {
		dst.Type = file.Type
	}
	if !provided["hwline"] {
		dst.Line = file.Line
	}
	if !provided["hwmode"] {
		dst.Mode = file.Mode
	}
	if !provided["hwstartrow"] {
		dst.StartRow = file.StartRow
	}
	if !provided["hwendrow"] {
		dst.EndRow = file.EndRow
	}
	if !provided["hwconfirm"] {
		dst.Confirmed = false
	}
}

func normalizeCLIHuaweiName(value string) (string, error) {
	name := strings.ToLower(strings.TrimSpace(value))
	name = strings.TrimSuffix(name, ".")
	if name == "" || len(name) > 253 {
		return "", errors.New("DNS 域名必须为完整域名，如 edge.example.com")
	}
	for i, label := range strings.Split(name, ".") {
		if i == 0 && label == "*" {
			continue
		}
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", errors.New("DNS 域名格式无效")
		}
		for _, c := range label {
			if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' && c != '_' {
				return "", errors.New("DNS 域名须为 ASCII/Punycode，不包含协议、路径或端口")
			}
		}
	}
	return name + ".", nil
}

func validateCLIHuaweiConfig(cfg *cliConfig) error {
	c := &cfg.huawei
	if !c.Enabled {
		return nil
	}
	if !strings.EqualFold(cfg.mode, "official") || cfg.compactIPv4 || strings.TrimSpace(cfg.export.GHUpload) != "" {
		return errors.New("华为 DNS 更新仅支持官方 CLI 扫描测速流程")
	}
	if cfg.speedLimit <= 0 || !finiteNonnegative(cfg.speedMin) {
		return errors.New("华为 DNS 更新需要开启测速并设置有效的合格速度阈值")
	}
	c.Type, c.Mode, c.Line = strings.ToUpper(strings.TrimSpace(c.Type)), strings.ToLower(strings.TrimSpace(c.Mode)), strings.TrimSpace(c.Line)
	if (c.Type != "A" && c.Type != "AAAA") || (c.Type == "A" && cfg.ipType != 4) || (c.Type == "AAAA" && cfg.ipType != 6) {
		return errors.New("-hwtype 须为 A/AAAA 且与 -offiptype 4/6 对应")
	}
	if c.Mode != "overwrite" && c.Mode != "append" {
		return errors.New("-hwmode 仅支持 overwrite 或 append")
	}
	if c.Line == "" || len(c.Line) > 128 || strings.ContainsAny(c.Line, "\r\n\x00") {
		return errors.New("-hwline 必须为已有记录的准确线路")
	}
	if c.StartRow < 1 || c.EndRow < 0 || (c.EndRow > 0 && c.EndRow < c.StartRow) {
		return errors.New("DNS 排名范围无效：起始排名 >= 1，结束排名 >= 起始排名，0 表示至末尾")
	}
	var err error
	if c.Zone, err = normalizeCLIHuaweiName(c.Zone); err != nil {
		return fmt.Errorf("-hwzone: %w", err)
	}
	if c.Name, err = normalizeCLIHuaweiName(c.Name); err != nil {
		return fmt.Errorf("-hwname: %w", err)
	}
	if c.Name != c.Zone && !strings.HasSuffix(c.Name, "."+c.Zone) {
		return errors.New("-hwname 必须属于 -hwzone 指定的域名")
	}
	if _, err := parseCLIDatacenters(cfg.dc); err != nil {
		return err
	}
	return nil
}

func selectCLIHuaweiResults(cfg *cliConfig, results []TestResult) ([]TestResult, error) {
	centers, err := parseCLIDatacenters(cfg.dc)
	if err != nil {
		return nil, err
	}
	valid := make([]TestResult, 0, len(results))
	for _, row := range results {
		if row.Port == cfg.port && len(huaweiIPs([]TestResult{row}, cfg.huawei.Type)) != 0 {
			valid = append(valid, row)
		}
	}
	if len(valid) == 0 {
		return nil, nil
	}
	if len(centers) == 0 {
		seen := map[string]bool{}
		for _, row := range valid {
			if code, ok := normalizeDatacenterCode(row.DataCenter); ok && !seen[code] {
				centers = append(centers, code)
				seen[code] = true
			}
		}
		if len(centers) == 0 {
			return nil, nil
		}
	}
	maxLatency := cfg.delay
	if maxLatency <= 0 {
		maxLatency = int(min(int64(math.MaxInt), int64(math.MaxInt64)/int64(time.Millisecond)))
	}
	selected, err := selectOfficialResults(valid, officialSelectionPolicy{Datacenters: centers, MaxLatencyMS: maxLatency, MinSpeedMB: cfg.speedMin, Limit: len(valid)})
	if err != nil {
		return nil, err
	}
	start, end := cfg.huawei.StartRow, cfg.huawei.EndRow
	if start < 1 || end < 0 || (end > 0 && end < start) {
		return nil, errors.New("DNS 排名范围无效")
	}
	if start > len(selected) {
		return nil, nil
	}
	if end == 0 || end > len(selected) {
		end = len(selected)
	}
	return selected[start-1 : end], nil
}

func loadCLIHuaweiStore() (*huaweiDNSStore, error) {
	path, err := defaultDataPath("huawei-dns.private")
	if err != nil {
		return nil, err
	}
	store := &huaweiDNSStore{path: path}
	if err := store.load(); err != nil {
		return nil, fmt.Errorf("华为加密配置读取失败（保留原文件）: %w", err)
	}
	ak, sk := strings.TrimSpace(os.Getenv("CFDATA_HUAWEI_AK")), strings.TrimSpace(os.Getenv("CFDATA_HUAWEI_SK"))
	region, project := strings.TrimSpace(os.Getenv("CFDATA_HUAWEI_REGION")), strings.TrimSpace(os.Getenv("CFDATA_HUAWEI_PROJECT_ID"))
	if (ak == "") != (sk == "") {
		return nil, errors.New("替换华为凭证时须同时提供 CFDATA_HUAWEI_AK 和 CFDATA_HUAWEI_SK")
	}
	if ak != "" || region != "" || project != "" {
		c := store.config
		c.AK, c.SK, c.Persist = ak, sk, false
		if region != "" {
			c.Region = region
		}
		if project != "" {
			c.ProjectID = project
		}
		if err := store.configure(c); err != nil {
			return nil, err
		}
	}
	if err := validateHuaweiConfig(store.config); err != nil {
		return nil, errors.New("华为凭证未配置或无效：请用 CLI 菜单 7 / Web 保存加密凭证，或设置 CFDATA_HUAWEI_* 环境变量")
	}
	return store, nil
}

func confirmCLIHuaweiPreview(_ huaweiPreview) bool {
	fmt.Print("[Huawei DNS] 确认更新以上 IP？输入 y 更新，其他输入/EOF 不更新 > ")
	line, outcome := readMenuLine()
	return outcome == promptOK && strings.EqualFold(line, "y")
}

func updateCLIHuaweiDNS(ctx context.Context, store *huaweiDNSStore, cfg *cliConfig, results []TestResult, confirm func(huaweiPreview) bool) error {
	if !cfg.huawei.Enabled {
		return nil
	}
	if err := validateCLIHuaweiConfig(cfg); err != nil {
		return err
	}
	selected, err := selectCLIHuaweiResults(cfg, results)
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		fmt.Println("[Huawei DNS] 没有排名范围内的合格 IP，保留原 DNS，不进行更新")
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	c := cfg.huawei
	zones, err := store.zones(ctx)
	if err != nil {
		return err
	}
	zoneID := ""
	for _, zone := range zones {
		if strings.EqualFold(zone.Name, c.Zone) {
			if zoneID != "" {
				return errors.New("发现多个同名公网域名，请使用限定 DNS 权限的账户后重试")
			}
			zoneID = zone.ID
		}
	}
	if zoneID == "" {
		return errors.New("未发现指定的华为公网域名")
	}
	records, err := store.records(ctx, zoneID)
	if err != nil {
		return err
	}
	var target *huaweiRecord
	for _, record := range records {
		if strings.EqualFold(record.Name, c.Name) && record.Type == c.Type && record.Line == c.Line {
			if target != nil {
				return errors.New("同名、类型、线路匹配到多个记录，拒绝猜测更新目标")
			}
			copy := record
			target = &copy
		}
	}
	if target == nil {
		return errors.New("未发现指定域名/类型/线路的现有记录；不会自动创建记录")
	}
	p, err := store.preview(ctx, *target, selected)
	if err != nil {
		return err
	}
	if c.Mode == "append" {
		rows := make([]TestResult, 0, len(p.Old)+len(selected))
		for _, ip := range p.Old {
			rows = append(rows, TestResult{IP: ip})
		}
		rows = append(rows, selected...)
		p.New = huaweiIPs(rows, c.Type)
		p.Changed = !sameHuaweiIPSet(p.Old, p.New)
	}
	fmt.Printf("[Huawei DNS] 目标: %s %s / %s，TTL=%d，模式=%s\n", p.Target.Name, p.Target.Type, p.Target.Line, p.Target.TTL, c.Mode)
	fmt.Printf("[Huawei DNS] 原 IP: %s\n[Huawei DNS] 新 IP: %s\n", strings.Join(p.Old, ", "), strings.Join(p.New, ", "))
	if len(p.New) == 0 || !p.Changed {
		fmt.Println("[Huawei DNS] 无需更新，保留原记录")
		return nil
	}
	if !c.Confirmed && (confirm == nil || !confirm(p)) {
		fmt.Println("[Huawei DNS] 未确认，保留原记录")
		return nil
	}
	status, err := store.update(ctx, p, ctx.Err)
	if err != nil {
		return err
	}
	fmt.Printf("[Huawei DNS] %s\n", status)
	return nil
}
