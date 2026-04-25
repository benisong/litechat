package service

import (
	"database/sql"
	"fmt"
	"litechat/internal/model"
	"litechat/internal/store"
	"strconv"
	"strings"
	"time"
)

const napCatProvider = "napcat"

type channelCommand struct {
	Action string
	Arg    string
}

type ChannelDispatchService struct {
	characterStore *store.CharacterStore
	chatStore      *store.ChatStore
	channelStore   *store.ChannelStore
	configStore    *store.ConfigStore
	userStore      *store.UserStore
}

func NewChannelDispatchService(
	characterStore *store.CharacterStore,
	chatStore *store.ChatStore,
	channelStore *store.ChannelStore,
	configStore *store.ConfigStore,
	userStore *store.UserStore,
) *ChannelDispatchService {
	return &ChannelDispatchService{
		characterStore: characterStore,
		chatStore:      chatStore,
		channelStore:   channelStore,
		configStore:    configStore,
		userStore:      userStore,
	}
}

func (s *ChannelDispatchService) ResolveNapCatOwner() (*model.User, string, error) {
	explicitID, err := s.configStore.Get("napcat_owner_user_id")
	if err == nil && strings.TrimSpace(explicitID) != "" {
		user, err := s.userStore.GetByID(strings.TrimSpace(explicitID))
		if err == nil && user.Role != "admin" {
			return user, "config", nil
		}
	}

	mode := s.userStore.GetCurrentMode()
	qqsvc, err := s.userStore.GetByUsernameAndMode("qqsvc", mode)
	if err == nil && qqsvc.Role != "admin" {
		return qqsvc, "qqsvc_default", nil
	}

	user, err := s.userStore.GetFirstNonAdminByMode(mode)
	if err != nil {
		return nil, "", err
	}
	return user, "first_non_admin", nil
}

func (s *ChannelDispatchService) DispatchNapCatMessage(selfID, externalUserID, rawText string) (*model.ChannelDispatchResult, error) {
	owner, _, err := s.ResolveNapCatOwner()
	if err != nil {
		return nil, err
	}

	session, err := s.channelStore.GetOrCreateSession(napCatProvider, selfID, externalUserID, owner.ID)
	if err != nil {
		return nil, err
	}

	cmd := parseChannelCommand(rawText)
	result, err := s.dispatchCommand(owner, session, cmd)
	if err != nil {
		return nil, err
	}
	result.OwnerUserID = owner.ID
	result.OwnerName = owner.Username
	return result, nil
}

func (s *ChannelDispatchService) dispatchCommand(owner *model.User, session *model.ChannelSession, cmd channelCommand) (*model.ChannelDispatchResult, error) {
	switch cmd.Action {
	case "help":
		return &model.ChannelDispatchResult{
			ShouldReply: true,
			Reason:      "command_handled",
			Action:      "help",
			ReplyText:   helpText(),
		}, nil
	case "list_characters":
		return s.listCharacters(owner, session)
	case "show_character_card":
		return s.showCharacterCard(owner, session, cmd.Arg)
	case "select_character":
		return s.selectCharacter(owner, session, cmd.Arg)
	case "current_character":
		return s.currentCharacter(owner, session)
	case "new_chat":
		return s.newChat(owner, session, cmd.Arg)
	case "list_sessions":
		return s.listSessions(owner, session)
	case "switch_session":
		return s.switchSession(owner, session, cmd.Arg)
	case "status":
		return s.status(owner, session)
	case "reset_session":
		return s.resetSession(owner, session)
	default:
		return &model.ChannelDispatchResult{
			ShouldReply: true,
			Reason:      "chat_not_connected_yet",
			Action:      "chat_message",
			ReplyText:   "普通聊天分支还没接入 AI。当前可用命令：帮助、角色列表、查看角色卡、选择角色、新聊天、会话列表、切换会话、当前状态、重置会话。",
		}, nil
	}
}

func parseChannelCommand(rawText string) channelCommand {
	text := strings.TrimSpace(rawText)
	if text == "" {
		return channelCommand{Action: "chat_message"}
	}

	fields := strings.Fields(text)
	head := fields[0]
	arg := ""
	if len(fields) > 1 {
		arg = strings.TrimSpace(strings.Join(fields[1:], " "))
	}

	switch head {
	case "帮助", "/help", "help", "菜单":
		return channelCommand{Action: "help"}
	case "角色列表", "/chars", "角色卡列表", "查看角色卡列表":
		return channelCommand{Action: "list_characters"}
	case "查看角色卡", "/card", "角色卡":
		return channelCommand{Action: "show_character_card", Arg: arg}
	case "选择角色", "/use":
		return channelCommand{Action: "select_character", Arg: arg}
	case "当前角色", "/current":
		return channelCommand{Action: "current_character"}
	case "新聊天", "/new":
		return channelCommand{Action: "new_chat", Arg: arg}
	case "会话列表", "/sessions":
		return channelCommand{Action: "list_sessions"}
	case "切换会话", "/session":
		return channelCommand{Action: "switch_session", Arg: arg}
	case "当前状态", "/status":
		return channelCommand{Action: "status"}
	case "重置会话", "/reset":
		return channelCommand{Action: "reset_session"}
	default:
		return channelCommand{Action: "chat_message", Arg: text}
	}
}

func helpText() string {
	return strings.Join([]string{
		"可用命令：",
		"1. 角色列表",
		"2. 查看角色卡 角色名/序号",
		"3. 选择角色 角色名/序号",
		"4. 当前角色",
		"5. 新聊天 [角色名/序号]",
		"6. 会话列表",
		"7. 切换会话 序号",
		"8. 当前状态",
		"9. 重置会话",
	}, "\n")
}

func (s *ChannelDispatchService) listCharacters(owner *model.User, session *model.ChannelSession) (*model.ChannelDispatchResult, error) {
	chars, err := s.characterStore.List(owner.ID)
	if err != nil {
		return nil, err
	}
	if len(chars) == 0 {
		return &model.ChannelDispatchResult{
			ShouldReply: true,
			Reason:      "command_handled",
			Action:      "list_characters",
			ReplyText:   "当前没有可用角色卡。",
		}, nil
	}

	lines := []string{"角色列表："}
	for idx, char := range chars {
		current := ""
		if char.ID == session.ActiveCharacterID {
			current = " [当前]"
		}
		tag := strings.TrimSpace(char.Tags)
		if tag != "" {
			lines = append(lines, fmt.Sprintf("%d. %s%s (%s)", idx+1, char.Name, current, tag))
			continue
		}
		lines = append(lines, fmt.Sprintf("%d. %s%s", idx+1, char.Name, current))
	}
	lines = append(lines, "可发送：查看角色卡 2 / 选择角色 2 / 新聊天 2")
	return &model.ChannelDispatchResult{
		ShouldReply: true,
		Reason:      "command_handled",
		Action:      "list_characters",
		ReplyText:   strings.Join(lines, "\n"),
	}, nil
}

func (s *ChannelDispatchService) showCharacterCard(owner *model.User, session *model.ChannelSession, arg string) (*model.ChannelDispatchResult, error) {
	char, err := s.resolveCharacter(owner.ID, session, arg)
	if err != nil {
		if err == sql.ErrNoRows {
			return &model.ChannelDispatchResult{
				ShouldReply: true,
				Reason:      "command_handled",
				Action:      "show_character_card",
				ReplyText:   "没有找到对应角色卡。先发送“角色列表”看看可选项。",
			}, nil
		}
		return nil, err
	}

	lines := []string{
		fmt.Sprintf("角色：%s", char.Name),
		fmt.Sprintf("标签：%s", defaultIfEmpty(char.Tags, "无")),
		fmt.Sprintf("人称：%s", char.POV),
		fmt.Sprintf("简介：%s", clipText(char.Description, 120)),
		fmt.Sprintf("性格：%s", clipText(char.Personality, 120)),
		fmt.Sprintf("场景：%s", clipText(char.Scenario, 120)),
		fmt.Sprintf("开场：%s", clipText(char.FirstMsg, 120)),
	}
	return &model.ChannelDispatchResult{
		ShouldReply: true,
		Reason:      "command_handled",
		Action:      "show_character_card",
		ReplyText:   strings.Join(lines, "\n"),
	}, nil
}

func (s *ChannelDispatchService) selectCharacter(owner *model.User, session *model.ChannelSession, arg string) (*model.ChannelDispatchResult, error) {
	if strings.TrimSpace(arg) == "" {
		return &model.ChannelDispatchResult{
			ShouldReply: true,
			Reason:      "command_handled",
			Action:      "select_character",
			ReplyText:   "请指定角色名或序号，例如：选择角色 2",
		}, nil
	}

	char, err := s.resolveCharacter(owner.ID, session, arg)
	if err != nil {
		if err == sql.ErrNoRows {
			return &model.ChannelDispatchResult{
				ShouldReply: true,
				Reason:      "command_handled",
				Action:      "select_character",
				ReplyText:   "没有找到对应角色卡。先发送“角色列表”看看可选项。",
			}, nil
		}
		return nil, err
	}

	if err := s.channelStore.UpdateSessionCharacter(session.ID, char.ID); err != nil {
		return nil, err
	}
	return &model.ChannelDispatchResult{
		ShouldReply: true,
		Reason:      "command_handled",
		Action:      "select_character",
		ReplyText:   fmt.Sprintf("已选择角色：%s\n当前会话已清空。你可以发送“新聊天”开始新会话。", char.Name),
	}, nil
}

func (s *ChannelDispatchService) currentCharacter(owner *model.User, session *model.ChannelSession) (*model.ChannelDispatchResult, error) {
	if strings.TrimSpace(session.ActiveCharacterID) == "" {
		return &model.ChannelDispatchResult{
			ShouldReply: true,
			Reason:      "command_handled",
			Action:      "current_character",
			ReplyText:   "当前还没有选中角色。先发送“角色列表”或“选择角色 2”。",
		}, nil
	}

	char, err := s.characterStore.GetByID(session.ActiveCharacterID, owner.ID)
	if err != nil {
		return &model.ChannelDispatchResult{
			ShouldReply: true,
			Reason:      "command_handled",
			Action:      "current_character",
			ReplyText:   "当前角色不存在了。请重新发送“角色列表”选择角色。",
		}, nil
	}

	return &model.ChannelDispatchResult{
		ShouldReply: true,
		Reason:      "command_handled",
		Action:      "current_character",
		ReplyText:   fmt.Sprintf("当前角色：%s\n标签：%s", char.Name, defaultIfEmpty(char.Tags, "无")),
	}, nil
}

func (s *ChannelDispatchService) newChat(owner *model.User, session *model.ChannelSession, arg string) (*model.ChannelDispatchResult, error) {
	char, err := s.resolveCharacter(owner.ID, session, arg)
	if err != nil {
		if err == sql.ErrNoRows {
			return &model.ChannelDispatchResult{
				ShouldReply: true,
				Reason:      "command_handled",
				Action:      "new_chat",
				ReplyText:   "当前还没有可用角色。先发送“角色列表”，再发送“新聊天 2”。",
			}, nil
		}
		return nil, err
	}

	chat := &model.Chat{
		CharacterID: char.ID,
		Title:       fmt.Sprintf("%s %s", char.Name, time.Now().Format("01-02 15:04")),
	}
	if err := s.chatStore.Create(chat, owner.ID); err != nil {
		return nil, err
	}
	if err := s.channelStore.EnsureChatLink(napCatProvider, session.SelfID, session.ExternalUserID, owner.ID, chat.ID); err != nil {
		return nil, err
	}
	if err := s.channelStore.UpdateSessionChat(session.ID, char.ID, chat.ID); err != nil {
		return nil, err
	}

	return &model.ChannelDispatchResult{
		ShouldReply: true,
		Reason:      "command_handled",
		Action:      "new_chat",
		ReplyText:   fmt.Sprintf("已创建新聊天：%s\n当前角色：%s", chat.Title, char.Name),
	}, nil
}

func (s *ChannelDispatchService) listSessions(owner *model.User, session *model.ChannelSession) (*model.ChannelDispatchResult, error) {
	list, err := s.channelStore.ListChatLinks(napCatProvider, session.SelfID, session.ExternalUserID, owner.ID)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return &model.ChannelDispatchResult{
			ShouldReply: true,
			Reason:      "command_handled",
			Action:      "list_sessions",
			ReplyText:   "当前还没有会话。发送“新聊天”开始。",
		}, nil
	}

	lines := []string{"会话列表："}
	for idx, item := range list {
		current := ""
		if item.ChatID == session.ActiveChatID {
			current = " [当前]"
		}
		lines = append(lines, fmt.Sprintf("%d. %s%s (%s)", idx+1, item.Title, current, defaultIfEmpty(item.CharacterName, "未知角色")))
	}
	lines = append(lines, "可发送：切换会话 2")
	return &model.ChannelDispatchResult{
		ShouldReply: true,
		Reason:      "command_handled",
		Action:      "list_sessions",
		ReplyText:   strings.Join(lines, "\n"),
	}, nil
}

func (s *ChannelDispatchService) switchSession(owner *model.User, session *model.ChannelSession, arg string) (*model.ChannelDispatchResult, error) {
	index, err := strconv.Atoi(strings.TrimSpace(arg))
	if err != nil || index < 1 {
		return &model.ChannelDispatchResult{
			ShouldReply: true,
			Reason:      "command_handled",
			Action:      "switch_session",
			ReplyText:   "请使用序号切换会话，例如：切换会话 2",
		}, nil
	}

	item, err := s.channelStore.GetChatLinkByIndex(napCatProvider, session.SelfID, session.ExternalUserID, owner.ID, index)
	if err != nil {
		if err == sql.ErrNoRows {
			return &model.ChannelDispatchResult{
				ShouldReply: true,
				Reason:      "command_handled",
				Action:      "switch_session",
				ReplyText:   "没有这个会话序号。先发送“会话列表”查看。",
			}, nil
		}
		return nil, err
	}

	if err := s.channelStore.UpdateSessionChat(session.ID, item.CharacterID, item.ChatID); err != nil {
		return nil, err
	}
	return &model.ChannelDispatchResult{
		ShouldReply: true,
		Reason:      "command_handled",
		Action:      "switch_session",
		ReplyText:   fmt.Sprintf("已切换到会话：%s", item.Title),
	}, nil
}

func (s *ChannelDispatchService) status(owner *model.User, session *model.ChannelSession) (*model.ChannelDispatchResult, error) {
	currentCharacter := "未选择"
	if strings.TrimSpace(session.ActiveCharacterID) != "" {
		if char, err := s.characterStore.GetByID(session.ActiveCharacterID, owner.ID); err == nil {
			currentCharacter = char.Name
		}
	}

	currentChat := "未开始"
	if strings.TrimSpace(session.ActiveChatID) != "" {
		if item, err := s.channelStore.GetChatLinkByID(napCatProvider, session.SelfID, session.ExternalUserID, owner.ID, session.ActiveChatID); err == nil {
			currentChat = item.Title
		}
	}

	reply := strings.Join([]string{
		fmt.Sprintf("承载用户：%s", owner.Username),
		fmt.Sprintf("当前角色：%s", currentCharacter),
		fmt.Sprintf("当前会话：%s", currentChat),
		fmt.Sprintf("外部联系人：%s", session.ExternalUserID),
	}, "\n")
	return &model.ChannelDispatchResult{
		ShouldReply: true,
		Reason:      "command_handled",
		Action:      "status",
		ReplyText:   reply,
	}, nil
}

func (s *ChannelDispatchService) resetSession(owner *model.User, session *model.ChannelSession) (*model.ChannelDispatchResult, error) {
	if err := s.channelStore.ResetSession(session.ID); err != nil {
		return nil, err
	}
	return &model.ChannelDispatchResult{
		ShouldReply: true,
		Reason:      "command_handled",
		Action:      "reset_session",
		ReplyText:   "当前会话已重置。历史记录保留，当前角色不变。",
	}, nil
}

func (s *ChannelDispatchService) resolveCharacter(ownerUserID string, session *model.ChannelSession, arg string) (*model.Character, error) {
	if strings.TrimSpace(arg) == "" {
		if strings.TrimSpace(session.ActiveCharacterID) == "" {
			return nil, sql.ErrNoRows
		}
		return s.characterStore.GetByID(session.ActiveCharacterID, ownerUserID)
	}

	list, err := s.characterStore.List(ownerUserID)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, sql.ErrNoRows
	}

	if idx, err := strconv.Atoi(strings.TrimSpace(arg)); err == nil {
		if idx < 1 || idx > len(list) {
			return nil, sql.ErrNoRows
		}
		return list[idx-1], nil
	}

	for _, char := range list {
		if char.Name == strings.TrimSpace(arg) || strings.EqualFold(char.Name, strings.TrimSpace(arg)) {
			return char, nil
		}
	}
	return nil, sql.ErrNoRows
}

func clipText(text string, limit int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "无"
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "..."
}

func defaultIfEmpty(text, fallback string) string {
	if strings.TrimSpace(text) == "" {
		return fallback
	}
	return strings.TrimSpace(text)
}
