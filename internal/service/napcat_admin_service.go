package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"litechat/internal/model"
	"litechat/internal/store"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type NapCatAdminService struct {
	channelStore *store.ChannelStore
	configStore  *store.ConfigStore
	httpClient   *http.Client
}

func NewNapCatAdminService(channelStore *store.ChannelStore, configStore *store.ConfigStore) *NapCatAdminService {
	return &NapCatAdminService{
		channelStore: channelStore,
		configStore:  configStore,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (s *NapCatAdminService) GetConfig() (*model.NapCatConfig, error) {
	baseURL, _ := s.configStore.Get("napcat_api_base_url")
	token, _ := s.configStore.Get("napcat_access_token")
	enabledRaw, _ := s.configStore.Get("napcat_enabled")
	return &model.NapCatConfig{
		APIBaseURL:  strings.TrimSpace(baseURL),
		AccessToken: strings.TrimSpace(token),
		Enabled:     strings.EqualFold(strings.TrimSpace(enabledRaw), "true"),
	}, nil
}

func (s *NapCatAdminService) UpdateConfig(cfg *model.NapCatConfig) error {
	if err := s.configStore.Set("napcat_api_base_url", strings.TrimSpace(cfg.APIBaseURL)); err != nil {
		return err
	}
	if err := s.configStore.Set("napcat_access_token", strings.TrimSpace(cfg.AccessToken)); err != nil {
		return err
	}
	if err := s.configStore.Set("napcat_enabled", fmt.Sprintf("%t", cfg.Enabled)); err != nil {
		return err
	}
	return nil
}

func (s *NapCatAdminService) GetFriends() (*model.NapCatFriendListResponse, error) {
	cfg, err := s.GetConfig()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.APIBaseURL) == "" {
		return nil, fmt.Errorf("napcat api base url is empty")
	}

	loginResp, err := s.call(cfg, "get_login_info", map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	friendResp, err := s.call(cfg, "get_friend_list", map[string]interface{}{})
	if err != nil {
		return nil, err
	}

	whitelist, err := s.channelStore.ListWhitelist(napCatProvider)
	if err != nil {
		return nil, err
	}
	whitelistMap := make(map[string]bool, len(whitelist))
	for _, entry := range whitelist {
		whitelistMap[entry.ExternalUserID] = entry.Enabled
	}

	loginData := struct {
		UserID   interface{} `json:"user_id"`
		Nickname string      `json:"nickname"`
	}{}
	if err := decodeActionData(loginResp.Data, &loginData); err != nil {
		return nil, err
	}

	friendData := []struct {
		UserID   interface{} `json:"user_id"`
		Nickname string      `json:"nickname"`
		Remark   string      `json:"remark"`
	}{}
	if err := decodeActionData(friendResp.Data, &friendData); err != nil {
		return nil, err
	}

	resp := &model.NapCatFriendListResponse{
		LoginUserID:   normalizeNapCatID(loginData.UserID),
		LoginNickname: strings.TrimSpace(loginData.Nickname),
		Friends:       make([]model.NapCatFriend, 0, len(friendData)),
	}
	for _, friend := range friendData {
		userID := normalizeNapCatID(friend.UserID)
		displayName := strings.TrimSpace(friend.Remark)
		if displayName == "" {
			displayName = strings.TrimSpace(friend.Nickname)
		}
		resp.Friends = append(resp.Friends, model.NapCatFriend{
			UserID:      userID,
			Nickname:    strings.TrimSpace(friend.Nickname),
			Remark:      strings.TrimSpace(friend.Remark),
			DisplayName: displayName,
			Whitelisted: whitelistMap[userID],
		})
	}
	return resp, nil
}

func (s *NapCatAdminService) ReplaceWhitelist(req *model.ReplaceNapCatWhitelistRequest) error {
	entries := make([]*model.ChannelWhitelistEntry, 0, len(req.Entries))
	for _, item := range req.Entries {
		externalUserID := normalizeNapCatID(item.ExternalUserID)
		if externalUserID == "" {
			continue
		}
		enabled := true
		if item.Enabled != nil {
			enabled = *item.Enabled
		}
		entries = append(entries, &model.ChannelWhitelistEntry{
			Provider:       napCatProvider,
			SelfID:         "",
			ExternalUserID: externalUserID,
			DisplayName:    strings.TrimSpace(item.DisplayName),
			Note:           strings.TrimSpace(item.Note),
			Enabled:        enabled,
		})
	}
	return s.channelStore.ReplaceWhitelist(napCatProvider, entries)
}

type napCatActionResponse struct {
	Status  string          `json:"status"`
	RetCode int             `json:"retcode"`
	Data    json.RawMessage `json:"data"`
	Msg     string          `json:"msg"`
	Wording string          `json:"wording"`
}

func (s *NapCatAdminService) call(cfg *model.NapCatConfig, action string, payload map[string]interface{}) (*napCatActionResponse, error) {
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(cfg.APIBaseURL, "/") + "/" + strings.TrimLeft(action, "/")
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(cfg.AccessToken) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(cfg.AccessToken))
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	result := &napCatActionResponse{}
	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("napcat http %d: %s", resp.StatusCode, fallbackText(result.Wording, result.Msg))
	}
	if result.Status != "" && !strings.EqualFold(result.Status, "ok") {
		return nil, fmt.Errorf("napcat action %s failed: %s", action, fallbackText(result.Wording, result.Msg))
	}
	if result.RetCode != 0 {
		return nil, fmt.Errorf("napcat action %s failed with retcode %d: %s", action, result.RetCode, fallbackText(result.Wording, result.Msg))
	}
	return result, nil
}

func decodeActionData(raw json.RawMessage, dest interface{}) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, dest)
}

func fallbackText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return "unknown error"
}

func normalizeNapCatID(value interface{}) string {
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
