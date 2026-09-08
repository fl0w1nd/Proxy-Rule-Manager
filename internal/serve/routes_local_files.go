package serve

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/util"
)

const localFileMaxBytes = 4 << 20

type localFileItem struct {
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	Lines      int    `json:"lines"`
	ModifiedAt string `json:"modified_at"`
}

type localFileDetail struct {
	localFileItem
	Content string `json:"content"`
}

type localFileWriteRequest struct {
	Name    string  `json:"name,omitempty"`
	Content *string `json:"content"`
}

type localFileRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (s *Server) localFilesDir() string {
	return filepath.Join(s.DataDir, "local")
}

func (s *Server) handleLocalFiles(w http.ResponseWriter, _ *http.Request) {
	items, err := s.listLocalFiles()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "local_file_list_failed", "读取本地文件失败", map[string]any{})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleLocalFile(w http.ResponseWriter, r *http.Request) {
	path, ok := s.localFilePath(w, chi.URLParam(r, "name"))
	if !ok {
		return
	}
	item, content, err := readLocalFile(path)
	if errors.Is(err, os.ErrNotExist) {
		writeAPIError(w, http.StatusNotFound, "file_not_found", "找不到该本地文件", map[string]any{})
		return
	}
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "local_file_read_failed", "读取本地文件失败", map[string]any{})
		return
	}
	writeJSON(w, http.StatusOK, localFileDetail{localFileItem: item, Content: content})
}

func (s *Server) handleLocalFileCreate(w http.ResponseWriter, r *http.Request) {
	var request localFileWriteRequest
	if !decodeLocalFileRequest(w, r, &request) {
		return
	}
	name, ok := s.checkedLocalFileName(w, request.Name)
	if !ok {
		return
	}
	content, ok := localFileContent(w, request.Content)
	if !ok {
		return
	}
	path, err := util.JoinInside(s.localFilesDir(), name)
	if err != nil {
		writeInvalidLocalFileName(w)
		return
	}
	if _, err := os.Lstat(path); err == nil {
		writeAPIError(w, http.StatusConflict, "file_exists", "文件已经存在", map[string]any{"name": name})
		return
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		writeAPIError(w, http.StatusInternalServerError, "local_file_write_failed", "写入本地文件失败", map[string]any{})
		return
	}
	detail, err := writeLocalFile(path, content)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "local_file_write_failed", "写入本地文件失败", map[string]any{})
		return
	}
	writeJSON(w, http.StatusCreated, detail)
}

func (s *Server) handleLocalFileUpdate(w http.ResponseWriter, r *http.Request) {
	path, ok := s.localFilePath(w, chi.URLParam(r, "name"))
	if !ok {
		return
	}
	var request localFileWriteRequest
	if !decodeLocalFileRequest(w, r, &request) {
		return
	}
	content, ok := localFileContent(w, request.Content)
	if !ok {
		return
	}
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		writeAPIError(w, http.StatusNotFound, "file_not_found", "找不到该本地文件", map[string]any{})
		return
	} else if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "local_file_write_failed", "写入本地文件失败", map[string]any{})
		return
	}
	detail, err := writeLocalFile(path, content)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "local_file_write_failed", "写入本地文件失败", map[string]any{})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleLocalFileDelete(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	path, ok := s.localFilePath(w, name)
	if !ok {
		return
	}
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		writeAPIError(w, http.StatusNotFound, "file_not_found", "找不到该本地文件", map[string]any{})
		return
	} else if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "local_file_delete_failed", "删除本地文件失败", map[string]any{})
		return
	}
	if refs := s.localFileRefs(name); len(refs) > 0 {
		writeAPIError(w, http.StatusConflict, "file_in_use", "文件正被规则引用，无法删除", map[string]any{"rules": refs})
		return
	}
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeAPIError(w, http.StatusNotFound, "file_not_found", "找不到该本地文件", map[string]any{})
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "local_file_delete_failed", "删除本地文件失败", map[string]any{})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": filepath.Base(path)})
}

func (s *Server) listLocalFiles() ([]localFileItem, error) {
	dir := s.localFilesDir()
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []localFileItem{}, nil
	}
	if err != nil {
		return nil, err
	}
	items := make([]localFileItem, 0, len(entries))
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		if _, err := parseLocalFileName(entry.Name()); err != nil {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		items = append(items, localFileMeta(entry.Name(), info, content))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

func (s *Server) localFilePath(w http.ResponseWriter, raw string) (string, bool) {
	name, ok := s.checkedLocalFileName(w, raw)
	if !ok {
		return "", false
	}
	path, err := util.JoinInside(s.localFilesDir(), name)
	if err != nil {
		writeInvalidLocalFileName(w)
		return "", false
	}
	return path, true
}

func (s *Server) checkedLocalFileName(w http.ResponseWriter, raw string) (string, bool) {
	name, err := parseLocalFileName(raw)
	if err != nil {
		writeInvalidLocalFileName(w)
		return "", false
	}
	return name, true
}

func (s *Server) localFileRefs(name string) []localFileRef {
	cfg := s.config()
	resolve := config.NewLocalFileResolver(s.DataDir)
	target, targetErr := resolve(name)
	refs := make([]localFileRef, 0)
	seen := map[string]bool{}
	for _, rule := range cfg.Rules {
		for _, source := range rule.Sources {
			if source.File == "" || seen[rule.ID] {
				continue
			}
			matched := false
			if targetErr == nil {
				if resolved, err := resolve(source.File); err == nil && resolved == target {
					matched = true
				}
			} else if source.File == name {
				matched = true
			}
			if !matched {
				continue
			}
			seen[rule.ID] = true
			refs = append(refs, localFileRef{ID: rule.ID, Name: rule.Name})
		}
	}
	return refs
}

func parseLocalFileName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if err := util.EnsureSafeSegment(name, "filename"); err != nil {
		return "", err
	}
	if name != filepath.Base(name) || strings.HasPrefix(name, ".") || len(name) > 255 {
		return "", fmt.Errorf("invalid filename %q", name)
	}
	for _, r := range name {
		if r < 32 || r == 127 {
			return "", fmt.Errorf("invalid filename %q", name)
		}
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".list", ".yaml", ".txt":
		return name, nil
	default:
		return "", fmt.Errorf("unsupported file extension")
	}
}

func localFileContent(w http.ResponseWriter, content *string) (string, bool) {
	if content == nil {
		writeAPIError(w, http.StatusUnprocessableEntity, "invalid_request", "请求内容无效", map[string]any{
			"errors": []configIssue{{Path: "content", Message: "required"}},
		})
		return "", false
	}
	if len(*content) > localFileMaxBytes {
		writeAPIError(w, http.StatusRequestEntityTooLarge, "request_too_large", "文件内容超过 4 MiB", map[string]any{})
		return "", false
	}
	if !utf8.ValidString(*content) {
		writeAPIError(w, http.StatusUnprocessableEntity, "invalid_request", "文件内容必须是 UTF-8 文本", map[string]any{
			"errors": []configIssue{{Path: "content", Message: "must be valid UTF-8"}},
		})
		return "", false
	}
	return *content, true
}

func decodeLocalFileRequest(w http.ResponseWriter, r *http.Request, request *localFileWriteRequest) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeAPIError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "请求需要 application/json", map[string]any{})
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, localFileMaxBytes+64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(request); err != nil {
		var maxBytes *http.MaxBytesError
		if errors.As(err, &maxBytes) {
			writeAPIError(w, http.StatusRequestEntityTooLarge, "request_too_large", "文件内容超过 4 MiB", map[string]any{})
		} else {
			writeAPIError(w, http.StatusUnprocessableEntity, "invalid_request", "请求内容无效", map[string]any{
				"errors": []configIssue{{Path: "request", Message: err.Error()}},
			})
		}
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeAPIError(w, http.StatusUnprocessableEntity, "invalid_request", "请求只能包含一个 JSON 对象", map[string]any{
			"errors": []configIssue{{Path: "request", Message: "must contain exactly one JSON object"}},
		})
		return false
	}
	return true
}

func readLocalFile(path string) (localFileItem, string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return localFileItem{}, "", err
	}
	if !info.Mode().IsRegular() {
		return localFileItem{}, "", os.ErrNotExist
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return localFileItem{}, "", err
	}
	return localFileMeta(filepath.Base(path), info, content), string(content), nil
}

func writeLocalFile(path string, content string) (localFileDetail, error) {
	payload := []byte(content)
	if err := util.AtomicWriteFile(path, payload); err != nil {
		return localFileDetail{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return localFileDetail{}, err
	}
	item := localFileMeta(filepath.Base(path), info, payload)
	return localFileDetail{localFileItem: item, Content: content}, nil
}

func localFileMeta(name string, info os.FileInfo, content []byte) localFileItem {
	return localFileItem{
		Name:       name,
		Size:       info.Size(),
		Lines:      countTextLines(content),
		ModifiedAt: util.FormatISO(info.ModTime()),
	}
}

func countTextLines(content []byte) int {
	if len(content) == 0 {
		return 0
	}
	n := bytes.Count(content, []byte{'\n'})
	if content[len(content)-1] != '\n' {
		n++
	}
	return n
}

func writeInvalidLocalFileName(w http.ResponseWriter) {
	writeAPIError(w, http.StatusUnprocessableEntity, "invalid_filename", "文件名无效，只能使用 .list / .yaml / .txt，且不能包含路径", map[string]any{
		"errors": []configIssue{{Path: "name", Message: "must be a .list, .yaml, or .txt filename without path separators"}},
	})
}
