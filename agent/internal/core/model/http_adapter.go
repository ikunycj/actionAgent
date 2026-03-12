package model

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const anthropicVersionDefault = "2023-06-01"

type HTTPAdapterConfig struct {
	Provider   string
	APIStyle   string
	BaseURL    string
	Model      string
	MaxTokens  int
	HTTPClient *http.Client
}

type HTTPAdapter struct {
	cfg HTTPAdapterConfig
}

func NewHTTPAdapter(cfg HTTPAdapterConfig) *HTTPAdapter {
	cfg.APIStyle = strings.ToLower(strings.TrimSpace(cfg.APIStyle))
	cfg.BaseURL = strings.TrimSpace(cfg.BaseURL)
	cfg.Model = strings.TrimSpace(cfg.Model)
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 1024
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{}
	}
	return &HTTPAdapter{cfg: cfg}
}

func (a *HTTPAdapter) Name() string { return a.cfg.Provider }

func (a *HTTPAdapter) Complete(ctx context.Context, req Request, cred Credential) (Response, error) {
	switch a.cfg.APIStyle {
	case "openai":
		return a.completeOpenAI(ctx, req, cred)
	case "anthropic":
		return a.completeAnthropic(ctx, req, cred)
	default:
		return Response{}, &ProviderError{Class: ErrFormat, Msg: "unsupported api style"}
	}
}

func (a *HTTPAdapter) completeOpenAI(ctx context.Context, req Request, cred Credential) (Response, error) {
	if shouldUseOpenAIResponses(req.Input) {
		return a.completeOpenAIResponses(ctx, req, cred)
	}
	return a.completeOpenAIChat(ctx, req, cred)
}

func shouldUseOpenAIResponses(input map[string]any) bool {
	if input == nil {
		return false
	}
	if _, ok := input["messages"]; ok {
		return false
	}
	_, hasInput := input["input"]
	return hasInput
}

func (a *HTTPAdapter) completeOpenAIChat(ctx context.Context, req Request, cred Credential) (Response, error) {
	modelName := chooseModel(req.Model, a.cfg.Model)
	body := map[string]any{
		"model":    modelName,
		"messages": normalizeOpenAIMessages(req.Input),
	}
	if v, ok := req.Input["temperature"]; ok {
		body["temperature"] = v
	}
	if v, ok := req.Input["max_tokens"]; ok {
		body["max_tokens"] = v
	}
	endpoint := joinURL(a.cfg.BaseURL, "/chat/completions")
	raw, err := doJSON(ctx, a.cfg.HTTPClient, endpoint, map[string]string{
		"Authorization": "Bearer " + cred.Secret,
	}, body)
	if err != nil {
		return Response{}, err
	}
	var out struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content any `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return Response{}, &ProviderError{Class: ErrFormat, Msg: err.Error()}
	}
	text := ""
	if len(out.Choices) > 0 {
		text = contentToText(out.Choices[0].Message.Content)
	}
	payload := map[string]any{
		"text": text,
	}
	var rawObj map[string]any
	if err := json.Unmarshal(raw, &rawObj); err == nil {
		payload["raw"] = rawObj
	}
	return Response{Provider: a.cfg.Provider, Model: chooseModel(out.Model, modelName), Output: payload}, nil
}

func (a *HTTPAdapter) completeOpenAIResponses(ctx context.Context, req Request, cred Credential) (Response, error) {
	modelName := chooseModel(req.Model, a.cfg.Model)
	body := map[string]any{
		"model": modelName,
		"input": normalizeOpenAIInput(req.Input),
	}
	for _, key := range []string{"temperature", "max_tokens", "max_output_tokens"} {
		if v, ok := req.Input[key]; ok {
			body[key] = v
		}
	}
	endpoint := joinURL(a.cfg.BaseURL, "/responses")
	raw, err := doJSON(ctx, a.cfg.HTTPClient, endpoint, map[string]string{
		"Authorization": "Bearer " + cred.Secret,
	}, body)
	if err != nil {
		return Response{}, err
	}
	var out struct {
		Model      string `json:"model"`
		OutputText string `json:"output_text"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return Response{}, &ProviderError{Class: ErrFormat, Msg: err.Error()}
	}
	payload := map[string]any{
		"text": strings.TrimSpace(out.OutputText),
	}
	var rawObj map[string]any
	if err := json.Unmarshal(raw, &rawObj); err == nil {
		payload["raw"] = rawObj
		if payload["text"] == "" {
			payload["text"] = extractOpenAIResponsesText(rawObj)
		}
	}
	return Response{Provider: a.cfg.Provider, Model: chooseModel(out.Model, modelName), Output: payload}, nil
}

func (a *HTTPAdapter) completeAnthropic(ctx context.Context, req Request, cred Credential) (Response, error) {
	modelName := chooseModel(req.Model, a.cfg.Model)
	body := map[string]any{
		"model":      modelName,
		"max_tokens": a.cfg.MaxTokens,
		"messages":   normalizeAnthropicMessages(req.Input),
	}
	if v, ok := req.Input["max_tokens"]; ok {
		body["max_tokens"] = v
	}
	endpoint := joinURL(a.cfg.BaseURL, "/messages")
	headers := map[string]string{
		"x-api-key":         cred.Secret,
		"anthropic-version": anthropicVersionDefault,
	}
	if ver, ok := req.Input["anthropic_version"].(string); ok && strings.TrimSpace(ver) != "" {
		headers["anthropic-version"] = strings.TrimSpace(ver)
	}
	raw, err := doJSON(ctx, a.cfg.HTTPClient, endpoint, headers, body)
	if err != nil {
		return Response{}, err
	}
	var out struct {
		Model   string `json:"model"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return Response{}, &ProviderError{Class: ErrFormat, Msg: err.Error()}
	}
	parts := make([]string, 0, len(out.Content))
	for _, c := range out.Content {
		if c.Type == "text" && strings.TrimSpace(c.Text) != "" {
			parts = append(parts, c.Text)
		}
	}
	payload := map[string]any{
		"text": strings.Join(parts, "\n"),
	}
	var rawObj map[string]any
	if err := json.Unmarshal(raw, &rawObj); err == nil {
		payload["raw"] = rawObj
	}
	return Response{Provider: a.cfg.Provider, Model: chooseModel(out.Model, modelName), Output: payload}, nil
}

func doJSON(ctx context.Context, client *http.Client, endpoint string, headers map[string]string, body map[string]any) ([]byte, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, &ProviderError{Class: ErrFormat, Msg: err.Error()}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(b))
	if err != nil {
		return nil, &ProviderError{Class: ErrUnknown, Msg: err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "*/*")
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "ActionAgent/1.0")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, &ProviderError{Class: classifyHTTPTransport(err), Msg: err.Error()}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, classifyHTTPFailure(resp.StatusCode, raw)
	}
	return raw, nil
}

func classifyHTTPTransport(err error) ErrorClass {
	if err == nil {
		return ErrUnknown
	}
	if strings.Contains(strings.ToLower(err.Error()), "timeout") || strings.Contains(strings.ToLower(err.Error()), "deadline exceeded") {
		return ErrTimeout
	}
	return ErrUnknown
}

func classifyHTTPFailure(status int, raw []byte) error {
	msg := strings.TrimSpace(string(raw))
	code := ""
	var oai struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &oai); err == nil {
		if oai.Error.Message != "" {
			msg = oai.Error.Message
		}
		code = oai.Error.Code
		if code == "" {
			code = oai.Error.Type
		}
	}
	if msg == "" {
		msg = fmt.Sprintf("provider http status %d", status)
	}
	switch status {
	case 401, 403:
		return &ProviderError{Class: ErrAuthPermanent, Msg: msg}
	case 402:
		return &ProviderError{Class: ErrBilling, Msg: msg}
	case 404:
		return &ProviderError{Class: ErrModelNotFound, Msg: msg}
	case 408, 504:
		return &ProviderError{Class: ErrTimeout, Msg: msg}
	case 429:
		return &ProviderError{Class: ErrRateLimit, Msg: msg}
	case 400, 422:
		if strings.Contains(strings.ToLower(code), "model_not_found") {
			return &ProviderError{Class: ErrModelNotFound, Msg: msg}
		}
		return &ProviderError{Class: ErrFormat, Msg: msg}
	default:
		if status >= 500 {
			return &ProviderError{Class: ErrUnknown, Msg: msg}
		}
		return &ProviderError{Class: ErrUnknown, Msg: msg}
	}
}

func chooseModel(inReq, inCfg string) string {
	if strings.TrimSpace(inReq) != "" {
		return strings.TrimSpace(inReq)
	}
	return strings.TrimSpace(inCfg)
}

func joinURL(base, path string) string {
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

func normalizeOpenAIMessages(input map[string]any) []map[string]any {
	msgs := normalizeMessages(input)
	if len(msgs) > 0 {
		return msgs
	}
	return []map[string]any{{"role": "user", "content": "hello"}}
}

func normalizeOpenAIInput(input map[string]any) any {
	if input == nil {
		return []map[string]any{{"role": "user", "content": "hello"}}
	}
	if raw, ok := input["input"]; ok && raw != nil {
		return raw
	}
	msgs := normalizeMessages(input)
	if len(msgs) > 0 {
		return msgs
	}
	return []map[string]any{{"role": "user", "content": "hello"}}
}

func normalizeAnthropicMessages(input map[string]any) []map[string]any {
	msgs := normalizeMessages(input)
	if len(msgs) == 0 {
		return []map[string]any{{"role": "user", "content": "hello"}}
	}
	out := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		role, _ := m["role"].(string)
		if role != "assistant" {
			role = "user"
		}
		out = append(out, map[string]any{
			"role":    role,
			"content": contentToText(m["content"]),
		})
	}
	return out
}

func normalizeMessages(input map[string]any) []map[string]any {
	if input == nil {
		return nil
	}
	if raw, ok := input["messages"]; ok {
		if arr, ok := raw.([]any); ok {
			out := make([]map[string]any, 0, len(arr))
			for _, item := range arr {
				switch v := item.(type) {
				case map[string]any:
					role, _ := v["role"].(string)
					if strings.TrimSpace(role) == "" {
						role = "user"
					}
					out = append(out, map[string]any{
						"role":    role,
						"content": v["content"],
					})
				case string:
					out = append(out, map[string]any{"role": "user", "content": v})
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	if s, ok := input["text"].(string); ok && strings.TrimSpace(s) != "" {
		return []map[string]any{{"role": "user", "content": s}}
	}
	if raw, ok := input["input"]; ok {
		return []map[string]any{{"role": "user", "content": contentToText(raw)}}
	}
	b, _ := json.Marshal(input)
	if len(b) == 0 {
		return nil
	}
	return []map[string]any{{"role": "user", "content": string(b)}}
}

func contentToText(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case []any:
		parts := make([]string, 0, len(x))
		for _, item := range x {
			if m, ok := item.(map[string]any); ok {
				if t, ok := m["text"].(string); ok && strings.TrimSpace(t) != "" {
					parts = append(parts, t)
				}
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n")
		}
		b, _ := json.Marshal(x)
		return string(b)
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func extractOpenAIResponsesText(raw map[string]any) string {
	if raw == nil {
		return ""
	}
	if v, ok := raw["output_text"].(string); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	items, ok := raw["output"].([]any)
	if !ok {
		return ""
	}
	parts := make([]string, 0)
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		content, ok := obj["content"].([]any)
		if !ok {
			continue
		}
		for _, c := range content {
			cm, ok := c.(map[string]any)
			if !ok {
				continue
			}
			if t, ok := cm["text"].(string); ok && strings.TrimSpace(t) != "" {
				parts = append(parts, strings.TrimSpace(t))
			}
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n")
}
