package handler

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"msp/internal/config"
	"msp/internal/constants"
	"msp/internal/domain"
	"msp/internal/util"
)

func (h *Handler) HandleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		view := h.configService.GetConfigView()
		writeJSON(w, http.StatusOK, view)
	case http.MethodPost:
		var cfg config.Config
		if err := decodeJSONBody(w, r, &cfg, defaultJSONBodyLimit); err != nil {
			writeJSONDecodeError(w, err)
			return
		}

		newCfg, err := h.configService.UpdateConfig(cfg)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, domain.ConfigResponse{Error: &domain.ApiError{Message: constants.ErrMsgWriteConfig}})
			return
		}
		writeJSON(w, http.StatusOK, domain.ConfigResponse{Config: newCfg})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *Handler) HandleShares(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req domain.SharesOpRequest
	if err := decodeJSONBody(w, r, &req, defaultJSONBodyLimit); err != nil {
		writeJSONDecodeError(w, err)
		return
	}

	op, p, label := normalizeSharesOp(req)
	newCfg, err := h.applySharesOp(op, p, label)

	if err != nil {
		if strings.Contains(err.Error(), "exists") || strings.Contains(err.Error(), "missing") {
			writeJSON(w, http.StatusBadRequest, domain.SharesOpResponse{Error: &domain.ApiError{Message: err.Error()}})
		} else {
			writeJSON(w, http.StatusInternalServerError, domain.SharesOpResponse{Error: &domain.ApiError{Message: constants.ErrMsgWriteConfig}})
		}
		return
	}

	h.media.InvalidateMediaCache()
	writeJSON(w, http.StatusOK, domain.SharesOpResponse{Config: newCfg})
}

func normalizeSharesOp(req domain.SharesOpRequest) (op string, path string, label string) {
	op = strings.ToLower(strings.TrimSpace(req.Op))
	path = util.NormalizePath(req.Path)
	label = strings.TrimSpace(req.Label)
	if label == "" && path != "" {
		label = filepath.Base(path)
	}
	return op, path, label
}

func (h *Handler) applySharesOp(op string, path string, label string) (config.Config, error) {
	switch op {
	case "add":
		return h.handleShareAdd(path, label)
	case "remove":
		return h.handleShareRemove(path)
	default:
		return config.Config{}, fmt.Errorf("不支持的 op（add/remove）")
	}
}

func (h *Handler) handleShareAdd(p, label string) (config.Config, error) {
	if p == "" || !util.IsExistingDir(p) {
		return config.Config{}, fmt.Errorf("目录不存在或不可访问")
	}

	var newCfg config.Config
	err := h.config.UpdateConfig(func(cfg *config.Config) {
		cfg.Shares = append(cfg.Shares, domain.Share{Label: label, Path: p})
		cfg.Shares = util.NormalizeShares(cfg.Shares)
		cfg.Shares = util.DedupeShares(cfg.Shares)
		newCfg = *cfg
	})
	return newCfg, err
}

func (h *Handler) handleShareRemove(p string) (config.Config, error) {
	if p == "" {
		return config.Config{}, fmt.Errorf("缺少 Path")
	}

	var newCfg config.Config
	err := h.config.UpdateConfig(func(cfg *config.Config) {
		out := make([]domain.Share, 0, len(cfg.Shares))
		for _, sh := range cfg.Shares {
			if !util.SamePath(sh.Path, p) {
				out = append(out, sh)
			}
		}
		cfg.Shares = out
		newCfg = *cfg
	})
	return newCfg, err
}

func (h *Handler) HandleIP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"lanIPs": util.GetLanIPv4s(),
	})
}