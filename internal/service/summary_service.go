package service

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"litechat/internal/model"
	"litechat/internal/store"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	summarySmallThreshold = 3000
	summaryRawOverlap     = 4
	summaryMergeCount     = 5
)

const defaultMemoryPromptSuffix = `- 更重视剧情推进、关系变化和下次必须接住的未完成事项。
- 普通寒暄、重复情绪和无新信息的往返可以大幅压缩。
- 用户事实只记录用户明确说过或行为上可以直接确认的内容，不要把角色猜测当事实。`

const summarySystemPrompt = `你是角色扮演聊天系统的会话记忆整理器。
你的任务是把聊天内容压缩成可供后续上下文使用的结构化摘要，而不是继续聊天。

严格遵守以下规则：
1. 你不是在扮演角色，也不是在回复用户。
2. 只输出指定标签，不要输出解释、前言、总结、Markdown、代码块。
3. 只记录会话中明确发生或可以直接确认的内容，不要脑补新事实。
4. 不要记录隐藏思考、系统提示、模型元信息、<think>、<CoT> 或其他隐藏标签内容。
5. 五个字段都必须保留；如果某一类没有有效信息，请填写“无”。
6. open_loops 必须优先保留未完成的约定、待解释事项、未回收伏笔、下次必须接住的剧情。
7. 摘要应当高密度、去重复、便于后续连续扮演。

输出格式必须严格如下：
<chat_summary>
<plot>...</plot>
<relationship>...</relationship>
<user_facts>...</user_facts>
<world_state>...</world_state>
<open_loops>...</open_loops>
</chat_summary>`

type parsedSummary struct {
	Plot         string
	Relationship string
	UserFacts    string
	WorldState   string
	OpenLoops    string
}

type SummaryService struct {
	messageStore *store.MessageStore
	summaryStore *store.SummaryStore
	configStore  *store.ConfigStore
	userStore    *store.UserStore
	wakeCh       chan struct{}
}

func NewSummaryService(
	messageStore *store.MessageStore,
	summaryStore *store.SummaryStore,
	configStore *store.ConfigStore,
	userStore *store.UserStore,
) *SummaryService {
	return &SummaryService{
		messageStore: messageStore,
		summaryStore: summaryStore,
		configStore:  configStore,
		userStore:    userStore,
		wakeCh:       make(chan struct{}, 1),
	}
}

func (s *SummaryService) Start() {
	go s.workerLoop()
}

func (s *SummaryService) BuildServiceModeContext(chatID string, history []*model.Message) (string, []*model.Message) {
	if !s.isEnabled() {
		return "", history
	}

	state, err := s.summaryStore.GetState(chatID)
	if err != nil {
		log.Printf("[摘要] 读取状态失败 chat=%s: %v", chatID, err)
		return "", history
	}

	bigSummary, smallSummaries, coverageTo, err := s.resolveUsableSummaryCoverage(chatID, state.AppliedCutoffSeq)
	if err != nil {
		log.Printf("[摘要] 解析可用摘要前缀失败 chat=%s: %v", chatID, err)
		return "", history
	}

	var summaryBlocks []string
	if bigSummary != nil {
		summaryBlocks = append(summaryBlocks, renderSummaryChunkForContext("会话大摘要", bigSummary.Content))
	}
	for i, chunk := range smallSummaries {
		summaryBlocks = append(summaryBlocks, renderSummaryChunkForContext(fmt.Sprintf("会话小摘要 %d", i+1), chunk.Content))
	}
	if len(summaryBlocks) == 0 {
		return "", history
	}

	if coverageTo <= 0 {
		return strings.Join(summaryBlocks, "\n\n"), history
	}

	rawStartSeq := coverageTo - summaryRawOverlap + 1
	if rawStartSeq < 1 {
		rawStartSeq = 1
	}

	filtered := make([]*model.Message, 0, len(history))
	for _, msg := range history {
		if msg.Seq >= rawStartSeq {
			filtered = append(filtered, msg)
		}
	}

	return strings.Join(summaryBlocks, "\n\n"), filtered
}

func (s *SummaryService) resolveUsableSummaryCoverage(chatID string, maxToSeq int) (*model.ChatSummaryChunk, []*model.ChatSummaryChunk, int, error) {
	if maxToSeq <= 0 {
		return nil, nil, 0, nil
	}

	bigChunk, err := s.summaryStore.GetLatestUsableBigChunk(chatID, maxToSeq)
	if err != nil {
		return nil, nil, 0, err
	}
	if bigChunk != nil && bigChunk.FromSeq != 1 {
		bigChunk = nil
	}

	smallChunks, err := s.summaryStore.ListUsableSmallChunks(chatID, maxToSeq)
	if err != nil {
		return nil, nil, 0, err
	}

	coverageTo := 0
	if bigChunk != nil {
		coverageTo = bigChunk.ToSeq
	}

	usableSmalls := make([]*model.ChatSummaryChunk, 0, len(smallChunks))
	for _, chunk := range smallChunks {
		switch {
		case coverageTo == 0:
			if chunk.FromSeq != 1 {
				continue
			}
			usableSmalls = append(usableSmalls, chunk)
			coverageTo = chunk.ToSeq
		case chunk.ToSeq <= coverageTo:
			continue
		case chunk.FromSeq != coverageTo+1:
			return bigChunk, usableSmalls, coverageTo, nil
		default:
			usableSmalls = append(usableSmalls, chunk)
			coverageTo = chunk.ToSeq
		}
	}

	if bigChunk == nil && len(usableSmalls) == 0 {
		return nil, nil, 0, nil
	}

	return bigChunk, usableSmalls, coverageTo, nil
}

func (s *SummaryService) OnAssistantMessageStored(chatID string) {
	if !s.isEnabled() {
		return
	}
	if err := s.scheduleSmallIfNeeded(chatID, false); err != nil {
		log.Printf("[摘要] 调度小摘要失败 chat=%s: %v", chatID, err)
		return
	}
	s.wake()
}

func (s *SummaryService) InvalidateFromSeq(chatID string, fromSeq int) {
	if !s.isEnabled() || fromSeq <= 0 {
		return
	}

	state, err := s.summaryStore.GetState(chatID)
	if err != nil {
		log.Printf("[摘要] 读取状态失败 chat=%s: %v", chatID, err)
		return
	}

	forceRebuild := state.AppliedCutoffSeq >= fromSeq
	if err := s.summaryStore.MarkChunksDirtyFromSeq(chatID, fromSeq); err != nil {
		log.Printf("[摘要] 标记 dirty 失败 chat=%s: %v", chatID, err)
		return
	}
	if err := s.summaryStore.ResetCurrentBigSummaryIfDirty(chatID); err != nil {
		log.Printf("[摘要] 清理大摘要指针失败 chat=%s: %v", chatID, err)
		return
	}

	newCutoff := state.AppliedCutoffSeq
	if forceRebuild {
		_, _, recoveredCutoff, err := s.resolveUsableSummaryCoverage(chatID, fromSeq-1)
		if err != nil {
			log.Printf("[摘要] 回收可用摘要前缀失败 chat=%s: %v", chatID, err)
			return
		}
		newCutoff = recoveredCutoff
	}
	if newCutoff < 0 {
		newCutoff = 0
	}

	if err := s.summaryStore.RollbackCutoff(chatID, newCutoff, fromSeq); err != nil {
		log.Printf("[摘要] 回退 cutoff 失败 chat=%s: %v", chatID, err)
		return
	}
	if err := s.scheduleSmallIfNeeded(chatID, forceRebuild); err != nil {
		log.Printf("[摘要] 失效后重新调度失败 chat=%s: %v", chatID, err)
	}
	s.wake()
}

func (s *SummaryService) scheduleSmallIfNeeded(chatID string, force bool) error {
	state, err := s.summaryStore.GetState(chatID)
	if err != nil {
		return err
	}

	latestSeq, err := s.messageStore.LatestSeq(chatID)
	if err != nil {
		return err
	}
	if latestSeq <= state.AppliedCutoffSeq {
		return s.scheduleBigIfNeeded(chatID)
	}

	messages, err := s.messageStore.ListByChatIDRange(chatID, state.AppliedCutoffSeq+1, latestSeq)
	if err != nil {
		return err
	}
	if len(messages) == 0 {
		return s.scheduleBigIfNeeded(chatID)
	}
	if !force && countEffectiveChars(messages) < summarySmallThreshold {
		return s.scheduleBigIfNeeded(chatID)
	}

	if err := s.summaryStore.ScheduleSmallJob(chatID, state.AppliedCutoffSeq+1, latestSeq, state.AppliedCutoffSeq); err != nil {
		return err
	}
	return s.scheduleBigIfNeeded(chatID)
}

func (s *SummaryService) scheduleBigIfNeeded(chatID string) error {
	count, err := s.summaryStore.CountActiveSmallChunks(chatID)
	if err != nil {
		return err
	}
	if count < summaryMergeCount {
		return nil
	}

	smalls, err := s.summaryStore.ListActiveSmallChunks(chatID)
	if err != nil {
		return err
	}
	if len(smalls) < summaryMergeCount {
		return nil
	}

	return s.summaryStore.ScheduleBigJob(chatID, smalls[0].FromSeq, smalls[summaryMergeCount-1].ToSeq, 0)
}

func (s *SummaryService) workerLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
		case <-s.wakeCh:
		}

		for {
			processed, err := s.processNextJob()
			if err != nil {
				log.Printf("[摘要] worker 执行失败: %v", err)
				break
			}
			if !processed {
				break
			}
		}
	}
}

func (s *SummaryService) processNextJob() (bool, error) {
	if !s.isEnabled() {
		return false, nil
	}

	job, err := s.summaryStore.ClaimNextJob()
	if err != nil {
		return false, err
	}
	if job == nil {
		return false, nil
	}

	var runErr error
	switch job.JobType {
	case "small":
		runErr = s.runSmallJob(job)
	case "big":
		runErr = s.runBigJob(job)
	default:
		runErr = fmt.Errorf("未知摘要任务类型: %s", job.JobType)
	}

	if runErr == nil {
		if err := s.summaryStore.CompleteJob(job.ID); err != nil {
			return true, err
		}
		return true, nil
	}

	if strings.HasPrefix(runErr.Error(), "stale:") {
		if err := s.summaryStore.MarkJobStale(job.ID, runErr.Error()); err != nil {
			return true, err
		}
		return true, nil
	}

	attempt := job.AttemptCount + 1
	nextRunAt := time.Now().Add(nextRetryDelay(attempt))
	if err := s.summaryStore.FailJob(job.ID, attempt, nextRunAt, runErr.Error()); err != nil {
		return true, err
	}
	return true, nil
}

func (s *SummaryService) runSmallJob(job *model.ChatSummaryJob) error {
	state, err := s.summaryStore.GetState(job.ChatID)
	if err != nil {
		return err
	}
	if state.AppliedCutoffSeq >= job.ToSeq {
		return fmt.Errorf("stale: job range already summarized")
	}
	if state.DirtyFromSeq > 0 && state.DirtyFromSeq != job.FromSeq && state.DirtyFromSeq <= job.ToSeq {
		return fmt.Errorf("stale: job range invalidated by newer change")
	}

	settings, err := s.configStore.GetSettings()
	if err != nil {
		return err
	}
	if settings.ServiceMode != "service" {
		return fmt.Errorf("stale: summary disabled outside service mode")
	}

	rawMessages, err := s.messageStore.ListByChatIDRange(job.ChatID, job.FromSeq, job.ToSeq)
	if err != nil {
		return err
	}
	if len(rawMessages) == 0 {
		return fmt.Errorf("stale: no messages in target range")
	}

	activeBig, precedingSmalls, _, err := s.resolveUsableSummaryCoverage(job.ChatID, job.FromSeq-1)
	if err != nil {
		return err
	}
	sourceFingerprint := summarySourceFingerprint(activeBig, precedingSmalls, rawMessages)

	prompt := buildSmallSummaryPrompt(activeBig, precedingSmalls, rawMessages, settings.MemoryPromptSuffix)
	rawSummary, err := s.callSummaryCompletion(settings, prompt, 1200)
	if err != nil {
		return err
	}

	normalized, err := parseSummaryChunk(rawSummary)
	if err != nil {
		return err
	}

	state, err = s.summaryStore.GetState(job.ChatID)
	if err != nil {
		return err
	}
	if state.AppliedCutoffSeq >= job.ToSeq {
		return fmt.Errorf("stale: cutoff moved past current job")
	}

	currentRawMessages, err := s.messageStore.ListByChatIDRange(job.ChatID, job.FromSeq, job.ToSeq)
	if err != nil {
		return err
	}
	currentActiveBig, currentPrecedingSmalls, _, err := s.resolveUsableSummaryCoverage(job.ChatID, job.FromSeq-1)
	if err != nil {
		return err
	}
	if summarySourceFingerprint(currentActiveBig, currentPrecedingSmalls, currentRawMessages) != sourceFingerprint {
		return fmt.Errorf("stale: summary sources changed during generation")
	}

	chunk := &model.ChatSummaryChunk{
		ChatID:  job.ChatID,
		Level:   "small",
		FromSeq: job.FromSeq,
		ToSeq:   job.ToSeq,
		Content: normalized,
		Status:  "active",
	}
	if err := s.summaryStore.CreateChunk(chunk); err != nil {
		return err
	}
	if err := s.summaryStore.ApplySmallSummary(job.ChatID, job.ToSeq); err != nil {
		return err
	}
	return s.scheduleBigIfNeeded(job.ChatID)
}

func (s *SummaryService) runBigJob(job *model.ChatSummaryJob) error {
	settings, err := s.configStore.GetSettings()
	if err != nil {
		return err
	}
	if settings.ServiceMode != "service" {
		return fmt.Errorf("stale: summary disabled outside service mode")
	}

	activeBig, _ := s.summaryStore.GetActiveBigChunk(job.ChatID)
	activeSmalls, err := s.summaryStore.ListActiveSmallChunks(job.ChatID)
	if err != nil {
		return err
	}
	if len(activeSmalls) < summaryMergeCount {
		return fmt.Errorf("stale: active small summaries are not enough")
	}

	targetSmalls := activeSmalls[:summaryMergeCount]
	coverageTo := targetSmalls[len(targetSmalls)-1].ToSeq
	state, err := s.summaryStore.GetState(job.ChatID)
	if err != nil {
		return err
	}
	if state.DirtyFromSeq > 0 && state.DirtyFromSeq <= coverageTo {
		return fmt.Errorf("stale: summary range was invalidated before merge")
	}
	sourceFingerprint := summarySourceFingerprint(activeBig, targetSmalls, nil)
	prompt := buildBigSummaryPrompt(activeBig, targetSmalls, settings.MemoryPromptSuffix)
	rawSummary, err := s.callSummaryCompletion(settings, prompt, 1800)
	if err != nil {
		return err
	}

	normalized, err := parseSummaryChunk(rawSummary)
	if err != nil {
		return err
	}

	state, err = s.summaryStore.GetState(job.ChatID)
	if err != nil {
		return err
	}
	if state.DirtyFromSeq > 0 && state.DirtyFromSeq <= coverageTo {
		return fmt.Errorf("stale: summary range changed during merge")
	}
	currentActiveBig, _ := s.summaryStore.GetActiveBigChunk(job.ChatID)
	currentActiveSmalls, err := s.summaryStore.ListActiveSmallChunks(job.ChatID)
	if err != nil {
		return err
	}
	if len(currentActiveSmalls) < summaryMergeCount {
		return fmt.Errorf("stale: active small summaries changed during merge")
	}
	if summarySourceFingerprint(currentActiveBig, currentActiveSmalls[:summaryMergeCount], nil) != sourceFingerprint {
		return fmt.Errorf("stale: merge sources changed during generation")
	}

	if activeBig != nil {
		if err := s.summaryStore.SupersedeBigChunk(job.ChatID); err != nil {
			return err
		}
	}
	bigChunk := &model.ChatSummaryChunk{
		ChatID:  job.ChatID,
		Level:   "big",
		FromSeq: firstSeq(activeBig, targetSmalls),
		ToSeq:   targetSmalls[len(targetSmalls)-1].ToSeq,
		Content: normalized,
		Status:  "active",
	}
	if err := s.summaryStore.CreateChunk(bigChunk); err != nil {
		return err
	}
	smallIDs := make([]string, 0, len(targetSmalls))
	for _, chunk := range targetSmalls {
		smallIDs = append(smallIDs, chunk.ID)
	}
	if err := s.summaryStore.MarkSmallChunksMerged(smallIDs, bigChunk.ID); err != nil {
		return err
	}
	return s.summaryStore.SetCurrentBigSummary(job.ChatID, bigChunk.ID)
}

func (s *SummaryService) callSummaryCompletion(settings *model.AppSettings, prompt string, maxTokens int) (string, error) {
	modelName := strings.TrimSpace(settings.DefaultModel)
	if !settings.UseDefaultModelForMemory {
		if customModel := strings.TrimSpace(settings.MemoryModel); customModel != "" {
			modelName = customModel
		}
	}
	if modelName == "" {
		return "", fmt.Errorf("未配置可用模型")
	}

	reqBody := model.ChatCompletionRequest{
		Model: modelName,
		Messages: []model.ChatCompletionMessage{
			{Role: "system", Content: summarySystemPrompt},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.2,
		MaxTokens:   maxTokens,
		TopP:        0.9,
		Stream:      false,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	apiURL := strings.TrimRight(settings.APIEndpoint, "/") + "/chat/completions"
	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+settings.APIKey)

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("摘要请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("摘要请求错误 %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析摘要结果失败: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("摘要模型未返回内容")
	}

	content := strings.TrimSpace(result.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("摘要模型未返回内容")
	}
	return content, nil
}

func (s *SummaryService) isEnabled() bool {
	return s.userStore.GetCurrentMode() == "service"
}

func (s *SummaryService) wake() {
	select {
	case s.wakeCh <- struct{}{}:
	default:
	}
}

func buildSmallSummaryPrompt(
	activeBig *model.ChatSummaryChunk,
	activeSmalls []*model.ChatSummaryChunk,
	rawMessages []*model.Message,
	suffix string,
) string {
	var builder strings.Builder
	builder.WriteString("任务类型：小摘要。\n")
	builder.WriteString("请结合已有摘要状态，为新的会话片段生成一条新的结构化小摘要。\n\n")

	if activeBig != nil {
		builder.WriteString("[当前大摘要]\n")
		builder.WriteString(activeBig.Content)
		builder.WriteString("\n\n")
	}

	if len(activeSmalls) > 0 {
		builder.WriteString("[当前未合并小摘要]\n")
		for i, chunk := range activeSmalls {
			builder.WriteString(fmt.Sprintf("第%d条小摘要（覆盖 %d-%d）\n", i+1, chunk.FromSeq, chunk.ToSeq))
			builder.WriteString(chunk.Content)
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}

	builder.WriteString("[本次需要整理的新消息]\n")
	builder.WriteString(renderMessagesForSummary(rawMessages))
	builder.WriteString("\n")

	builder.WriteString("额外要求：\n")
	builder.WriteString("- 保持和已有摘要状态一致，不要改写已经确定的事实。\n")
	builder.WriteString("- 重点压缩重复寒暄和无信息密度的往返，但不要遗漏关键转折。\n")
	if strings.TrimSpace(suffix) != "" {
		builder.WriteString("- 管理员补充要求：\n")
		builder.WriteString(strings.TrimSpace(suffix))
		builder.WriteString("\n")
	}

	return builder.String()
}

func buildBigSummaryPrompt(
	activeBig *model.ChatSummaryChunk,
	targetSmalls []*model.ChatSummaryChunk,
	suffix string,
) string {
	var builder strings.Builder
	builder.WriteString("任务类型：大摘要。\n")
	builder.WriteString("请将已有大摘要与下面 5 条连续小摘要合并成一条新的大摘要，去重并保留仍然有效的信息。\n\n")

	if activeBig != nil {
		builder.WriteString("[已有大摘要]\n")
		builder.WriteString(activeBig.Content)
		builder.WriteString("\n\n")
	}

	builder.WriteString("[待合并的小摘要]\n")
	for i, chunk := range targetSmalls {
		builder.WriteString(fmt.Sprintf("第%d条小摘要（覆盖 %d-%d）\n", i+1, chunk.FromSeq, chunk.ToSeq))
		builder.WriteString(chunk.Content)
		builder.WriteString("\n")
	}
	builder.WriteString("\n额外要求：\n")
	builder.WriteString("- 合并时要去重、压缩重复表述，但保留剧情推进和关系变化。\n")
	builder.WriteString("- open_loops 只保留仍未解决、仍需要下次接住的事项。\n")
	if strings.TrimSpace(suffix) != "" {
		builder.WriteString("- 管理员补充要求：\n")
		builder.WriteString(strings.TrimSpace(suffix))
		builder.WriteString("\n")
	}

	return builder.String()
}

func renderMessagesForSummary(messages []*model.Message) string {
	var builder strings.Builder
	for _, msg := range messages {
		content := strings.TrimSpace(msg.Content)
		if msg.Role == "assistant" {
			content = cleanAssistantContent(content)
		}
		if content == "" {
			continue
		}
		builder.WriteString(fmt.Sprintf("[%d][%s] %s\n", msg.Seq, msg.Role, content))
	}
	return strings.TrimSpace(builder.String())
}

func parseSummaryChunk(raw string) (string, error) {
	cleaned := stripMarkdownCodeFence(cleanAssistantContent(raw))
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return "", fmt.Errorf("摘要结果为空")
	}

	summary := parsedSummary{
		Plot:         normalizeSummaryField(extractTaggedContent(cleaned, "plot")),
		Relationship: normalizeSummaryField(extractTaggedContent(cleaned, "relationship")),
		UserFacts:    normalizeSummaryField(extractTaggedContent(cleaned, "user_facts")),
		WorldState:   normalizeSummaryField(extractTaggedContent(cleaned, "world_state")),
		OpenLoops:    normalizeSummaryField(extractTaggedContent(cleaned, "open_loops")),
	}

	if summary.Plot == "" || summary.Relationship == "" || summary.UserFacts == "" || summary.WorldState == "" || summary.OpenLoops == "" {
		return "", fmt.Errorf("摘要字段不完整")
	}

	return fmt.Sprintf(
		"<chat_summary>\n<plot>%s</plot>\n<relationship>%s</relationship>\n<user_facts>%s</user_facts>\n<world_state>%s</world_state>\n<open_loops>%s</open_loops>\n</chat_summary>",
		summary.Plot, summary.Relationship, summary.UserFacts, summary.WorldState, summary.OpenLoops,
	), nil
}

func renderSummaryChunkForContext(title, raw string) string {
	summary := parsedSummary{
		Plot:         normalizeSummaryField(extractTaggedContent(raw, "plot")),
		Relationship: normalizeSummaryField(extractTaggedContent(raw, "relationship")),
		UserFacts:    normalizeSummaryField(extractTaggedContent(raw, "user_facts")),
		WorldState:   normalizeSummaryField(extractTaggedContent(raw, "world_state")),
		OpenLoops:    normalizeSummaryField(extractTaggedContent(raw, "open_loops")),
	}

	var builder strings.Builder
	builder.WriteString("[")
	builder.WriteString(title)
	builder.WriteString("]\n")
	builder.WriteString("剧情进展：")
	builder.WriteString(summary.Plot)
	builder.WriteString("\n关系变化：")
	builder.WriteString(summary.Relationship)
	builder.WriteString("\n用户事实：")
	builder.WriteString(summary.UserFacts)
	builder.WriteString("\n世界状态：")
	builder.WriteString(summary.WorldState)
	builder.WriteString("\n未完成事项：")
	builder.WriteString(summary.OpenLoops)
	return builder.String()
}

func normalizeSummaryField(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = multiNewlineRegex.ReplaceAllString(raw, "\n\n")
	if raw == "" {
		return "无"
	}
	return raw
}

func countEffectiveChars(messages []*model.Message) int {
	total := 0
	for _, msg := range messages {
		content := strings.TrimSpace(msg.Content)
		if msg.Role == "assistant" {
			content = cleanAssistantContent(content)
		}
		if content == "" {
			continue
		}
		total += len([]rune(content))
	}
	return total
}

func nextRetryDelay(attempt int) time.Duration {
	switch attempt {
	case 1:
		return time.Minute
	case 2:
		return 5 * time.Minute
	default:
		return 30 * time.Minute
	}
}

func firstSeq(activeBig *model.ChatSummaryChunk, chunks []*model.ChatSummaryChunk) int {
	if activeBig != nil && activeBig.FromSeq > 0 {
		return activeBig.FromSeq
	}
	if len(chunks) == 0 {
		return 0
	}
	return chunks[0].FromSeq
}

func summarySourceFingerprint(bigChunk *model.ChatSummaryChunk, smallChunks []*model.ChatSummaryChunk, messages []*model.Message) string {
	hasher := sha1.New()
	writeChunk := func(prefix string, chunk *model.ChatSummaryChunk) {
		if chunk == nil {
			return
		}
		_, _ = hasher.Write([]byte(fmt.Sprintf("%s|%s|%d|%d|%s\n", prefix, chunk.ID, chunk.FromSeq, chunk.ToSeq, chunk.Content)))
	}
	for _, chunk := range smallChunks {
		writeChunk("small", chunk)
	}
	writeChunk("big", bigChunk)
	for _, msg := range messages {
		_, _ = hasher.Write([]byte(fmt.Sprintf("msg|%s|%d|%s|%s\n", msg.ID, msg.Seq, msg.Role, msg.Content)))
	}
	return hex.EncodeToString(hasher.Sum(nil))
}
