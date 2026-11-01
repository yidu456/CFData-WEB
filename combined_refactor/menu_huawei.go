package main

import (
	"fmt"
	"strconv"
	"strings"
)

func runCLIHuaweiWizard(c *cliHuaweiConfig) int {
	next := *c
	next.Confirmed = false
	sectionHeader("华为 DNS 更新")
	idx, outcome := askChoice("华为 DNS", "仅官方测速；合格结果合并排序后选择排名范围，更新前须本次确认", []string{"启用", "不启用"}, boolChoiceIndex(next.Enabled))
	if outcome != promptOK {
		return outcome
	}
	next.Enabled = idx == 0
	if !next.Enabled {
		*c = next
		return promptOK
	}
	for _, field := range []struct {
		label, hint string
		value       *string
	}{
		{"公网域名", "如 example.com，填写域名，不是 Zone ID", &next.Zone},
		{"记录完整域名", "如 edge.example.com；须已存在，且属于上述公网域名", &next.Name},
	} {
		for {
			v, o := askLine(field.label, displayOrEmpty(*field.value), field.hint)
			if o != promptOK {
				return o
			}
			if v == "" {
				v = *field.value
			}
			name, err := normalizeCLIHuaweiName(v)
			if err != nil {
				fmt.Printf("    %v\n", err)
				continue
			}
			if field.value == &next.Name && name != next.Zone && !strings.HasSuffix(name, "."+next.Zone) {
				fmt.Println("    记录完整域名必须属于公网域名")
				continue
			}
			*field.value = name
			break
		}
	}
	typeIdx := 0
	if next.Type == "AAAA" {
		typeIdx = 1
	}
	idx, outcome = askChoice("记录类型", "A 对应 offiptype=4，AAAA 对应 offiptype=6", []string{"A", "AAAA"}, typeIdx)
	if outcome != promptOK {
		return outcome
	}
	next.Type = []string{"A", "AAAA"}[idx]
	v, outcome := askLine("记录线路", displayOrEmpty(next.Line), "准确填写已有记录的线路，普通默认线路是 default_view")
	if outcome != promptOK {
		return outcome
	}
	if v != "" {
		next.Line = v
	}
	modeIdx := 0
	if next.Mode == "append" {
		modeIdx = 1
	}
	idx, outcome = askChoice("IP 更新方式", "只修改 IP，不修改 TTL/线路/权重；追加保留现有 IP 并去重", []string{"覆盖（overwrite）", "追加（append）"}, modeIdx)
	if outcome != promptOK {
		return outcome
	}
	next.Mode = []string{"overwrite", "append"}[idx]
	start, outcome := askInt("DNS 起始排名", strconv.Itoa(next.StartRow), "所有所选中心的合格 IP 合并，速度降序，同速延迟升序；不是每中心取前 N 个", next.StartRow, 1, 0)
	if outcome != promptOK {
		return outcome
	}
	next.StartRow = start
	for {
		end, o := askInt("DNS 结束排名", strconv.Itoa(next.EndRow), "含结束排名；0 至末尾；1..10 表示全局合格结果前 10 个，与导出范围独立", next.EndRow, 0, 0)
		if o != promptOK {
			return o
		}
		if end > 0 && end < next.StartRow {
			fmt.Println("    结束排名不能小于起始排名")
			continue
		}
		next.EndRow = end
		break
	}
	fmt.Println("    AK/SK 不写入本配置，请用主菜单 7 保存加密凭证；测速达标上限需足够大，才可能填满 DNS 排名范围")
	*c = next
	return promptOK
}

func configureCLIHuaweiCredentials(store *huaweiDNSStore) int {
	sectionHeader("华为 DNS 凭证（独立加密存储）")
	fmt.Println("AK/SK 不写入 CLI 配置或日志。终端输入可能回显；不共享屏幕，也可用 CFDATA_HUAWEI_* 环境变量仅内存配置。")
	store.mu.Lock()
	c := store.config
	store.mu.Unlock()
	for _, field := range []struct {
		label string
		value *string
	}{{"Huawei AK", &c.AK}, {"Huawei SK", &c.SK}} {
		display := "未设置"
		if *field.value != "" {
			display = "已设置（不显示）"
		}
		v, outcome := askLine(field.label, display, "输入替换，直接回车保留已有值；b 返回，不修改原配置")
		if outcome != promptOK {
			return outcome
		}
		if v != "" {
			*field.value = v
		}
	}
	if c.Region == "" {
		c.Region = "cn-north-4"
	}
	v, outcome := askLine("华为区域", c.Region, "使用区域的 DNS 项目鉴权，例如 cn-north-4；不接受自定义 API 端点")
	if outcome != promptOK {
		return outcome
	}
	if v != "" {
		c.Region = v
	}
	v, outcome = askLine("Project ID", displayOrEmpty(c.ProjectID), "上述区域的 32 位十六进制项目 ID，不是账号 ID")
	if outcome != promptOK {
		return outcome
	}
	if v != "" {
		c.ProjectID = v
	}
	if err := validateHuaweiConfig(c); err != nil {
		fmt.Printf("[Huawei DNS] 未应用配置: %v\n", err)
		return promptBack
	}
	idx, outcome := askChoice("凭证保存方式", "加密保存可在重启后复用；仅内存应用只对当前 CLI 菜单进程有效，不访问云端", []string{"仅内存应用", "保存加密配置"}, 1)
	if outcome != promptOK {
		return outcome
	}
	c.Persist = idx == 1
	if err := store.configure(c); err != nil {
		fmt.Printf("[Huawei DNS] 未应用配置，原配置保留: %v\n", err)
		return promptBack
	}
	if c.Persist {
		fmt.Printf("[Huawei DNS] 已保存加密凭证: %s\n", store.path)
	} else {
		fmt.Println("[Huawei DNS] 已应用仅内存凭证，退出当前菜单进程后失效")
	}
	return promptOK
}
