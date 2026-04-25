package model

import "time"

// ChannelWhitelistEntry stores allowed external contacts for a provider.
type ChannelWhitelistEntry struct {
	ID             string    `json:"id" db:"id"`
	Provider       string    `json:"provider" db:"provider"`
	SelfID         string    `json:"self_id" db:"self_id"`
	ExternalUserID string    `json:"external_user_id" db:"external_user_id"`
	DisplayName    string    `json:"display_name" db:"display_name"`
	Note           string    `json:"note" db:"note"`
	Enabled        bool      `json:"enabled" db:"enabled"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// UpsertChannelWhitelistRequest is used by admin APIs to manage whitelist rows.
type UpsertChannelWhitelistRequest struct {
	SelfID         string `json:"self_id"`
	ExternalUserID string `json:"external_user_id" binding:"required"`
	DisplayName    string `json:"display_name"`
	Note           string `json:"note"`
	Enabled        *bool  `json:"enabled"`
}

// NapCatCallbackEvent is the subset of OneBot/NapCat fields needed for routing.
type NapCatCallbackEvent struct {
	Time        int64       `json:"time"`
	PostType    string      `json:"post_type"`
	MessageType string      `json:"message_type"`
	SubType     string      `json:"sub_type"`
	SelfID      interface{} `json:"self_id"`
	UserID      interface{} `json:"user_id"`
	MessageID   interface{} `json:"message_id"`
	Message     interface{} `json:"message"`
	RawMessage  string      `json:"raw_message"`
}

// NapCatFilterDecision describes whether a callback event should be processed.
type NapCatFilterDecision struct {
	Provider    string `json:"provider"`
	ShouldReply bool   `json:"should_reply"`
	Reason      string `json:"reason"`
	SelfID      string `json:"self_id,omitempty"`
	UserID      string `json:"user_id,omitempty"`
	PostType    string `json:"post_type,omitempty"`
	MessageType string `json:"message_type,omitempty"`
	SubType     string `json:"sub_type,omitempty"`
	RawMessage  string `json:"raw_message,omitempty"`
	Action      string `json:"action,omitempty"`
	ReplyText   string `json:"reply_text,omitempty"`
	OwnerUserID string `json:"owner_user_id,omitempty"`
	OwnerName   string `json:"owner_name,omitempty"`
	Error       string `json:"error,omitempty"`
}

// ChannelSession tracks the current state for one external contact.
type ChannelSession struct {
	ID                string    `json:"id" db:"id"`
	Provider          string    `json:"provider" db:"provider"`
	SelfID            string    `json:"self_id" db:"self_id"`
	ExternalUserID    string    `json:"external_user_id" db:"external_user_id"`
	OwnerUserID       string    `json:"owner_user_id" db:"owner_user_id"`
	ActiveCharacterID string    `json:"active_character_id" db:"active_character_id"`
	ActiveChatID      string    `json:"active_chat_id" db:"active_chat_id"`
	State             string    `json:"state" db:"state"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time `json:"updated_at" db:"updated_at"`
}

// ChannelChatLinkView is a chat entry visible to a single external contact.
type ChannelChatLinkView struct {
	ChatID        string    `json:"chat_id" db:"chat_id"`
	Title         string    `json:"title" db:"title"`
	CharacterID   string    `json:"character_id" db:"character_id"`
	CharacterName string    `json:"character_name" db:"character_name"`
	LastMessage   string    `json:"last_message"`
	MessageCount  int       `json:"message_count"`
	ChatUpdatedAt time.Time `json:"chat_updated_at" db:"chat_updated_at"`
	LinkedAt      time.Time `json:"linked_at" db:"linked_at"`
}

// ChannelDispatchResult is the output from the fixed command dispatcher.
type ChannelDispatchResult struct {
	ShouldReply bool   `json:"should_reply"`
	Reason      string `json:"reason"`
	Action      string `json:"action"`
	ReplyText   string `json:"reply_text"`
	OwnerUserID string `json:"owner_user_id,omitempty"`
	OwnerName   string `json:"owner_name,omitempty"`
}

// UpdateNapCatOwnerRequest binds NapCat to a local user.
type UpdateNapCatOwnerRequest struct {
	UserID string `json:"user_id" binding:"required"`
}

// NapCatOwnerResponse describes the currently resolved NapCat owner.
type NapCatOwnerResponse struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	UserName string `json:"user_name"`
	Source   string `json:"source"`
}

// NapCatConfig stores the single-account NapCat connection settings.
type NapCatConfig struct {
	APIBaseURL  string `json:"api_base_url"`
	AccessToken string `json:"access_token"`
	Enabled     bool   `json:"enabled"`
}

// NapCatFriend is the normalized friend record returned by the admin API.
type NapCatFriend struct {
	UserID      string `json:"user_id"`
	Nickname    string `json:"nickname"`
	Remark      string `json:"remark"`
	DisplayName string `json:"display_name"`
	Whitelisted bool   `json:"whitelisted"`
}

// NapCatFriendListResponse is used by the admin panel to render selectable contacts.
type NapCatFriendListResponse struct {
	LoginUserID   string         `json:"login_user_id"`
	LoginNickname string         `json:"login_nickname"`
	Friends       []NapCatFriend `json:"friends"`
}

// ReplaceNapCatWhitelistRequest replaces the whitelist with the selected friends.
type ReplaceNapCatWhitelistRequest struct {
	Entries []UpsertChannelWhitelistRequest `json:"entries"`
}
