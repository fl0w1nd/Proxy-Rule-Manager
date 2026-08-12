package serve

import (
	"encoding/base64"
	"net/http"
	"strconv"
)

type apiErrorBody struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
}

func writeAPIError(w http.ResponseWriter, status int, code, message string, details map[string]any) {
	if details == nil {
		details = map[string]any{}
	}
	writeJSON(w, status, apiErrorBody{Error: apiError{Code: code, Message: message, Details: details}})
}

func pageLimit(w http.ResponseWriter, r *http.Request) (int, bool) {
	value := r.URL.Query().Get("limit")
	if value == "" {
		return 20, true
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit < 1 || limit > 100 {
		writeAPIError(w, http.StatusBadRequest, "invalid_limit", "limit 必须在 1 到 100 之间", map[string]any{})
		return 0, false
	}
	return limit, true
}

func encodeCursor(value string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}

func decodeCursor(value string) (string, error) {
	data, err := base64.RawURLEncoding.DecodeString(value)
	return string(data), err
}
