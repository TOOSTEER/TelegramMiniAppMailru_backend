package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/storage"
	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/tgauth"
)

type telegramProfileResponse struct {
	TelegramID      int64  `json:"telegram_id"`
	FirstName       string `json:"first_name"`
	LastName        string `json:"last_name,omitempty"`
	Username        string `json:"username,omitempty"`
	LanguageCode    string `json:"language_code,omitempty"`
	PhotoURL        string `json:"photo_url,omitempty"`
	IsPremium       bool   `json:"is_premium"`
	AllowsWriteToPM bool   `json:"allows_write_to_pm"`
	HasPhone        bool   `json:"has_phone"`
	PhoneNumber     string `json:"phone_number,omitempty"`
}

func profileToResponse(p storage.TelegramProfile, includePhone bool) telegramProfileResponse {
	resp := telegramProfileResponse{
		TelegramID:      p.TelegramID,
		FirstName:       p.FirstName,
		LastName:        p.LastName,
		Username:        p.Username,
		LanguageCode:    p.LanguageCode,
		PhotoURL:        p.PhotoURL,
		IsPremium:       p.IsPremium,
		AllowsWriteToPM: p.AllowsWriteToPM,
		HasPhone:        p.PhoneNumber != "",
	}
	if includePhone {
		resp.PhoneNumber = p.PhoneNumber
	}
	return resp
}

func (h *handler) telegramProfileSync(c *gin.Context) {
	u := initData(c).User

	err := h.telegramProfiles.Upsert(c.Request.Context(), storage.TelegramProfile{
		TelegramID:      u.ID,
		FirstName:       u.FirstName,
		LastName:        u.LastName,
		Username:        u.Username,
		LanguageCode:    u.LanguageCode,
		PhotoURL:        u.PhotoURL,
		IsPremium:       u.IsPremium,
		AllowsWriteToPM: u.AllowsWriteToPM,
	})
	if err != nil {
		_ = c.Error(err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}

	h.respondTelegramProfile(c, u.ID)
}

func (h *handler) telegramProfileGet(c *gin.Context) {
	h.respondTelegramProfile(c, initData(c).User.ID)
}

func (h *handler) respondTelegramProfile(c *gin.Context, telegramID int64) {
	p, err := h.telegramProfiles.Find(c.Request.Context(), telegramID)
	if errors.Is(err, storage.ErrTelegramProfileNotFound) {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "telegram_profile_not_found"})
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	c.JSON(http.StatusOK, profileToResponse(p, true))
}

const maxContactBodySize = 4 * 1024

type contactRequest struct {
	Contact string `json:"contact"`
}

func (h *handler) telegramContact(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxContactBodySize)

	var req contactRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Contact == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "contact_invalid"})
		return
	}

	contact, err := tgauth.ValidateContact(req.Contact, h.botToken, h.initDataMaxAge, time.Now())
	if errors.Is(err, tgauth.ErrExpired) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "contact_expired"})
		return
	}
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "contact_invalid"})
		return
	}

	u := initData(c).User
	if contact.UserID != u.ID {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "contact_mismatch"})
		return
	}

	phone := normalizePhone(contact.PhoneNumber)
	if phone == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "contact_invalid"})
		return
	}

	ctx := c.Request.Context()

	if err := h.telegramProfiles.Upsert(ctx, storage.TelegramProfile{
		TelegramID:      u.ID,
		FirstName:       u.FirstName,
		LastName:        u.LastName,
		Username:        u.Username,
		LanguageCode:    u.LanguageCode,
		PhotoURL:        u.PhotoURL,
		IsPremium:       u.IsPremium,
		AllowsWriteToPM: u.AllowsWriteToPM,
	}); err != nil {
		_ = c.Error(err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}

	if err := h.telegramProfiles.SavePhone(ctx, u.ID, phone); err != nil {
		_ = c.Error(err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}

	h.log.Info("телефон получен от Telegram",
		zap.Int64("telegram_id", u.ID),
		zap.Bool("has_phone", true),
	)

	h.respondTelegramProfile(c, u.ID)
}

func normalizePhone(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	if strings.HasPrefix(s, "+") {
		return "+" + b.String()
	}
	return b.String()
}
