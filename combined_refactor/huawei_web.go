package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type webPublishOptions struct {
	authorized   func() bool
	DNS          bool                `json:"dns"`
	GitHub       bool                `json:"github"`
	GitHubConfig githubUploadRequest `json:"githubConfig"`
	Format       string              `json:"format"`
}
type huaweiWebRequest struct {
	huaweiConfigRequest
	ZoneID    string `json:"zoneId"`
	RecordID  string `json:"recordId"`
	BatchID   uint64 `json:"batchId"`
	Token     string `json:"token"`
	Confirmed bool   `json:"confirmed"`
	webPublishOptions
}

func huaweiWebAuthorized(r *http.Request) bool {
	origin, err := url.Parse(r.Header.Get("Origin"))
	if err != nil || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" || origin.Path != "" {
		return false
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if origin.Scheme != scheme || !strings.EqualFold(origin.Host, r.Host) {
		return false
	}
	if authEnabled() {
		return isAuthenticated(r)
	}
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	localHost := strings.EqualFold(host, "localhost")
	if ip, err := netip.ParseAddr(host); err == nil {
		localHost = ip.IsLoopback()
	}
	remote, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip, err := netip.ParseAddr(remote)
	return localHost && err == nil && ip.IsLoopback()
}

func (s *appSession) refreshWebPublicationAfterReconnect() {
	s.taskMutex.Lock()
	defer s.taskMutex.Unlock()
	if s.webPublishContext != nil && s.webPublishContext.Err() != nil {
		s.invalidateWebPublicationLocked()
	}
}

func huaweiPermission(authorized func() bool) bool {
	return authorized != nil && authorized()
}

func (s *appSession) invalidateWebPublicationLocked() {
	if s.webPublishCancel != nil {
		s.webPublishCancel()
	}
	s.webPublishContext, s.webPublishCancel = context.WithCancel(context.Background())
	s.dnsPreview = nil
}
func (s *appSession) publicationContext(parent context.Context) (context.Context, context.CancelFunc) {
	s.taskMutex.Lock()
	if s.webPublishContext == nil {
		s.invalidateWebPublicationLocked()
	}
	base := s.webPublishContext
	s.taskMutex.Unlock()
	ctx, cancel := context.WithTimeout(base, 30*time.Second)
	stop := context.AfterFunc(parent, cancel)
	return ctx, func() { stop(); cancel() }
}
func (s *appSession) cancelWebPublication() {
	s.taskMutex.Lock()
	defer s.taskMutex.Unlock()
	if s.webPublishCancel != nil {
		s.webPublishCancel()
	}
}

func (s *appSession) claimWebPublication(id uint64, destination string, automatic bool, authorized func() bool) (webBatchSnapshot, error) {
	if id == 0 || !huaweiPermission(authorized) {
		return webBatchSnapshot{}, errors.New("publishing requires same-origin local access or an active login")
	}
	batch, err := s.completedWebBatchResults(id)
	if err != nil {
		return batch, err
	}
	s.taskMutex.Lock()
	defer s.taskMutex.Unlock()
	if s.isTaskRunning || s.webBatch == nil || s.webBatch.ID != id {
		return webBatchSnapshot{}, errors.New("batch changed")
	}
	if s.webPublishAttempts == nil {
		s.webPublishAttempts = map[string]bool{}
	}
	key := fmt.Sprintf("%d:%s", id, destination)
	if automatic && s.webPublishAttempts[key] {
		return webBatchSnapshot{}, errors.New("automatic publication already attempted")
	}
	s.webPublishAttempts[key] = true
	return batch, nil
}

func webGitHubContent(batch webBatchSnapshot, format string) (string, error) {
	if len(batch.Results) == 0 {
		return "", errors.New("empty selected results")
	}
	var out bytes.Buffer
	if format == "csv" {
		w := csv.NewWriter(&out)
		_ = w.Write([]string{"IP", "Port", "DataCenter", "Speed", "AvgLatencyMS"})
		for _, r := range batch.Results {
			_ = w.Write([]string{r.IP, strconv.Itoa(r.Port), r.DataCenter, r.Speed, strconv.FormatFloat(float64(r.AvgLatency)/float64(time.Millisecond), 'f', 2, 64)})
		}
		w.Flush()
		return out.String(), w.Error()
	}
	if format != "txt" {
		return "", errors.New("choose txt or csv")
	}
	for _, r := range batch.Results {
		fmt.Fprintf(&out, "%s#%s\n", net.JoinHostPort(r.IP, strconv.Itoa(r.Port)), r.DataCenter)
	}
	return out.String(), nil
}

func (s *appSession) publishStatus(destination string, id uint64, status, message string) {
	s.sendWSMessage("web_publish_status", map[string]interface{}{"destination": destination, "batchId": id, "status": status, "message": message})
}

func (s *appSession) publishWebDNS(parent context.Context, store *huaweiDNSStore, id uint64, p *huaweiPreview, automatic bool, authorized func() bool) {
	ctx, cancel := s.publicationContext(parent)
	defer cancel()
	batch, err := s.claimWebPublication(id, "dns", automatic, authorized)
	if err != nil {
		s.publishStatus("dns", id, "denied", err.Error())
		return
	}
	s.dnsMutex.Lock()
	defer s.dnsMutex.Unlock()
	if ctx.Err() != nil || !huaweiPermission(authorized) {
		s.publishStatus("dns", id, "denied", "Initiating connection permission expired or request cancelled")
		return
	}
	s.taskMutex.Lock()
	target := s.dnsTarget
	s.taskMutex.Unlock()
	if target == nil {
		s.publishStatus("dns", id, "failed", "Select an existing Huawei DNS record first")
		return
	}
	if automatic {
		preview, e := store.preview(ctx, *target, batch.Results)
		err = e
		p = &preview
	}
	if err != nil || p == nil {
		s.publishStatus("dns", id, "failed", "DNS preview failed; refresh configuration and target")
		return
	}
	gate := func() error {
		if ctx.Err() != nil {
			return context.Canceled
		}
		if !huaweiPermission(authorized) {
			return errors.New("DNS permission expired")
		}
		if _, err := s.completedWebBatchResults(id); err != nil {
			return err
		}
		s.taskMutex.Lock()
		defer s.taskMutex.Unlock()
		if s.dnsTarget == nil || s.dnsTarget.ID != p.Target.ID || s.dnsTarget.ZoneID != p.Target.ZoneID || s.dnsTarget.Name != p.Target.Name || s.dnsTarget.Type != p.Target.Type || s.dnsTarget.Line != p.Target.Line {
			return errors.New("DNS selection changed; preview again")
		}
		return nil
	}
	s.publishStatus("dns", id, "running", "Updating selected existing record (records only)")
	status, err := store.update(ctx, *p, gate)
	if err != nil {
		s.publishStatus("dns", id, "failed", err.Error())
		return
	}
	s.publishStatus("dns", id, status, "DNS "+status+"; TTL, line and other properties preserved")
}

func (s *appSession) publishWebGitHub(parent context.Context, id uint64, options webPublishOptions, automatic bool, authorized func() bool) {
	ctx, cancel := s.publicationContext(parent)
	defer cancel()
	batch, err := s.claimWebPublication(id, "github", automatic, authorized)
	if err != nil {
		s.publishStatus("github", id, "denied", err.Error())
		return
	}
	params := options.GitHubConfig
	params.Content, err = webGitHubContent(batch, options.Format)
	if err != nil {
		s.publishStatus("github", id, "failed", err.Error())
		return
	}
	s.publishStatus("github", id, "running", "Uploading backend-selected batch results")
	// Revalidate on every attempt, including the existing helper's GET -> PUT boundary.
	gate := func() error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !huaweiPermission(authorized) {
			return errors.New("publishing permission expired")
		}
		_, err := s.completedWebBatchResults(id)
		return err
	}
	params.beforeMutation = gate
	if err := gate(); err != nil {
		s.publishStatus("github", id, "denied", err.Error())
		return
	}
	_, err = uploadGitHubContentWithRetry(ctx, params, nil)
	if err != nil {
		s.publishStatus("github", id, "failed", "GitHub upload failed; check token/repository/branch and refresh remote state before retry")
		return
	}
	s.publishStatus("github", id, "uploaded", "GitHub selected batch uploaded")
}

func (s *appSession) autoPublishCompletedWebBatch() {
	batch, err := s.completedWebBatchResults(0)
	if err != nil {
		return
	}
	s.taskMutex.Lock()
	options := s.webPublishOptions
	// Disconnect cancels in-flight reads/writes, not an explicitly backgrounded batch's future opt-in.
	if s.backgroundTask && s.webPublishContext != nil && s.webPublishContext.Err() != nil {
		s.invalidateWebPublicationLocked()
	}
	s.taskMutex.Unlock()
	if !huaweiPermission(options.authorized) {
		return
	}
	if options.DNS {
		safeGo("web-dns-publish", s, func() { s.publishWebDNS(context.Background(), webHuaweiDNS, batch.ID, nil, true, options.authorized) })
	}
	if options.GitHub {
		safeGo("web-github-publish", s, func() { s.publishWebGitHub(context.Background(), batch.ID, options, true, options.authorized) })
	}
}

func handleHuaweiWebAction(parent context.Context, s *appSession, store *huaweiDNSStore, action string, data json.RawMessage, authorized func() bool) bool {
	if !strings.HasPrefix(action, "huawei_") && action != "web_publish_options" && action != "web_github_publish" {
		return false
	}
	fail := func(err error) {
		s.sendWSMessage("huawei_error", map[string]string{"action": action, "message": err.Error()})
	}
	if !huaweiPermission(authorized) {
		fail(errors.New("Huawei DNS and selected publishing require same-origin local access or active Web login"))
		return true
	}
	var req huaweiWebRequest
	if len(data) > 16384 || json.Unmarshal(data, &req) != nil {
		fail(errors.New("invalid Huawei/publishing request"))
		return true
	}
	reply := func(value interface{}) {
		s.sendWSMessage("huawei_response", map[string]interface{}{"action": action, "data": value})
	}
	switch action {
	case "huawei_config_get":
		reply(store.status())
	case "huawei_config_set":
		if err := store.configure(req.huaweiConfigRequest); err != nil {
			fail(err)
		} else {
			s.taskMutex.Lock()
			s.dnsZones, s.dnsRecords, s.dnsTarget, s.dnsPreview = nil, nil, nil, nil
			s.webPublishOptions.DNS = false
			s.taskMutex.Unlock()
			reply(store.status())
		}
	case "web_publish_options":
		req.GitHubConfig.Content = ""
		req.GitHubConfig.beforeMutation = nil
		if req.GitHub && (req.GitHubConfig.Token == "" || req.GitHubConfig.Owner == "" || req.GitHubConfig.Repo == "" || req.GitHubConfig.Path == "" || (req.Format != "txt" && req.Format != "csv")) {
			fail(errors.New("configure GitHub token, owner/repository, path and format first"))
			return true
		}
		s.taskMutex.Lock()
		req.webPublishOptions.authorized = authorized
		s.webPublishOptions = req.webPublishOptions
		s.taskMutex.Unlock()
		reply(map[string]bool{"dns": req.DNS, "github": req.GitHub})
	case "huawei_update":
		s.taskMutex.Lock()
		p := s.dnsPreview
		if p == nil || req.Token == "" || p.Token != req.Token || req.BatchID != p.BatchID || !req.Confirmed {
			s.taskMutex.Unlock()
			fail(errors.New("confirm a current server preview first"))
			return true
		}
		copy := *p
		s.dnsPreview = nil
		s.taskMutex.Unlock()
		safeGo("web-dns-publish", s, func() { s.publishWebDNS(parent, store, req.BatchID, &copy, false, authorized) })
	case "web_github_publish":
		safeGo("web-github-publish", s, func() { s.publishWebGitHub(parent, req.BatchID, req.webPublishOptions, false, authorized) })
	case "huawei_test", "huawei_zones", "huawei_records", "huawei_select", "huawei_preview":
		ctx, cancel := s.publicationContext(parent)
		safeGo("huawei-read", s, func() {
			defer cancel()
			s.dnsMutex.Lock()
			defer s.dnsMutex.Unlock()
			if ctx.Err() != nil || !huaweiPermission(authorized) {
				fail(errors.New("request cancelled or permission expired"))
				return
			}
			switch action {
			case "huawei_test", "huawei_zones":
				zones, err := store.zones(ctx)
				if err != nil {
					fail(err)
					return
				}
				s.taskMutex.Lock()
				s.dnsZones = zones
				s.taskMutex.Unlock()
				reply(zones)
			case "huawei_records":
				store.mu.Lock()
				rev := store.revision
				store.mu.Unlock()
				s.taskMutex.Lock()
				allowed := false
				for _, z := range s.dnsZones {
					if z.ID == req.ZoneID && z.revision == rev {
						allowed = true
					}
				}
				s.taskMutex.Unlock()
				if !allowed {
					fail(errors.New("discover a public zone first"))
					return
				}
				records, err := store.records(ctx, req.ZoneID)
				if err != nil {
					fail(err)
					return
				}
				s.taskMutex.Lock()
				s.dnsRecords = records
				s.taskMutex.Unlock()
				reply(records)
			case "huawei_select":
				s.taskMutex.Lock()
				s.dnsTarget = nil
				s.dnsPreview = nil
				for _, rec := range s.dnsRecords {
					if rec.ID == req.RecordID && rec.ZoneID == req.ZoneID {
						copy := rec
						s.dnsTarget = &copy
						break
					}
				}
				target := s.dnsTarget
				s.taskMutex.Unlock()
				if target == nil {
					fail(errors.New("select a discovered existing A/AAAA record"))
					return
				}
				reply(*target)
			case "huawei_preview":
				if req.BatchID == 0 {
					fail(errors.New("completed same-session batch required"))
					return
				}
				batch, err := s.completedWebBatchResults(req.BatchID)
				if err != nil {
					fail(err)
					return
				}
				s.taskMutex.Lock()
				target := s.dnsTarget
				s.taskMutex.Unlock()
				if target == nil {
					fail(errors.New("select an existing A/AAAA record first"))
					return
				}
				p, err := store.preview(ctx, *target, batch.Results)
				if err != nil {
					fail(err)
					return
				}
				p.Token, err = newSessionToken()
				if err != nil {
					fail(errors.New("preview token unavailable"))
					return
				}
				p.BatchID = batch.ID
				if ctx.Err() != nil {
					fail(errors.New("preview invalidated"))
					return
				}
				if _, err := s.completedWebBatchResults(batch.ID); err != nil {
					fail(err)
					return
				}
				s.taskMutex.Lock()
				s.dnsPreview = &p
				s.taskMutex.Unlock()
				reply(p)
			}
		})
	default:
		fail(errors.New("unknown Huawei action"))
	}
	return true
}
