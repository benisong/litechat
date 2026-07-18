package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"litechat/internal/model"
	"litechat/internal/statusbar"
	"litechat/internal/store"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

const (
	defaultSummaryCharLimit = 3000
	summaryMaxTokens        = 2000
	summaryWarningCount     = 100
	summaryJobLease         = 2 * time.Minute
)

const defaultMemoryPromptSuffix = `- 必须保留会影响后续回复的稳定事实：人物、地点、物品、组织、时间线、约定、偏好、禁忌、称呼、任务状态。
- 对重要事件保留具体对象、原因、结果和当前状态，不要只写“关系变近”“继续推进”等空泛描述。
- 更重视剧情推进、关系变化和下次必须接住的未完成事项。
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
8. 不要因为信息短就丢弃；只要会影响后续回复、角色状态或用户偏好，就必须保留。
9. 角色卡内容只用于理解人物、称呼、视角和设定边界，不得把静态角色卡资料复制进摘要。
10. 状态栏是独立展示数据，不属于聊天记录；不得摘要或输出状态栏字段。

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
	messageStore   *store.MessageStore
	summaryStore   *store.SummaryStore
	characterStore *store.CharacterStore
	configStore    *store.ConfigStore
	userStore      *store.UserStore
	enabled        atomic.Bool
	runtimeCache   atomic.Pointer[summaryRuntimeCache]
}

// summaryRuntimeCache is immutable after publication. Chat requests only perform
// one atomic load, so they never wait for summary database or network work.
type summaryRuntimeCache struct {
	epoch           uint64
	chatGenerations map[string]uint64
	contexts        map[string]summaryContextSnapshot
	warnings        map[string]string
	suspended       map[string]bool
}

type summaryContextSnapshot struct {
	content    string
	coverageTo int
}

type summaryCacheVersion struct {
	epoch          uint64
	chatGeneration uint64
}

// summaryPlan is a fixed snapshot of one leased background summary job.
type summaryPlan struct {
	job          *store.PendingSummaryJob
	activeBig    *model.ChatSummaryChunk
	activeSmalls []*model.ChatSummaryChunk
	rawMessages  []*model.Message
	character    *model.Character
	firstSummary bool
}

func NewSummaryService(
	messageStore *store.MessageStore,
	summaryStore *store.SummaryStore,
	characterStore *store.CharacterStore,
	configStore *store.ConfigStore,
	userStore *store.UserStore,
) *SummaryService {
	service := &SummaryService{
		messageStore:   messageStore,
		summaryStore:   summaryStore,
		characterStore: characterStore,
		configStore:    configStore,
		userStore:      userStore,
	}
	service.runtimeCache.Store(newSummaryRuntimeCache(1))
	service.enabled.Store(userStore.GetCurrentMode() == "service")
	if err := summaryStore.RecoverInterruptedSummaryJobs(); err != nil {
		log.Printf("[summary] 恢复中断任务失败: %v", err)
	}
	if service.isEnabled() {
		if err := service.warmRuntimeCache(); err != nil {
			log.Printf("[summary] 启动时加载摘要快照失败: %v", err)
		}
	}
	return service
}

func (s *SummaryService) BuildServiceModeContext(chatID string, history []*model.Message) (string, []*model.Message) {
	if !s.isEnabled() {
		return "", history
	}

	cache := s.runtimeCache.Load()
	if cache == nil {
		return "", history
	}
	if cache.suspended[chatID] {
		return "", history
	}
	context, hasSummary := cache.contexts[chatID]
	if !hasSummary || context.content == "" || context.coverageTo <= 0 {
		return "", history
	}

	filtered := make([]*model.Message, 0, len(history))
	for _, msg := range history {
		if msg.Seq > context.coverageTo {
			filtered = append(filtered, msg)
		}
	}
	return context.content, filtered
}

// QueueSummaryIfNeeded records the assistant boundary to summarize on a later user message.
// It never calls the model and therefore never delays the reply that crossed the threshold.
func (s *SummaryService) QueueSummaryIfNeeded(chatID string, history []*model.Message) (bool, error) {
	if !s.isEnabled() {
		return false, nil
	}
	version := s.currentCacheVersion(chatID)

	state, err := s.reconcileSummaryState(chatID)
	if err != nil {
		return false, err
	}
	bigSummary, smallSummaries, coverageTo, err := s.resolveUsableSummaryCoverage(chatID, state.AppliedCutoffSeq)
	if err != nil {
		return false, err
	}
	s.publishMaterializedCache(chatID, version, bigSummary, smallSummaries, coverageTo, countMessagesAfterSeq(history, coverageTo))

	unsummarized := make([]*model.Message, 0, len(history))
	for _, msg := range history {
		if msg.Seq > coverageTo {
			unsummarized = append(unsummarized, msg)
		}
	}
	required := countEffectiveChars(unsummarized) >= s.summaryCharLimit()
	toSeq := 0
	if required {
		toSeq = selectSummaryEnd(unsummarized)
	}
	currentFloor := 0
	for _, msg := range history {
		if msg.Role == "assistant" {
			currentFloor++
		}
	}
	queued, err := s.summaryStore.UpdateSummaryEligibility(
		chatID,
		required,
		currentFloor,
		toSeq,
		latestMessageSeq(history),
	)
	if err == nil && queued {
		log.Printf("[summary] 已登记异步摘要 chat=%s from=%d to=%d floor=%d", chatID, coverageTo+1, toSeq, currentFloor)
	}
	return queued, err
}

// StartTurnSummaryAsync claims a boundary already recorded after an earlier
// assistant reply. The returned stage is only consumed by another background job.
func (s *SummaryService) StartTurnSummaryAsync(chatID string, userMessageSeq int) <-chan struct{} {
	stageDone := make(chan struct{})
	if !s.isEnabled() {
		close(stageDone)
		return stageDone
	}
	go func() {
		defer close(stageDone)
		job, err := s.summaryStore.ClaimPendingSummaryBefore(
			chatID,
			time.Now().Add(-summaryJobLease),
			userMessageSeq,
		)
		if err != nil {
			log.Printf("[summary] 领取异步摘要失败 chat=%s: %v", chatID, err)
			return
		}
		if job != nil {
			s.runClaimedSummaryLogged(job)
		}
	}()
	return stageDone
}

// FinishTurnSummaryAsync records the newest assistant boundary after the previous
// background stage. It reloads the latest history so delayed turns cannot overwrite
// newer eligibility state with stale snapshots.
func (s *SummaryService) FinishTurnSummaryAsync(chatID string, previousStage <-chan struct{}) <-chan struct{} {
	done := make(chan struct{})
	if !s.isEnabled() {
		close(done)
		return done
	}
	go func() {
		defer close(done)
		if previousStage != nil {
			<-previousStage
		}
		history, err := s.messageStore.ListByChatID(chatID)
		if err != nil {
			log.Printf("[summary] 后台读取最新历史失败 chat=%s: %v", chatID, err)
			return
		}
		if _, err := s.QueueSummaryIfNeeded(chatID, history); err != nil {
			log.Printf("[summary] 后台登记下一轮摘要失败 chat=%s: %v", chatID, err)
		}
	}()
	return done
}

// StartPendingSummary remains available for recovery paths and never performs
// database work on its caller's goroutine.
func (s *SummaryService) StartPendingSummary(chatID string, userMessageSeq int) {
	if !s.isEnabled() {
		return
	}
	go func() {
		job, err := s.summaryStore.ClaimPendingSummaryBefore(
			chatID,
			time.Now().Add(-summaryJobLease),
			userMessageSeq,
		)
		if err != nil {
			log.Printf("[summary] 领取异步摘要失败 chat=%s: %v", chatID, err)
			return
		}
		if job == nil {
			return
		}
		s.runClaimedSummaryLogged(job)
	}()
}

func (s *SummaryService) runClaimedSummaryLogged(job *store.PendingSummaryJob) {
	if err := s.runClaimedSummary(job); err != nil {
		log.Printf("[summary] 异步摘要失败 chat=%s: %v", job.ChatID, err)
	}
}

func (s *SummaryService) runPendingSummary(chatID string) error {
	job, err := s.summaryStore.ClaimPendingSummary(chatID, time.Now().Add(-summaryJobLease))
	if err != nil || job == nil {
		return err
	}
	return s.runClaimedSummary(job)
}

func (s *SummaryService) runClaimedSummary(job *store.PendingSummaryJob) error {
	plan, err := s.buildPendingSummaryPlan(job)
	if err != nil {
		_ = s.summaryStore.FailPendingSummary(job, err)
		s.refreshRuntimeCacheAsync(job.ChatID)
		return err
	}
	summaryContent, err := s.GeneratePlannedSummary(plan)
	if err != nil {
		_ = s.summaryStore.FailPendingSummary(job, err)
		s.refreshRuntimeCacheAsync(job.ChatID)
		return err
	}
	if err := s.summaryStore.CompletePendingSummary(job, summaryContent, s.summaryCharLimit()); err != nil {
		_ = s.summaryStore.FailPendingSummary(job, err)
		s.refreshRuntimeCacheAsync(job.ChatID)
		return err
	}
	if err := s.refreshRuntimeCache(job.ChatID); err != nil {
		log.Printf("[summary] 摘要成功但刷新内存快照失败 chat=%s: %v", job.ChatID, err)
	}
	log.Printf("[summary] 异步摘要已生效 chat=%s summary=1-%d attempt=%d", job.ChatID, job.ToSeq, job.Attempt)
	return nil
}

func (s *SummaryService) buildPendingSummaryPlan(job *store.PendingSummaryJob) (*summaryPlan, error) {
	if job == nil {
		return nil, fmt.Errorf("摘要任务为空")
	}
	activeBig, activeSmalls, coverageTo, err := s.resolveUsableSummaryCoverage(job.ChatID, job.BaseCutoffSeq)
	if err != nil {
		return nil, err
	}
	if coverageTo != job.BaseCutoffSeq {
		return nil, store.ErrSummaryStateChanged
	}
	character, err := s.characterStore.GetByChatID(job.ChatID)
	if err != nil {
		return nil, fmt.Errorf("读取摘要角色卡失败: %w", err)
	}
	rawMessages, err := s.messageStore.ListByChatIDRange(job.ChatID, coverageTo+1, job.ToSeq)
	if err != nil {
		return nil, err
	}
	if len(rawMessages) == 0 {
		return nil, store.ErrSummaryStateChanged
	}
	last := rawMessages[len(rawMessages)-1]
	if last.Seq != job.ToSeq || last.ID != job.TargetMessageID || last.Role != "assistant" {
		return nil, store.ErrSummaryStateChanged
	}
	return &summaryPlan{
		job:          job,
		activeBig:    activeBig,
		activeSmalls: activeSmalls,
		rawMessages:  rawMessages,
		character:    character,
		firstSummary: coverageTo == 0 && activeBig == nil && len(activeSmalls) == 0,
	}, nil
}

// GeneratePlannedSummary performs only the remote model request; it owns no database transaction.
func (s *SummaryService) GeneratePlannedSummary(plan *summaryPlan) (string, error) {
	if plan == nil || plan.job == nil {
		return "", fmt.Errorf("摘要计划为空")
	}
	settings, err := s.configStore.GetSettings()
	if err != nil {
		return "", err
	}
	if settings.ServiceMode != "service" {
		return "", fmt.Errorf("当前不在 service 模式")
	}

	prompt := buildRollingSummaryPrompt(
		plan.character,
		plan.firstSummary,
		plan.activeBig,
		plan.activeSmalls,
		plan.rawMessages,
		plan.job.BaseCutoffSeq+1,
		plan.job.ToSeq,
		settings.MemoryPromptSuffix,
	)
	rawSummary, err := s.callSummaryCompletion(settings, prompt, summaryMaxTokens)
	if err != nil {
		return "", err
	}
	return parseSummaryChunk(rawSummary)
}

func (s *SummaryService) Warning(chatID string) string {
	if !s.isEnabled() {
		return ""
	}
	cache := s.runtimeCache.Load()
	if cache == nil {
		return ""
	}
	return cache.warnings[chatID]
}

func newSummaryRuntimeCache(epoch uint64) *summaryRuntimeCache {
	return &summaryRuntimeCache{
		epoch:           epoch,
		chatGenerations: make(map[string]uint64),
		contexts:        make(map[string]summaryContextSnapshot),
		warnings:        make(map[string]string),
		suspended:       make(map[string]bool),
	}
}

func cloneSummaryRuntimeCache(current *summaryRuntimeCache) *summaryRuntimeCache {
	if current == nil {
		return newSummaryRuntimeCache(1)
	}
	next := newSummaryRuntimeCache(current.epoch)
	for chatID, generation := range current.chatGenerations {
		next.chatGenerations[chatID] = generation
	}
	for chatID, context := range current.contexts {
		next.contexts[chatID] = context
	}
	for chatID, warning := range current.warnings {
		next.warnings[chatID] = warning
	}
	for chatID, suspended := range current.suspended {
		next.suspended[chatID] = suspended
	}
	return next
}

func (s *SummaryService) currentCacheVersion(chatID string) summaryCacheVersion {
	cache := s.runtimeCache.Load()
	if cache == nil {
		return summaryCacheVersion{}
	}
	return summaryCacheVersion{
		epoch:          cache.epoch,
		chatGeneration: cache.chatGenerations[chatID],
	}
}

func (s *SummaryService) publishMaterializedCache(
	chatID string,
	version summaryCacheVersion,
	bigSummary *model.ChatSummaryChunk,
	smallSummaries []*model.ChatSummaryChunk,
	coverageTo int,
	unsummarizedCount int,
) {
	var context *summaryContextSnapshot
	var summaryBlocks []string
	if bigSummary != nil {
		summaryBlocks = append(summaryBlocks, renderSummaryChunkForContext("会话大摘要", bigSummary.Content))
	}
	for i, chunk := range smallSummaries {
		summaryBlocks = append(summaryBlocks, renderSummaryChunkForContext(fmt.Sprintf("会话小摘要 %d", i+1), chunk.Content))
	}
	if len(summaryBlocks) > 0 && coverageTo > 0 {
		context = &summaryContextSnapshot{
			content:    strings.Join(summaryBlocks, "\n\n"),
			coverageTo: coverageTo,
		}
	}
	s.publishChatCache(chatID, version, context, summaryWarning(unsummarizedCount, coverageTo))
}

func (s *SummaryService) publishChatCache(
	chatID string,
	version summaryCacheVersion,
	context *summaryContextSnapshot,
	warning string,
) {
	for {
		current := s.runtimeCache.Load()
		if current == nil || current.epoch != version.epoch ||
			current.chatGenerations[chatID] != version.chatGeneration || current.suspended[chatID] {
			return
		}
		next := cloneSummaryRuntimeCache(current)
		if context == nil {
			delete(next.contexts, chatID)
		} else {
			next.contexts[chatID] = *context
		}
		if warning == "" {
			delete(next.warnings, chatID)
		} else {
			next.warnings[chatID] = warning
		}
		if s.runtimeCache.CompareAndSwap(current, next) {
			return
		}
	}
}

func (s *SummaryService) evictRuntimeCache(chatID string) {
	for {
		current := s.runtimeCache.Load()
		if current == nil {
			return
		}
		next := cloneSummaryRuntimeCache(current)
		next.chatGenerations[chatID]++
		delete(next.contexts, chatID)
		delete(next.warnings, chatID)
		delete(next.suspended, chatID)
		if s.runtimeCache.CompareAndSwap(current, next) {
			return
		}
	}
}

func (s *SummaryService) suspendRuntimeCache(chatID string) {
	for {
		current := s.runtimeCache.Load()
		if current == nil {
			return
		}
		next := cloneSummaryRuntimeCache(current)
		next.chatGenerations[chatID]++
		next.suspended[chatID] = true
		delete(next.contexts, chatID)
		delete(next.warnings, chatID)
		if s.runtimeCache.CompareAndSwap(current, next) {
			return
		}
	}
}

func (s *SummaryService) resumeRuntimeCache(chatID string) {
	for {
		current := s.runtimeCache.Load()
		if current == nil {
			return
		}
		next := cloneSummaryRuntimeCache(current)
		next.chatGenerations[chatID]++
		delete(next.suspended, chatID)
		delete(next.contexts, chatID)
		delete(next.warnings, chatID)
		if s.runtimeCache.CompareAndSwap(current, next) {
			return
		}
	}
}

func (s *SummaryService) clearRuntimeCache() {
	for {
		current := s.runtimeCache.Load()
		epoch := uint64(1)
		if current != nil {
			epoch = current.epoch + 1
		}
		if s.runtimeCache.CompareAndSwap(current, newSummaryRuntimeCache(epoch)) {
			return
		}
	}
}

func (s *SummaryService) refreshRuntimeCache(chatID string) error {
	if !s.isEnabled() {
		return nil
	}
	version := s.currentCacheVersion(chatID)
	state, err := s.reconcileSummaryState(chatID)
	if err != nil {
		return err
	}
	bigSummary, smallSummaries, coverageTo, err := s.resolveUsableSummaryCoverage(chatID, state.AppliedCutoffSeq)
	if err != nil {
		return err
	}
	count, err := s.messageStore.CountAfterSeq(chatID, coverageTo)
	if err != nil {
		return err
	}
	s.publishMaterializedCache(chatID, version, bigSummary, smallSummaries, coverageTo, count)
	return nil
}

func (s *SummaryService) refreshRuntimeCacheAsync(chatID string) {
	go func() {
		if err := s.refreshRuntimeCache(chatID); err != nil {
			log.Printf("[summary] 后台刷新内存快照失败 chat=%s: %v", chatID, err)
		}
	}()
}

func (s *SummaryService) warmRuntimeCache() error {
	chatIDs, err := s.summaryStore.ListChatIDs()
	if err != nil {
		return err
	}
	for _, chatID := range chatIDs {
		if err := s.refreshRuntimeCache(chatID); err != nil {
			log.Printf("[summary] 跳过无法加载的摘要快照 chat=%s: %v", chatID, err)
		}
	}
	return nil
}

func summaryWarning(count, coverageTo int) string {
	if count <= summaryWarningCount {
		return ""
	}
	if coverageTo == 0 {
		return fmt.Sprintf("当前对话已有 %d 条消息尚未成功摘要，系统暂时仍发送完整历史，token 会继续增加；摘要将在后续消息中自动重试。", count)
	}
	return fmt.Sprintf("当前对话有 %d 条消息未被最新摘要覆盖，系统仍沿用上一份摘要和后续原文；摘要将在后续消息中自动重试。", count)
}

func countMessagesAfterSeq(messages []*model.Message, coverageTo int) int {
	count := 0
	for _, message := range messages {
		if message != nil && message.Seq > coverageTo {
			count++
		}
	}
	return count
}

func latestMessageSeq(messages []*model.Message) int {
	latest := 0
	for _, message := range messages {
		if message != nil && message.Seq > latest {
			latest = message.Seq
		}
	}
	return latest
}

func (s *SummaryService) resolveUsableSummaryCoverage(chatID string, maxToSeq int) (*model.ChatSummaryChunk, []*model.ChatSummaryChunk, int, error) {
	if maxToSeq <= 0 {
		return nil, nil, 0, nil
	}

	bigChunk, err := s.summaryStore.GetLatestUsableBigChunk(chatID, maxToSeq)
	if err != nil {
		return nil, nil, 0, err
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
			if chunk.FromSeq > 1 {
				return nil, nil, 0, nil
			}
			usableSmalls = append(usableSmalls, chunk)
			coverageTo = chunk.ToSeq
		case chunk.ToSeq <= coverageTo:
			continue
		case chunk.FromSeq > coverageTo+1:
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

func (s *SummaryService) reconcileSummaryState(chatID string) (*model.ChatSummaryState, error) {
	state, err := s.summaryStore.GetState(chatID)
	if err != nil {
		return nil, err
	}
	if state.AppliedCutoffSeq <= 0 {
		return state, nil
	}

	_, _, coverageTo, err := s.resolveUsableSummaryCoverage(chatID, state.AppliedCutoffSeq)
	if err != nil {
		return nil, err
	}
	if coverageTo == state.AppliedCutoffSeq {
		return state, nil
	}

	repairFrom := coverageTo + 1
	if err := s.summaryStore.MarkChunksDirtyFromSeq(chatID, repairFrom); err != nil {
		return nil, err
	}
	if err := s.summaryStore.ResetCurrentBigSummaryIfDirty(chatID); err != nil {
		return nil, err
	}
	if err := s.summaryStore.RollbackCutoff(chatID, coverageTo, repairFrom); err != nil {
		return nil, err
	}
	if err := s.summaryStore.ResetPendingSummaryFromSeq(chatID, repairFrom); err != nil {
		return nil, err
	}

	log.Printf("[summary] 已修复摘要断层 chat=%s old_cutoff=%d coverage=%d rebuild_from=%d",
		chatID, state.AppliedCutoffSeq, coverageTo, repairFrom)
	return s.summaryStore.GetState(chatID)
}

func (s *SummaryService) InvalidateFromSeq(chatID string, fromSeq int) error {
	if fromSeq <= 0 {
		return nil
	}
	s.evictRuntimeCache(chatID)
	err := s.summaryStore.InvalidateSummariesFromSeq(chatID, fromSeq, s.summaryCharLimit())
	s.refreshRuntimeCacheAsync(chatID)
	return err
}

func (s *SummaryService) DeleteMessageAndRecalculate(chatID, messageID string, cascade bool) (int64, error) {
	s.evictRuntimeCache(chatID)
	deleted, err := s.summaryStore.DeleteMessageAndRecalculate(chatID, messageID, cascade, s.summaryCharLimit())
	s.refreshRuntimeCacheAsync(chatID)
	return deleted, err
}

// DeleteMessageForRegeneration keeps summary cleanup completely outside the
// regeneration path. Until cleanup succeeds, this chat is quarantined to full
// history so an old or concurrently completed summary cannot leak back in.
func (s *SummaryService) DeleteMessageForRegeneration(chatID, messageID string, fromSeq int) error {
	s.suspendRuntimeCache(chatID)
	if err := s.messageStore.DeleteByID(messageID); err != nil {
		s.resumeRuntimeCache(chatID)
		s.refreshRuntimeCacheAsync(chatID)
		return err
	}
	go s.finishRegenerationInvalidation(chatID, fromSeq)
	return nil
}

func (s *SummaryService) finishRegenerationInvalidation(chatID string, fromSeq int) {
	const maxAttempts = 6
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := s.summaryStore.InvalidateSummariesFromSeq(chatID, fromSeq, s.summaryCharLimit())
		if err == nil {
			s.resumeRuntimeCache(chatID)
			if err := s.refreshRuntimeCache(chatID); err != nil {
				log.Printf("[summary] 重生成后刷新内存快照失败 chat=%s: %v", chatID, err)
			}
			return
		}
		log.Printf("[summary] 重生成后的后台失效失败 chat=%s attempt=%d/%d: %v", chatID, attempt, maxAttempts, err)
		if attempt < maxAttempts {
			time.Sleep(time.Duration(attempt) * 100 * time.Millisecond)
		}
	}
	log.Printf("[summary] 重生成后的摘要保持隔离，后续聊天将继续使用完整历史 chat=%s", chatID)
}

// ForgetChat invalidates all in-memory summary data without touching the database.
func (s *SummaryService) ForgetChat(chatID string) {
	s.evictRuntimeCache(chatID)
}

func (s *SummaryService) DeleteChatDataAsync(chatID string) {
	s.evictRuntimeCache(chatID)
	go func() {
		if err := s.summaryStore.DeleteChat(chatID); err != nil {
			log.Printf("[summary] 后台删除对话摘要失败 chat=%s: %v", chatID, err)
		}
	}()
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
	return s.enabled.Load()
}

// SetEnabled updates the chat-path flag atomically. Cache loading remains background work.
func (s *SummaryService) SetEnabled(enabled bool) {
	if s.enabled.Swap(enabled) == enabled {
		return
	}
	s.clearRuntimeCache()
	if enabled {
		go func() {
			if err := s.warmRuntimeCache(); err != nil {
				log.Printf("[summary] 启用后加载摘要快照失败: %v", err)
			}
		}()
	}
}

func (s *SummaryService) summaryCharLimit() int {
	settings, err := s.configStore.GetSettings()
	if err != nil || settings.MemorySummaryCharLimit <= 0 {
		return defaultSummaryCharLimit
	}
	return settings.MemorySummaryCharLimit
}

func buildRollingSummaryPrompt(
	character *model.Character,
	firstSummary bool,
	activeBig *model.ChatSummaryChunk,
	activeSmalls []*model.ChatSummaryChunk,
	rawMessages []*model.Message,
	fromSeq int,
	toSeq int,
	suffix string,
) string {
	var builder strings.Builder
	builder.WriteString("任务类型：滚动总摘要。\n")
	builder.WriteString("请把已有记忆与新增会话合并成一份完整、去重、可独立使用的结构化摘要。输出必须覆盖已有记忆中的有效事实，不能只总结新增片段。\n")
	builder.WriteString("角色卡区块只用于理解人物、称呼、视角和设定边界，不是待摘要内容；不要在结果中复述静态角色卡资料。\n")
	builder.WriteString("状态栏属于独立展示数据，不是聊天记录；如果输入中意外出现状态栏，必须完全忽略。\n\n")

	renderCharacterReference(&builder, character, firstSummary)

	if activeBig != nil || len(activeSmalls) > 0 {
		builder.WriteString(fmt.Sprintf("[上一条摘要｜截至消息序号 %d]\n", fromSeq-1))
		if activeBig != nil {
			builder.WriteString(activeBig.Content)
			builder.WriteString("\n")
		}
		for _, chunk := range activeSmalls {
			builder.WriteString(chunk.Content)
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}

	if firstSummary {
		builder.WriteString(fmt.Sprintf("[聊天记录｜消息序号 %d-%d]\n", fromSeq, toSeq))
	} else {
		builder.WriteString(fmt.Sprintf("[上一条摘要之后的聊天记录｜消息序号 %d-%d]\n", fromSeq, toSeq))
	}
	builder.WriteString(renderMessagesForSummary(rawMessages))
	builder.WriteString("\n\n合并要求：\n")
	builder.WriteString("- 删除已经被后续对话推翻或替代的旧状态，保留仍然有效的事实。\n")
	builder.WriteString("- 合并重复信息，不要因压缩而丢失人物关系、用户偏好、承诺、物品、位置和未完成事项。\n")
	builder.WriteString("- 如果上一条摘要遗留了静态角色卡复述或状态栏字段，在本次结果中删除这些内容。\n")
	builder.WriteString("- 只依据给定内容，不补写未发生的情节。\n")
	appendMemoryPromptSuffix(&builder, suffix)
	return builder.String()
}

func renderCharacterReference(builder *strings.Builder, character *model.Character, includeScenarioAndOpening bool) {
	if character == nil {
		return
	}
	if includeScenarioAndOpening {
		builder.WriteString("[角色卡参考资料｜首次摘要完整字段｜禁止复制到摘要]\n")
	} else {
		builder.WriteString("[角色卡参考资料｜后续摘要字段｜禁止复制到摘要]\n")
	}
	writeCharacterReferenceField(builder, "角色名", character.Name)
	writeCharacterReferenceField(builder, "角色描述", character.Description)
	writeCharacterReferenceField(builder, "性格", character.Personality)
	if includeScenarioAndOpening {
		writeCharacterReferenceField(builder, "场景设定", character.Scenario)
		writeCharacterReferenceField(builder, "开场白", character.FirstMsg)
	}
	writeCharacterReferenceField(builder, "头像", character.AvatarURL)
	writeCharacterReferenceField(builder, "标签", character.Tags)
	writeCharacterReferenceField(builder, "叙事视角", character.POV)
	writeCharacterReferenceField(builder, "使用自定义用户设定", fmt.Sprintf("%t", character.UseCustomUser))
	writeCharacterReferenceField(builder, "用户名称", character.UserName)
	writeCharacterReferenceField(builder, "用户设定", character.UserDetail)
	builder.WriteString("\n")
}

func writeCharacterReferenceField(builder *strings.Builder, label, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "（空）"
	}
	builder.WriteString(label)
	builder.WriteString("：")
	builder.WriteString(value)
	builder.WriteString("\n")
}

func renderMessagesForSummary(messages []*model.Message) string {
	var builder strings.Builder
	for _, msg := range messages {
		content := strings.TrimSpace(msg.Content)
		if msg.Role == "assistant" {
			content, _ = statusbar.Split(content)
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
		Plot:         strings.TrimSpace(extractTaggedContent(cleaned, "plot")),
		Relationship: strings.TrimSpace(extractTaggedContent(cleaned, "relationship")),
		UserFacts:    strings.TrimSpace(extractTaggedContent(cleaned, "user_facts")),
		WorldState:   strings.TrimSpace(extractTaggedContent(cleaned, "world_state")),
		OpenLoops:    strings.TrimSpace(extractTaggedContent(cleaned, "open_loops")),
	}
	if summary.Plot == "" && summary.Relationship == "" && summary.UserFacts == "" && summary.WorldState == "" && summary.OpenLoops == "" {
		// 部分兼容接口不会严格遵循 XML；保留其完整摘要，避免格式问题让压缩链路永久停摆。
		summary.Plot = cleaned
	}
	summary.Plot = normalizeSummaryField(summary.Plot)
	summary.Relationship = normalizeSummaryField(summary.Relationship)
	summary.UserFacts = normalizeSummaryField(summary.UserFacts)
	summary.WorldState = normalizeSummaryField(summary.WorldState)
	summary.OpenLoops = normalizeSummaryField(summary.OpenLoops)

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

func appendMemoryPromptSuffix(builder *strings.Builder, suffix string) {
	if defaultSuffix := strings.TrimSpace(defaultMemoryPromptSuffix); defaultSuffix != "" {
		builder.WriteString(defaultSuffix)
		builder.WriteString("\n")
	}
	if adminSuffix := strings.TrimSpace(suffix); adminSuffix != "" {
		builder.WriteString("- 管理员补充要求：\n")
		builder.WriteString(adminSuffix)
		builder.WriteString("\n")
	}
}

func countEffectiveChars(messages []*model.Message) int {
	total := 0
	for _, msg := range messages {
		total += effectiveMessageChars(msg)
	}
	return total
}

func effectiveMessageChars(msg *model.Message) int {
	if msg == nil {
		return 0
	}
	content := strings.TrimSpace(msg.Content)
	if msg.Role == "assistant" {
		content, _ = statusbar.Split(content)
		content = cleanAssistantContent(content)
	}
	return len([]rune(content))
}

func selectSummaryEnd(messages []*model.Message) int {
	if len(messages) == 0 {
		return 0
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" {
			return messages[i].Seq
		}
	}
	return 0
}
