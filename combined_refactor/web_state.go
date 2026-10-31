package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

const webStateMaxBytes = 256 << 10

type webStateDocument struct {
	Version  int                        `json:"version"`
	Revision uint64                     `json:"revision"`
	Sections map[string]json.RawMessage `json:"sections"`
}

type webStateStore struct {
	mu                           sync.Mutex
	path                         string
	document                     webStateDocument
	fileHash                     [32]byte
	fileStatus                   string
	migrationToken, sourceDigest string
}

var webState = &webStateStore{}
var webStateDC = regexp.MustCompile(`^[A-Z]{3}$`)
var webStateRepo = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
var webStateHex = regexp.MustCompile(`^[a-f0-9]{64}$`)
var webStateRequestID = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,80}$`)

func initWebState() {
	path, err := defaultDataPath("web-state.private")
	webState.path = path
	if err != nil || webState.load() != nil {
		fmt.Println("[Web state] encrypted storage unavailable; memory-only preferences remain usable")
	}
}

func decodeWebState(raw []byte, target interface{}) error {
	if len(raw) > webStateMaxBytes {
		return errors.New("invalid")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(target) != nil {
		return errors.New("invalid")
	}
	if d.Decode(new(interface{})) != io.EOF {
		return errors.New("invalid")
	}
	return nil
}

func readWebStateFile(path string) ([]byte, error) {
	if path == "" {
		return nil, errors.New("unavailable")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, webStateMaxBytes*2+1))
	if err != nil || len(raw) > webStateMaxBytes*2 {
		return nil, errors.New("unavailable")
	}
	return raw, nil
}

func (s *webStateStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := readWebStateFile(s.path)
	if os.IsNotExist(err) {
		s.fileStatus = "absent"
		return nil
	}
	if err != nil {
		s.fileStatus = "unavailable"
		return errors.New("unavailable")
	}
	plain, err := unprotectHuaweiPrivate(raw)
	var doc webStateDocument
	if err != nil || decodeWebState(plain, &doc) != nil || doc.Version != 1 || doc.Revision == 0 || validateWebStateSections(doc.Sections) != nil {
		s.fileStatus = "unavailable"
		return errors.New("unavailable")
	}
	s.document, s.fileHash, s.fileStatus = doc, sha256.Sum256(raw), "encrypted"
	return nil
}

func (s *webStateStore) snapshotLocked() webStateDocument {
	doc := webStateDocument{Version: 1, Revision: s.document.Revision, Sections: map[string]json.RawMessage{}}
	for key, value := range s.document.Sections {
		doc.Sections[key] = append(json.RawMessage(nil), value...)
	}
	return doc
}

func (s *webStateStore) snapshot() webStateDocument {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotLocked()
}

func (s *webStateStore) verifyFileLocked() error {
	raw, err := readWebStateFile(s.path)
	if s.document.Revision == 0 {
		if os.IsNotExist(err) {
			return nil
		}
	} else if err == nil && sha256.Sum256(raw) == s.fileHash {
		return nil
	}
	s.fileStatus = "unavailable"
	return errors.New("unavailable")
}

func (s *webStateStore) patch(revision uint64, sections map[string]json.RawMessage, migrate bool, authorized func() bool) (webStateDocument, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !huaweiPermission(authorized) {
		return webStateDocument{}, errors.New("unauthorized")
	}
	if validateWebStateSections(sections) != nil || len(sections) == 0 {
		return webStateDocument{}, errors.New("invalid")
	}
	if revision != s.document.Revision || revision == ^uint64(0) {
		return webStateDocument{}, errors.New("stale")
	}
	if s.path == "" || s.fileStatus == "unavailable" || !huaweiPrivateEncryptionSupported {
		return webStateDocument{}, errors.New("unavailable")
	}
	raw, err := readWebStateFile(s.path)
	if migrate && (!os.IsNotExist(err) || s.document.Revision != 0) {
		return webStateDocument{}, errors.New("exists")
	}
	if s.document.Revision > 0 {
		if err != nil || sha256.Sum256(raw) != s.fileHash {
			return webStateDocument{}, errors.New("stale")
		}
	} else if !os.IsNotExist(err) {
		return webStateDocument{}, errors.New("stale")
	}
	doc := s.snapshotLocked()
	for key, value := range sections {
		doc.Sections[key] = append(json.RawMessage(nil), value...)
	}
	doc.Revision++
	plain, err := json.Marshal(doc)
	if err != nil || len(plain) > webStateMaxBytes {
		return webStateDocument{}, errors.New("invalid")
	}
	if migrate {
		err = writeHuaweiPrivate(s.path, plain, false)
	} else {
		err = saveHuaweiPrivate(s.path, plain)
	}
	if err != nil {
		return webStateDocument{}, errors.New("save_failed")
	}
	raw, err = readWebStateFile(s.path)
	verified, e := unprotectHuaweiPrivate(raw)
	if err != nil || e != nil || !bytes.Equal(verified, plain) {
		s.fileStatus = "unavailable"
		return webStateDocument{}, errors.New("save_failed")
	}
	s.document, s.fileHash, s.fileStatus = doc, sha256.Sum256(raw), "encrypted"
	s.migrationToken, s.sourceDigest = "", ""
	return s.snapshotLocked(), nil
}

func webStateString(raw json.RawMessage, max int) (string, error) {
	var value string
	if json.Unmarshal(raw, &value) != nil || len(value) > max || strings.ContainsAny(value, "\x00\r\n") {
		return "", errors.New("invalid")
	}
	return value, nil
}

func webStateURL(value string) bool {
	if value == "" || value == "auto" {
		return true
	}
	if !strings.Contains(value, "://") {
		value = "https://" + value
	}
	u, e := url.Parse(value)
	return e == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != "" && u.User == nil && u.Fragment == ""
}

func validateWebStateSections(sections map[string]json.RawMessage) error {
	invalid := errors.New("invalid")
	if sections == nil || len(sections) > 5 {
		return invalid
	}
	for key, raw := range sections {
		if len(raw) > webStateMaxBytes {
			return invalid
		}
		switch key {
		case "theme", "autoload":
			value, e := webStateString(raw, 32)
			if e != nil || (key == "theme" && value != "auto" && value != "light" && value != "dark") || (key == "autoload" && value != "off" && value != "backend" && value != "configFile") {
				return invalid
			}
		case "config":
			if string(raw) == "null" {
				continue
			}
			var groups map[string]json.RawMessage
			if json.Unmarshal(raw, &groups) != nil || groups == nil || len(groups) > 3 {
				return invalid
			}
			for group, fields := range groups {
				if group == "selectedDatacenters" {
					var dcs []string
					if json.Unmarshal(fields, &dcs) != nil || len(dcs) > 100 {
						return invalid
					}
					seen := map[string]bool{}
					for _, dc := range dcs {
						if !webStateDC.MatchString(dc) || seen[dc] {
							return invalid
						}
						seen[dc] = true
					}
					continue
				}
				if group != "official" && group != "nsb" {
					return invalid
				}
				var values map[string]json.RawMessage
				if json.Unmarshal(fields, &values) != nil || values == nil {
					return invalid
				}
				for field, encoded := range values {
					value, e := webStateString(encoded, 2048)
					if e != nil {
						return invalid
					}
					allowed := false
					switch field {
					case "scanMode":
						allowed = value == "" || value == "tcping" || value == "httping"
					case "ipType":
						allowed = group == "official" && (value == "" || value == "4" || value == "6")
					case "autoSpeed":
						allowed = group == "official" && (value == "" || value == "on" || value == "off")
					case "enableSpeed", "tls":
						allowed = group == "nsb" && (value == "" || value == "on" || value == "off")
					case "compactView":
						allowed = group == "nsb" && (value == "" || value == "compact" || value == "full")
					case "speedUrl":
						allowed = webStateURL(value)
					case "sourceUrl":
						allowed = group == "nsb" && webStateURL(value)
					case "targetDC":
						allowed = value == "" || webStateDC.MatchString(value)
					case "outFile":
						allowed = group == "nsb" && len(value) <= 255 && !strings.ContainsAny(value, `/\:`)
					case "relinkPreset":
						allowed = group == "nsb" && (value == "" || value == "off" || value == "default" || value == "custom")
					case "relinkHost":
						allowed = group == "nsb" && len(value) <= 255
					case "port", "fallbackPort", "threads", "delay", "speedMin", "speedLimit", "finalLimit", "resultLimit", "speedTest":
						limit := float64(1000000)
						if field == "port" || field == "fallbackPort" {
							limit = 65535
						}
						n, e := strconv.ParseFloat(value, 64)
						allowed = value == "" || (e == nil && n >= 0 && n <= limit && (field == "speedMin" || n == float64(int64(n))))
						if (field == "port" || field == "finalLimit" || field == "autoSpeed") && group != "official" {
							allowed = false
						}
						if (field == "fallbackPort" || field == "resultLimit" || field == "speedTest") && group != "nsb" {
							allowed = false
						}
					default:
						return invalid
					}
					if !allowed {
						return invalid
					}
				}
			}
		case "githubAccounts", "edgetunnelAccounts":
			var accounts []map[string]json.RawMessage
			if json.Unmarshal(raw, &accounts) != nil || accounts == nil || len(accounts) > 50 {
				return invalid
			}
			for _, account := range accounts {
				if account == nil {
					return invalid
				}
				for field, encoded := range account {
					value, e := webStateString(encoded, 4096)
					if e != nil {
						return invalid
					}
					allowed := false
					if key == "githubAccounts" {
						switch field {
						case "name", "branch", "path", "message", "token":
							allowed = true
						case "repo":
							allowed = webStateRepo.MatchString(value)
						case "tokenHash":
							allowed = value == "" || webStateHex.MatchString(value)
						case "tokenCipher":
							allowed = validWebStateCipher(value, ".")
						}
					} else {
						switch field {
						case "name", "password":
							allowed = true
						case "host":
							allowed = value != "" && webStateURL(value)
						case "mode":
							allowed = value == "overwrite" || value == "append"
						case "passwordCipher":
							allowed = validWebStateCipher(value, ":")
						}
					}
					if !allowed {
						return invalid
					}
				}
				required := "repo"
				if key == "edgetunnelAccounts" {
					required = "host"
				}
				if _, ok := account[required]; !ok {
					return invalid
				}
			}
		default:
			return invalid
		}
	}
	return nil
}

func validWebStateCipher(value, separator string) bool {
	parts := strings.Split(value, separator)
	if len(parts) != 2 {
		return false
	}
	iv, e := base64.StdEncoding.DecodeString(parts[0])
	data, e2 := base64.StdEncoding.DecodeString(parts[1])
	return e == nil && e2 == nil && len(iv) == 12 && len(data) >= 16
}

type webStateRequest struct {
	RequestID      string                     `json:"requestId"`
	Content        bool                       `json:"content"`
	Revision       uint64                     `json:"revision"`
	Sections       map[string]json.RawMessage `json:"sections"`
	SourceDigest   string                     `json:"sourceDigest"`
	MigrationToken string                     `json:"migrationToken"`
}

func handleWebStateAction(s *webStateStore, action string, raw json.RawMessage, authorized func() bool, emit func(string, interface{})) bool {
	switch action {
	case "web_state_get", "web_state_patch", "web_state_migrate", "web_state_legacy_cleanup_ready":
	default:
		return false
	}
	var request webStateRequest
	err := decodeWebState(raw, &request)
	if !webStateRequestID.MatchString(request.RequestID) {
		request.RequestID = ""
		err = errors.New("invalid")
	}
	response := map[string]interface{}{"action": action, "requestId": request.RequestID, "ok": false}
	fail := func(code string) { response["code"] = code; emit("web_state_response", response) }
	if !huaweiPermission(authorized) {
		fail("unauthorized")
		return true
	}
	if err != nil || request.RequestID == "" {
		fail("invalid")
		return true
	}
	switch action {
	case "web_state_get":
		if request.Revision != 0 || len(request.Sections) != 0 || request.SourceDigest != "" || request.MigrationToken != "" {
			fail("invalid")
			return true
		}
		s.mu.Lock()
		if !huaweiPermission(authorized) {
			s.mu.Unlock()
			fail("unauthorized")
			return true
		}
		if e := s.verifyFileLocked(); e != nil {
			s.mu.Unlock()
			fail(e.Error())
			return true
		}
		status := s.fileStatus
		if status == "" {
			status = "absent"
		}
		response["metadata"] = map[string]interface{}{"privatePath": s.path, "privateFileStatus": status, "encryptedSaveSupported": huaweiPrivateEncryptionSupported, "revision": s.document.Revision}
		if request.Content {
			response["sections"] = s.snapshotLocked().Sections
		}
		s.mu.Unlock()
	case "web_state_patch", "web_state_migrate":
		if request.Content || request.MigrationToken != "" || (action == "web_state_patch" && request.SourceDigest != "") {
			fail("invalid")
			return true
		}
		if action == "web_state_migrate" && !webStateHex.MatchString(request.SourceDigest) {
			fail("invalid")
			return true
		}
		doc, e := s.patch(request.Revision, request.Sections, action == "web_state_migrate", authorized)
		if e != nil {
			fail(e.Error())
			return true
		}
		response["revision"] = doc.Revision
		if action == "web_state_migrate" {
			token := make([]byte, 32)
			if _, e := rand.Read(token); e != nil {
				fail("save_failed")
				return true
			}
			s.mu.Lock()
			if s.document.Revision != doc.Revision {
				s.mu.Unlock()
				fail("stale")
				return true
			}
			s.migrationToken, s.sourceDigest = hex.EncodeToString(token), request.SourceDigest
			response["migrationToken"] = s.migrationToken
			s.mu.Unlock()
		}
	case "web_state_legacy_cleanup_ready":
		if request.Content || len(request.Sections) != 0 || !webStateHex.MatchString(request.SourceDigest) || request.MigrationToken == "" {
			fail("invalid")
			return true
		}
		s.mu.Lock()
		if !huaweiPermission(authorized) {
			s.mu.Unlock()
			fail("unauthorized")
			return true
		}
		raw, e := readWebStateFile(s.path)
		if s.migrationToken == "" || request.MigrationToken != s.migrationToken || request.SourceDigest != s.sourceDigest || request.Revision != s.document.Revision || e != nil || sha256.Sum256(raw) != s.fileHash {
			s.mu.Unlock()
			fail("stale")
			return true
		}
		response["cleanupReady"] = true
		s.mu.Unlock()
	}
	response["ok"] = true
	emit("web_state_response", response)
	return true
}
