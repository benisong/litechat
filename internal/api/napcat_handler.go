package api

import (
	"database/sql"
	"fmt"
	"litechat/internal/model"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const napCatProvider = "napcat"

func (h *Handlers) HandleNapCatCallback(c *gin.Context) {
	var event model.NapCatCallbackEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"provider":     napCatProvider,
			"should_reply": false,
			"reason":       "invalid_json",
			"error":        err.Error(),
		})
		return
	}

	decision := h.filterNapCatEvent(&event, c.GetHeader("X-Self-ID"))
	if decision.ShouldReply {
		result, err := h.channelDispatchService.DispatchNapCatMessage(decision.SelfID, decision.UserID, decision.RawMessage)
		if err != nil {
			decision.ShouldReply = false
			decision.Reason = "dispatch_failed"
			decision.Error = err.Error()
		} else {
			decision.Action = result.Action
			decision.ReplyText = result.ReplyText
			decision.OwnerUserID = result.OwnerUserID
			decision.OwnerName = result.OwnerName
			decision.Reason = result.Reason
			decision.ShouldReply = result.ShouldReply
		}
	}
	log.Printf("napcat callback decision should_reply=%t reason=%s self_id=%s user_id=%s text=%q",
		decision.ShouldReply, decision.Reason, decision.SelfID, decision.UserID, decision.RawMessage)
	c.JSON(http.StatusOK, decision)
}

func (h *Handlers) GetNapCatOwner(c *gin.Context) {
	owner, source, err := h.channelDispatchService.ResolveNapCatOwner()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.NapCatOwnerResponse{
		UserID:   owner.ID,
		Username: owner.Username,
		UserName: owner.UserName,
		Source:   source,
	})
}

func (h *Handlers) UpdateNapCatOwner(c *gin.Context) {
	var req model.UpdateNapCatOwnerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.userStore.GetByID(strings.TrimSpace(req.UserID))
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if user.Role == "admin" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "admin cannot be used as napcat owner"})
		return
	}

	if err := h.configStore.Set("napcat_owner_user_id", user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.NapCatOwnerResponse{
		UserID:   user.ID,
		Username: user.Username,
		UserName: user.UserName,
		Source:   "config",
	})
}

func (h *Handlers) ListNapCatWhitelist(c *gin.Context) {
	list, err := h.channelStore.ListWhitelist(napCatProvider)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if list == nil {
		list = []*model.ChannelWhitelistEntry{}
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handlers) GetNapCatConfig(c *gin.Context) {
	cfg, err := h.napCatAdminService.GetConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if cfg.AccessToken != "" {
		cfg.AccessToken = "***" + cfg.AccessToken[max(0, len(cfg.AccessToken)-4):]
	}
	c.JSON(http.StatusOK, cfg)
}

func (h *Handlers) UpdateNapCatConfig(c *gin.Context) {
	cfg := &model.NapCatConfig{}
	if err := c.ShouldBindJSON(cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	current, err := h.napCatAdminService.GetConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if isKeyMasked(cfg.AccessToken) {
		cfg.AccessToken = current.AccessToken
	}

	if err := h.napCatAdminService.UpdateConfig(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if cfg.AccessToken != "" {
		cfg.AccessToken = "***" + cfg.AccessToken[max(0, len(cfg.AccessToken)-4):]
	}
	c.JSON(http.StatusOK, cfg)
}

func (h *Handlers) ListNapCatFriends(c *gin.Context) {
	resp, err := h.napCatAdminService.GetFriends()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handlers) ReplaceNapCatWhitelist(c *gin.Context) {
	var req model.ReplaceNapCatWhitelistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.napCatAdminService.ReplaceWhitelist(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	list, err := h.channelStore.ListWhitelist(napCatProvider)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if list == nil {
		list = []*model.ChannelWhitelistEntry{}
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handlers) CreateNapCatWhitelist(c *gin.Context) {
	var req model.UpsertChannelWhitelistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	entry := &model.ChannelWhitelistEntry{
		Provider:       napCatProvider,
		SelfID:         strings.TrimSpace(req.SelfID),
		ExternalUserID: normalizeOneBotID(req.ExternalUserID),
		DisplayName:    strings.TrimSpace(req.DisplayName),
		Note:           strings.TrimSpace(req.Note),
		Enabled:        true,
	}
	if req.Enabled != nil {
		entry.Enabled = *req.Enabled
	}
	if entry.ExternalUserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "external_user_id is required"})
		return
	}

	if err := h.channelStore.CreateWhitelistEntry(entry); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, entry)
}

func (h *Handlers) UpdateNapCatWhitelist(c *gin.Context) {
	var req model.UpsertChannelWhitelistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	entry, err := h.channelStore.GetWhitelistByID(c.Param("id"))
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "whitelist entry not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	entry.Provider = napCatProvider
	entry.SelfID = strings.TrimSpace(req.SelfID)
	entry.ExternalUserID = normalizeOneBotID(req.ExternalUserID)
	entry.DisplayName = strings.TrimSpace(req.DisplayName)
	entry.Note = strings.TrimSpace(req.Note)
	if req.Enabled != nil {
		entry.Enabled = *req.Enabled
	}
	if entry.ExternalUserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "external_user_id is required"})
		return
	}

	if err := h.channelStore.UpdateWhitelistEntry(entry); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, entry)
}

func (h *Handlers) DeleteNapCatWhitelist(c *gin.Context) {
	if err := h.channelStore.DeleteWhitelistEntry(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *Handlers) filterNapCatEvent(event *model.NapCatCallbackEvent, headerSelfID string) model.NapCatFilterDecision {
	decision := model.NapCatFilterDecision{
		Provider:    napCatProvider,
		ShouldReply: false,
		Reason:      "ignored",
		PostType:    strings.TrimSpace(event.PostType),
		MessageType: strings.TrimSpace(event.MessageType),
		SubType:     strings.TrimSpace(event.SubType),
	}

	selfID := normalizeOneBotID(event.SelfID)
	if selfID == "" {
		selfID = strings.TrimSpace(headerSelfID)
	}
	userID := normalizeOneBotID(event.UserID)
	rawMessage := extractNapCatText(event)

	decision.SelfID = selfID
	decision.UserID = userID
	decision.RawMessage = rawMessage

	switch {
	case decision.PostType != "message":
		decision.Reason = "unsupported_post_type"
		return decision
	case decision.MessageType != "private":
		decision.Reason = "unsupported_message_type"
		return decision
	case decision.SubType != "" && decision.SubType != "friend":
		decision.Reason = "unsupported_private_sub_type"
		return decision
	case userID == "":
		decision.Reason = "missing_user_id"
		return decision
	case selfID != "" && selfID == userID:
		decision.Reason = "self_message"
		return decision
	case rawMessage == "":
		decision.Reason = "empty_message"
		return decision
	}

	_, err := h.channelStore.FindEnabledWhitelist(napCatProvider, selfID, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			decision.Reason = "user_not_whitelisted"
			return decision
		}
		decision.Reason = "whitelist_lookup_failed"
		return decision
	}

	decision.ShouldReply = true
	decision.Reason = "whitelisted_private_friend_message"
	return decision
}

func normalizeOneBotID(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case int:
		return strconv.Itoa(v)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func extractNapCatText(event *model.NapCatCallbackEvent) string {
	if text := strings.TrimSpace(event.RawMessage); text != "" {
		return text
	}

	switch message := event.Message.(type) {
	case string:
		return strings.TrimSpace(message)
	case []interface{}:
		var builder strings.Builder
		for _, item := range message {
			segment, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			segmentType, _ := segment["type"].(string)
			if segmentType != "text" {
				continue
			}
			data, _ := segment["data"].(map[string]interface{})
			if data == nil {
				continue
			}
			text, _ := data["text"].(string)
			builder.WriteString(text)
		}
		return strings.TrimSpace(builder.String())
	default:
		return ""
	}
}
