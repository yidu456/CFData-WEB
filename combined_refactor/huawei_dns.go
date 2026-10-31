package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/config"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/region"
	dns "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/model"
	dnsregion "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/region"
)

const huaweiTimeout = 20 * time.Second

// Explicit SDK constants bypass environment/file region providers before signing.
var huaweiRegions = map[string]*region.Region{
	"cn-north-4": dnsregion.CN_NORTH_4, "cn-north-1": dnsregion.CN_NORTH_1,
	"cn-east-2": dnsregion.CN_EAST_2, "cn-east-3": dnsregion.CN_EAST_3,
	"cn-south-1": dnsregion.CN_SOUTH_1, "cn-southwest-2": dnsregion.CN_SOUTHWEST_2,
	"ap-southeast-1": dnsregion.AP_SOUTHEAST_1, "ap-southeast-2": dnsregion.AP_SOUTHEAST_2,
	"ap-southeast-3": dnsregion.AP_SOUTHEAST_3, "ap-southeast-4": dnsregion.AP_SOUTHEAST_4,
	"af-south-1": dnsregion.AF_SOUTH_1, "sa-brazil-1": dnsregion.SA_BRAZIL_1,
}

type huaweiConfigRequest struct {
	AK        string `json:"ak"`
	SK        string `json:"sk"`
	Region    string `json:"region"`
	ProjectID string `json:"projectId"`
	Persist   bool   `json:"persist"`
}

type huaweiConfigStatus struct {
	MaskedAK                 string   `json:"maskedAK"`
	HasSecret                bool     `json:"hasSecret"`
	Region                   string   `json:"region"`
	ProjectID                string   `json:"projectId"`
	Persisted                bool     `json:"persisted"`
	EncryptedSaveSupported   bool     `json:"encryptedSaveSupported"`
	PrivatePath              string   `json:"privatePath"`
	PrivateFileStatus        string   `json:"privateFileStatus"`
	LegacyPrivatePath        string   `json:"legacyPrivatePath"`
	LegacyFileStatus         string   `json:"legacyFileStatus"`
	LegacyMigrationSupported bool     `json:"legacyMigrationSupported"`
	LegacyCleanupReady       bool     `json:"legacyCleanupReady"`
	Regions                  []string `json:"regions"`
}

type huaweiZone struct {
	revision uint64
	ID       string `json:"id"`
	Name     string `json:"name"`
}
type huaweiRecord struct {
	revision uint64
	ID       string   `json:"id"`
	ZoneID   string   `json:"zoneId"`
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Line     string   `json:"line"`
	TTL      int32    `json:"ttl"`
	Records  []string `json:"records"`
}
type huaweiPreview struct {
	Token       string       `json:"token"`
	BatchID     uint64       `json:"batchId"`
	Target      huaweiRecord `json:"target"`
	Old         []string     `json:"old"`
	New         []string     `json:"new"`
	Changed     bool         `json:"changed"`
	revision    uint64
	fingerprint string
}

type huaweiDNSStore struct {
	mu                sync.Mutex
	config            huaweiConfigRequest
	revision          uint64
	savedConfig       huaweiConfigRequest
	privateFileStatus string
	path              string
	legacyPath        string
	legacyMigration   *huaweiMigrationReceipt
	transport         http.RoundTripper
}

var webHuaweiDNS = &huaweiDNSStore{}
var huaweiProjectIDPattern = regexp.MustCompile(`^[a-fA-F0-9]{32}$`)
var huaweiResourceIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`)

func validateHuaweiConfig(c huaweiConfigRequest) error {
	r, ok := huaweiRegions[c.Region]
	if !ok || len(r.Endpoints) != 1 || r.Endpoints[0] != "https://dns."+c.Region+".myhuaweicloud.com" {
		return errors.New("unsupported Huawei DNS region")
	}
	if !huaweiProjectIDPattern.MatchString(c.ProjectID) {
		return errors.New("project ID must be 32 hexadecimal characters for the selected region")
	}
	if c.AK == "" || c.SK == "" || len(c.AK) > 256 || len(c.SK) > 256 || strings.ContainsAny(c.AK+c.SK, "\r\n\x00") {
		return errors.New("Huawei AK/SK required (blank inputs preserve configured credentials)")
	}
	return nil
}

func (s *huaweiDNSStore) configure(c huaweiConfigRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c.AK, c.SK, c.Region, c.ProjectID = strings.TrimSpace(c.AK), strings.TrimSpace(c.SK), strings.TrimSpace(c.Region), strings.TrimSpace(c.ProjectID)
	if c.AK == "" {
		c.AK = s.config.AK
	}
	if c.SK == "" {
		c.SK = s.config.SK
	}
	if err := validateHuaweiConfig(c); err != nil {
		return err
	}
	if c.Persist {
		if !huaweiPrivateEncryptionSupported {
			return errors.New("encrypted Huawei configuration saving is unsupported on this platform; use memory-only Apply")
		}
		raw, err := json.Marshal(c)
		if err != nil {
			return errors.New("private configuration encoding failed")
		}
		if err := saveHuaweiPrivate(s.path, raw); err != nil {
			return errors.New("private configuration save failed")
		}
		s.savedConfig = c
		s.savedConfig.Persist = false
		s.privateFileStatus = "encrypted"
	}
	s.config = c
	s.revision++
	return nil
}

func (s *huaweiDNSStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, _, _, format, err := readHuaweiPrivateConfig(s.path)
	if errors.Is(err, os.ErrNotExist) {
		s.privateFileStatus = "absent"
		return nil
	}
	if err != nil {
		s.privateFileStatus = "unavailable"
		if privatePathStatus(s.path) == "available" {
			s.privateFileStatus = "invalid"
		}
		return errors.New("private configuration read failed")
	}
	s.config = c
	s.revision++
	s.savedConfig = c
	s.savedConfig.Persist = false
	s.privateFileStatus = format
	return nil
}

func (s *huaweiDNSStore) status() huaweiConfigStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	fileStatus := s.privateFileStatus
	if s.path == "" {
		fileStatus = "unavailable"
	} else if info, err := os.Stat(s.path); errors.Is(err, os.ErrNotExist) {
		fileStatus = "absent"
	} else if err != nil || !info.Mode().IsRegular() {
		fileStatus = "unavailable"
	} else if fileStatus == "" || fileStatus == "absent" {
		fileStatus = "unverified"
	}
	current := s.config
	current.Persist = false
	status := huaweiConfigStatus{HasSecret: s.config.SK != "", Region: s.config.Region, ProjectID: s.config.ProjectID, Persisted: (fileStatus == "encrypted" || fileStatus == "legacy-dpapi") && current == s.savedConfig, EncryptedSaveSupported: huaweiPrivateEncryptionSupported && s.path != "", PrivatePath: s.path, PrivateFileStatus: fileStatus, Regions: []string{}}
	status.LegacyPrivatePath = s.legacyPath
	status.LegacyFileStatus = privatePathStatus(s.legacyPath)
	status.LegacyMigrationSupported = huaweiPrivateEncryptionSupported && s.path != "" && s.legacyPath != "" && fileStatus == "absent" && status.LegacyFileStatus == "available"
	status.LegacyCleanupReady = s.legacyUnchangedLocked()
	if s.config.AK != "" {
		status.MaskedAK = "configured"
		if len(s.config.AK) > 8 {
			status.MaskedAK += " (ending " + s.config.AK[len(s.config.AK)-4:] + ")"
		}
	}
	for id := range huaweiRegions {
		status.Regions = append(status.Regions, id)
	}
	sort.Strings(status.Regions)
	return status
}

func initHuaweiPrivateConfig() {
	dir, err := os.UserConfigDir()
	if err == nil {
		webHuaweiDNS.legacyPath = filepath.Join(dir, "cfdata-web", "huawei-dns.private")
	}
	webHuaweiDNS.path, err = defaultDataPath("huawei-dns.private")
	if err != nil {
		webHuaweiDNS.privateFileStatus = "unavailable"
		fmt.Println("[Huawei DNS] executable data directory unavailable; use memory-only Apply")
		return
	}
	if err := webHuaweiDNS.load(); err != nil {
		fmt.Println("[Huawei DNS] private configuration unavailable; configure again in Web panel")
	}
}

func saveHuaweiPrivate(path string, raw []byte) error {
	return writeHuaweiPrivate(path, raw, true)
}

func writeHuaweiPrivate(path string, raw []byte, replace bool) error {
	if path == "" {
		return errors.New("private path unavailable")
	}
	raw, err := protectHuaweiPrivate(raw)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".huawei-private-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(raw)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if !replace {
		return os.Link(name, path)
	}
	return os.Rename(name, path)
}

type huaweiContextTransport struct {
	ctx         context.Context
	base        http.RoundTripper
	endpoint    string
	beforeWrite func() error
}

func (t huaweiContextTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if err := t.ctx.Err(); err != nil {
		return nil, err
	}
	if r.URL.Scheme != "https" || "https://"+r.URL.Host != t.endpoint {
		return nil, errors.New("unapproved DNS endpoint")
	}
	if r.Method != http.MethodGet && t.beforeWrite != nil {
		if err := t.beforeWrite(); err != nil {
			return nil, err
		}
	}
	if err := t.ctx.Err(); err != nil {
		return nil, err
	}
	return t.base.RoundTrip(r.WithContext(t.ctx))
}

func (s *huaweiDNSStore) client(ctx context.Context, beforeWrite func() error) (*dns.DnsClient, uint64, error) {
	s.mu.Lock()
	c, rev, base := s.config, s.revision, s.transport
	s.mu.Unlock()
	if err := validateHuaweiConfig(c); err != nil {
		return nil, 0, err
	}
	if base == nil {
		base = http.DefaultTransport
	}
	// DNS public API uses BasicCredentials; explicit project prevents automatic IAM discovery.
	cred, err := auth.NewBasicCredentialsBuilder().WithAk(c.AK).WithSk(c.SK).WithProjectId(c.ProjectID).SafeBuild()
	if err != nil {
		return nil, 0, errors.New("invalid Huawei credentials")
	}
	r := huaweiRegions[c.Region]
	httpConfig := config.DefaultHttpConfig().WithTimeout(huaweiTimeout).WithHttpRoundTripper(huaweiContextTransport{ctx: ctx, base: base, endpoint: r.Endpoints[0], beforeWrite: beforeWrite})
	hc, err := dns.DnsClientBuilder().WithCredential(cred).WithRegion(region.NewRegion(c.Region, r.Endpoints...)).WithHttpConfig(httpConfig).SafeBuild()
	if err != nil {
		return nil, 0, errors.New("Huawei DNS client configuration failed")
	}
	return dns.NewDnsClient(hc), rev, nil
}

func huaweiCloudError(ctx context.Context) error {
	if ctx.Err() != nil {
		return errors.New("Huawei DNS request cancelled or timed out; refresh cloud state before retry")
	}
	return errors.New("Huawei DNS request failed; check region, project ID, credentials and DNS permissions")
}

func (s *huaweiDNSStore) zones(ctx context.Context) ([]huaweiZone, error) {
	ctx, cancel := context.WithTimeout(ctx, huaweiTimeout)
	defer cancel()
	client, rev, err := s.client(ctx, nil)
	if err != nil {
		return nil, err
	}
	limit := int32(500)
	typ := "public"
	var marker *string
	out := []huaweiZone{}
	for page := 0; page < 20; page++ {
		r, err := client.ListPublicZones(&model.ListPublicZonesRequest{Type: &typ, Limit: &limit, Marker: marker})
		if err != nil {
			return nil, huaweiCloudError(ctx)
		}
		if r.Zones == nil {
			return out, nil
		}
		rows := *r.Zones
		for _, z := range rows {
			if z.Id != nil && z.Name != nil && z.ZoneType != nil && *z.ZoneType == "public" {
				out = append(out, huaweiZone{ID: *z.Id, Name: *z.Name, revision: rev})
			}
		}
		if len(rows) < int(limit) {
			return out, nil
		}
		if rows[len(rows)-1].Id == nil {
			return nil, errors.New("invalid Huawei pagination")
		}
		next := *rows[len(rows)-1].Id
		if marker != nil && next == *marker {
			return nil, errors.New("invalid Huawei pagination")
		}
		marker = &next
	}
	return nil, errors.New("zone discovery exceeds 10000 records; use a scoped DNS account")
}

func recordFromHuawei(r *model.ShowRecordSetWithLineResponse) (huaweiRecord, error) {
	if r.Id == nil || r.ZoneId == nil || r.Name == nil || r.Type == nil || r.Line == nil || r.Ttl == nil || r.Records == nil || r.Status == nil || *r.Status != "ACTIVE" || r.AliasTarget != nil || (*r.Type != "A" && *r.Type != "AAAA") {
		return huaweiRecord{}, errors.New("select an active existing non-alias A/AAAA record with line identity")
	}
	return huaweiRecord{ID: *r.Id, ZoneID: *r.ZoneId, Name: *r.Name, Type: *r.Type, Line: *r.Line, TTL: *r.Ttl, Records: append([]string{}, (*r.Records)...)}, nil
}

func (s *huaweiDNSStore) records(ctx context.Context, zoneID string) ([]huaweiRecord, error) {
	if !huaweiResourceIDPattern.MatchString(zoneID) {
		return nil, errors.New("invalid zone ID")
	}
	ctx, cancel := context.WithTimeout(ctx, huaweiTimeout)
	defer cancel()
	client, rev, err := s.client(ctx, nil)
	if err != nil {
		return nil, err
	}
	limit := int32(500)
	var marker *string
	out := []huaweiRecord{}
	for page := 0; page < 20; page++ {
		r, err := client.ShowRecordSetByZone(&model.ShowRecordSetByZoneRequest{ZoneId: zoneID, Limit: &limit, Marker: marker})
		if err != nil {
			return nil, huaweiCloudError(ctx)
		}
		if r.Recordsets == nil {
			return out, nil
		}
		rows := *r.Recordsets
		for _, row := range rows {
			data, _ := json.Marshal(row)
			var full model.ShowRecordSetWithLineResponse
			_ = json.Unmarshal(data, &full)
			if rec, err := recordFromHuawei(&full); err == nil && rec.ZoneID == zoneID {
				rec.revision = rev
				out = append(out, rec)
			}
		}
		if len(rows) < int(limit) {
			return out, nil
		}
		if rows[len(rows)-1].Id == nil {
			return nil, errors.New("invalid Huawei pagination")
		}
		next := *rows[len(rows)-1].Id
		if marker != nil && next == *marker {
			return nil, errors.New("invalid Huawei pagination")
		}
		marker = &next
	}
	return nil, errors.New("record discovery exceeds 10000 records")
}

func huaweiIPs(rows []TestResult, typ string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, row := range rows {
		addr, err := netip.ParseAddr(row.IP)
		if err != nil || addr.Zone() != "" || addr.Is4In6() || addr.IsUnspecified() || addr.IsMulticast() {
			continue
		}
		if (typ == "A" && !addr.Is4()) || (typ == "AAAA" && !addr.Is6()) || (typ != "A" && typ != "AAAA") {
			continue
		}
		ip := addr.String()
		if !seen[ip] {
			out = append(out, ip)
			seen[ip] = true
		}
	}
	return out
}
func sameHuaweiIPSet(a, b []string) bool {
	canon := func(values []string) []string {
		set := map[string]bool{}
		for _, v := range values {
			if ip, err := netip.ParseAddr(v); err == nil {
				set[ip.String()] = true
			} else {
				set[v] = true
			}
		}
		out := []string{}
		for v := range set {
			out = append(out, v)
		}
		sort.Strings(out)
		return out
	}
	return reflect.DeepEqual(canon(a), canon(b))
}
func huaweiFingerprint(r *model.ShowRecordSetWithLineResponse) string {
	raw, _ := json.Marshal(r)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func (s *huaweiDNSStore) preview(ctx context.Context, target huaweiRecord, results []TestResult) (huaweiPreview, error) {
	ctx, cancel := context.WithTimeout(ctx, huaweiTimeout)
	defer cancel()
	if !huaweiResourceIDPattern.MatchString(target.ZoneID) || !huaweiResourceIDPattern.MatchString(target.ID) {
		return huaweiPreview{}, errors.New("invalid target")
	}
	client, rev, err := s.client(ctx, nil)
	if err != nil {
		return huaweiPreview{}, err
	}
	if target.revision != rev {
		return huaweiPreview{}, errors.New("Huawei service configuration changed; rediscover and select the record again")
	}
	r, err := client.ShowRecordSetWithLine(&model.ShowRecordSetWithLineRequest{ZoneId: target.ZoneID, RecordsetId: target.ID})
	if err != nil {
		return huaweiPreview{}, huaweiCloudError(ctx)
	}
	current, err := recordFromHuawei(r)
	if err != nil {
		return huaweiPreview{}, err
	}
	current.revision = rev
	if current.ID != target.ID || current.ZoneID != target.ZoneID || current.Name != target.Name || current.Type != target.Type || current.Line != target.Line {
		return huaweiPreview{}, errors.New("selected record identity changed; discover and select again")
	}
	values := huaweiIPs(results, current.Type)
	return huaweiPreview{Target: current, Old: current.Records, New: values, Changed: len(values) > 0 && !sameHuaweiIPSet(current.Records, values), revision: rev, fingerprint: huaweiFingerprint(r)}, nil
}

func (s *huaweiDNSStore) update(ctx context.Context, p huaweiPreview, gate func() error) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, huaweiTimeout)
	defer cancel()
	if ctx.Err() != nil {
		return "", huaweiCloudError(ctx)
	}
	if err := gate(); err != nil {
		return "", err
	}
	check := func() error {
		if err := gate(); err != nil {
			return err
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.revision != p.revision {
			return errors.New("Huawei configuration changed; preview again")
		}
		return nil
	}
	client, rev, err := s.client(ctx, check)
	if err != nil {
		return "", err
	}
	if rev != p.revision {
		return "", errors.New("Huawei configuration changed; preview again")
	}
	r, err := client.ShowRecordSetWithLine(&model.ShowRecordSetWithLineRequest{ZoneId: p.Target.ZoneID, RecordsetId: p.Target.ID})
	if err != nil {
		return "", huaweiCloudError(ctx)
	}
	if huaweiFingerprint(r) != p.fingerprint {
		return "", errors.New("cloud record changed since preview; preview again")
	}
	if len(p.New) == 0 || sameHuaweiIPSet(p.Old, p.New) {
		return "skipped", nil
	}
	if err := check(); err != nil {
		return "", err
	}
	_, err = client.UpdateRecordSets(&model.UpdateRecordSetsRequest{ZoneId: p.Target.ZoneID, RecordsetId: p.Target.ID, Body: &model.UpdateRecordSetsReq{Name: p.Target.Name, Type: p.Target.Type, Records: &p.New}})
	if err != nil {
		return "", huaweiCloudError(ctx)
	}
	return "updated", nil
}
