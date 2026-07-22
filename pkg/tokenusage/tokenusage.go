// Copyright (c) 2022 Alibaba Group Holding Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tokenusage

import (
	"bytes"
	"slices"

	"github.com/higress-group/wasm-go/pkg/wrapper"
)

const (
	CtxKeyInputToken         = "input_token"
	CtxKeyInputTokenDetails  = "input_token_details"
	CtxKeyOutputToken        = "output_token"
	CtxKeyOutputTokenDetails = "output_token_details"
	CtxKeyTotalToken         = "total_token"
	CtxKeyModel              = "model"
	CtxKeyRequestModel       = "request_model"
	CtxKeyChatId             = "chat_id"

	ModelEmpty   = ""
	ModelUnknown = "unknown"

	ChatIdPathOpenAIChatCompletions = "id"
	ChatIdPathOpenAIResponses       = "response.id"
	ChatIdPathGemini                = "responseId"
	ChatIdPathAnthropicMessages     = "message.id"

	ModelPathOpenAIChatCompletions = "model"
	ModelPathOpenAIBatches         = "body.model"
	ModelPathOpenAIResponses       = "response.model"
	ModelPathAnthropicMessages     = "message.model"
	ModelPathGeminiGenerateContent = "modelVersion"

	UsageInputTokensPathOpenAIChatCompletions = "usage.prompt_tokens"
	UsageInputTokensPathOpenAIImages          = "usage.input_tokens"
	UsageInputTokensPathOpenAIResponses       = "response.usage.input_tokens"
	UsageInputTokensPathGemini                = "usageMetadata.promptTokenCount"
	UsageInputTokensPathAnthropicMessages     = "message.usage.input_tokens"

	UsageCacheCreationInputTokensPathAnthropicMessages = "usage.cache_creation_input_tokens"
	UsageCacheReadInputTokensPathAnthropicMessages     = "usage.cache_read_input_tokens"

	UsageInputTokensDetailsPathOpenAIChatCompletions = "usage.prompt_tokens_details"
	UsageInputTokensDetailsPathOpenAIResponses       = "response.usage.input_tokens_details"
	UsageInputTokensDetailsPathDoubao                = "usage.input_tokens_details"
	UsageInputTokensDetailsPathGemini                = "usageMetadata.promptTokensDetails"

	// DeepSeek exposes cache-read hits at the top level of usage (not inside prompt_tokens_details);
	// some gateways instead surface it under input_token_details, so both are tried (top-level wins).
	UsagePromptCacheHitTokensPathDeepSeek             = "usage.prompt_cache_hit_tokens"
	UsagePromptCacheHitTokensPathDeepSeekInputDetails = "usage.prompt_tokens_details.prompt_cache_hit_tokens"

	UsageOutputTokensPathOpenAIChatCompletions = "usage.completion_tokens"
	UsageOutputTokensPathOpenAIImages          = "usage.output_tokens"
	UsageOutputTokensPathOpenAIResponses       = "response.usage.output_tokens"
	UsageOutputTokensPathGemini                = "usageMetadata.candidatesTokenCount"
	UsageOutputTokensPathAnthropicMessages     = "message.usage.output_tokens"

	UsageMetadataThoughtsTokenCountPathGemini      = "usageMetadata.thoughtsTokenCount"
	UsageMetadataCachedContentTokenCountPathGemini = "usageMetadata.cachedContentTokenCount"
	UsageMetadataToolUsePromptTokenCountPathGemini = "usageMetadata.toolUsePromptTokenCount"
	UsageGeneratedImagesPathDoubao                 = "usage.generated_images"

	UsageOutputTokensDetailsPathOpenAIChatCompletions = "usage.completion_tokens_details"
	UsageOutputTokensDetailsPathOpenAIResponses       = "response.usage.output_tokens_details"
	UsageOutputTokensDetailsPathDoubao                = "usage.output_tokens_details"
	UsageOutputTokensDetailsPathGemini                = "usageMetadata.candidatesTokensDetails"

	UsageTotalTokensPathOpenAIChatCompletions = "usage.total_tokens"
	UsageTotalTokensPathOpenAIResponses       = "response.usage.total_tokens"
	UsageTotalTokensPathGemini                = "usageMetadata.totalTokenCount"

	InputTokenDetailsKeyAnthropicMessagesUsageCacheCreationInputTokens = "cache_creation_input_tokens"
	InputTokenDetailsKeyAnthropicMessagesUsageCacheReadInputTokens     = "cache_read_input_tokens"
	InputTokenDetailsKeyCachedTokens                                   = "cached_tokens"
	InputTokenDetailsKeyOpenAICacheWriteTokens                         = "cache_write_tokens"
	InputTokenDetailsKeyDeepSeekPromptCacheHitTokens                   = "prompt_cache_hit_tokens"
	InputTokenDetailsKeyGeminiCachedContentTokenCount                  = "cached_content_token_count"
	InputTokenDetailsKeyGeminiToolUsePromptTokenCount                  = "tool_use_prompt_token_count"

	OutputTokenDetailsKeyDoubaoGeneratedImages    = "generated_images"
	OutputTokenDetailsKeyGeminiThoughtsTokenCount = "thoughts_token_count"

	ctxKeyDeltaSSEMessage = "delta_sse_message"
	ctxKeyDeltaBeginning  = "delta_beginning"
)

type TokenUsage struct {
	// InputToken is the NET pure-text input: the provider's prompt tokens with the inclusive-family
	// cache (OpenAI cached_tokens/cache_write_tokens, Gemini cached_content, DeepSeek prompt_cache_hit)
	// subtracted out, clamped at 0. Anthropic cache is exclusive (never included in the provider's
	// input) so it is not subtracted. Cache tokens are reported separately via the fields below, so
	// consumers can bill each bucket at its own rate without re-deriving the split.
	InputToken         int64
	InputTokenDetails  map[string]int64
	OutputTokenDetails map[string]int64
	OutputToken        int64
	TotalToken         int64
	Model              string

	// CacheReadInputToken is the unified cache-read bucket across all providers: OpenAI cached_tokens
	// (also covers Bailian explicit/implicit cache-read, both reported via cached_tokens) + Anthropic
	// cache_read + Gemini cached_content + DeepSeek prompt_cache_hit.
	CacheReadInputToken int64
	// CacheWriteInputToken is the unified cache-write bucket:
	// OpenAI cache_write_tokens + Bailian explicit cache-creation + Anthropic cache_creation.
	CacheWriteInputToken int64

	// Per-provider cache fields below carry the raw values before aggregation/netting. They remain
	// available for logging and for consumers that need the per-provider breakdown.

	// CachedInputToken is the OpenAI cache-read bucket (prompt_tokens_details.cached_tokens).
	CachedInputToken int64

	// OpenAICacheWriteInputToken is OpenAI GPT-5.6 cache write (cache_write_tokens). Folded into
	// CacheWriteInputToken alongside the Anthropic cache-creation field.
	OpenAICacheWriteInputToken int64

	// DeepSeekPromptCacheHitToken is DeepSeek cache-read (top-level usage.prompt_cache_hit_tokens;
	// DeepSeek does not emit cached_tokens). Folded into CacheReadInputToken.
	DeepSeekPromptCacheHitToken int64

	// GeminiCachedContentToken is Gemini cache-read (usageMetadata.cachedContentTokenCount).
	// Folded into CacheReadInputToken.
	GeminiCachedContentToken int64

	// BailianCacheCreationInputToken is Bailian (阿里云百炼) OpenAI/DashScope-compat EXPLICIT
	// cache-creation (prompt_tokens_details.cache_creation_input_tokens, 125% rate). The cache-read
	// hit that may accompany it lands in cached_tokens/CachedInputToken like any other OpenAI-family
	// cache-read — explicit vs implicit billing tiers are not distinguished here; that is left to the
	// consumer (it needs the request's cache_control, which this response-only extractor never sees).
	// Inclusive (part of prompt_tokens), so netted out of InputToken; folds into CacheWriteInputToken.
	BailianCacheCreationInputToken int64

	// Anthropic Messages (exclusive family: not part of the provider's input_tokens).
	AnthropicCacheCreationInputToken int64
	AnthropicCacheReadInputToken     int64
}

// normalizeCacheBuckets aggregates the per-provider cache fields into CacheReadInputToken /
// CacheWriteInputToken and subtracts the inclusive-family cache from InputToken so InputToken
// becomes the net pure-text input. Idempotency across the streaming chunk loop is guaranteed by
// deriving InputToken from the raw prompt tokens on every chunk before this runs once at the end.
//
// Inclusive family (OpenAI cached_tokens/cache_write_tokens, Bailian explicit cache-creation,
// Gemini cached_content, DeepSeek hit) lives inside the provider's prompt tokens and is subtracted.
// Anthropic cache is exclusive — the provider's input_tokens never included it — so it is NOT
// subtracted, avoiding an under-count.
func (u *TokenUsage) normalizeCacheBuckets() {
	u.CacheReadInputToken = u.CachedInputToken +
		u.AnthropicCacheReadInputToken +
		u.GeminiCachedContentToken +
		u.DeepSeekPromptCacheHitToken
	u.CacheWriteInputToken = u.OpenAICacheWriteInputToken +
		u.BailianCacheCreationInputToken +
		u.AnthropicCacheCreationInputToken

	inclusiveCache := u.CachedInputToken +
		u.OpenAICacheWriteInputToken +
		u.BailianCacheCreationInputToken +
		u.GeminiCachedContentToken +
		u.DeepSeekPromptCacheHitToken
	u.InputToken -= inclusiveCache
	if u.InputToken < 0 {
		u.InputToken = 0
	}
}

func GetTokenUsage(ctx wrapper.HttpContext, body []byte) TokenUsage {
	chunks := bytes.SplitSeq(wrapper.UnifySSEChunk(body), []byte("\n\n"))
	u := TokenUsage{
		InputTokenDetails:  make(map[string]int64),
		OutputTokenDetails: make(map[string]int64),
	}
	for chunk := range chunks {
		// the feature strings are used to identify the usage data, like:
		// {"model":"gpt2","usage":{"prompt_tokens":1,"completion_tokens":1}}

		// openai/v1/responses
		chunk = mergeLargeResponseAPIChunks(ctx, chunk)

		if !bytes.Contains(chunk, []byte(`"usage"`)) && !bytes.Contains(chunk, []byte(`"usageMetadata"`)) {
			continue
		}

		ExtractModel(ctx, chunk, &u)
		ExtractInputTokens(ctx, chunk, &u)
		ExtractOutputTokens(ctx, chunk, &u)
		ExtractInputTokenDetails(ctx, chunk, &u)
		ExtractOutputTokenDetails(ctx, chunk, &u)
		ExtractTotalTokens(ctx, chunk, &u)
	}
	// Aggregate cache buckets and net the inclusive-family cache out of InputToken. Done once here,
	// after the chunk loop, because ExtractInputTokens re-reads the raw prompt tokens per chunk and
	// ExtractTotalTokens' fallback relies on the raw (pre-net) input to reconstruct the true total.
	u.normalizeCacheBuckets()
	// Republish the NET input on the public attribute so consumers reading CtxKeyInputToken (e.g.
	// ai-statistics metrics) see the same net value as the returned struct.
	ctx.SetUserAttribute(CtxKeyInputToken, u.InputToken)
	return u
}

func mergeLargeResponseAPIChunks(ctx wrapper.HttpContext, chunk []byte) []byte {
	if bytes.Contains(chunk, []byte(`"response.completed"`)) && !bytes.Contains(chunk, []byte(`"usage"`)) {
		ctx.SetContext(ctxKeyDeltaBeginning, true)
	}

	if ctx.GetBoolContext(ctxKeyDeltaBeginning, false) {
		// end of streaming
		if len(bytes.TrimSpace(chunk)) == 0 {
			ctx.SetContext(ctxKeyDeltaBeginning, false)
			chunk = ctx.GetByteSliceContext(ctxKeyDeltaSSEMessage, chunk)
			ctx.SetContext(ctxKeyDeltaSSEMessage, nil)
		} else {
			deltaMessage := ctx.GetByteSliceContext(ctxKeyDeltaSSEMessage, []byte{})
			deltaMessage = append(deltaMessage, chunk...)
			ctx.SetContext(ctxKeyDeltaSSEMessage, deltaMessage)
		}
	}

	return chunk
}

func ExtractModel(ctx wrapper.HttpContext, body []byte, u *TokenUsage) {
	if model := wrapper.GetValueFromBody(body, []string{
		ModelPathOpenAIChatCompletions,
		ModelPathOpenAIBatches,         // batches
		ModelPathOpenAIResponses,       // responses
		ModelPathAnthropicMessages,     // anthropic messages
		ModelPathGeminiGenerateContent, // Gemini GenerateContent
	}); model != nil {
		u.Model = model.String()
	} else if model, ok := ctx.GetUserAttribute(CtxKeyModel).(string); ok && !slices.Contains([]string{ModelEmpty, ModelUnknown}, model) { // anthropic messages
		u.Model = model
	} else if model := ctx.GetStringContext(CtxKeyRequestModel, ModelEmpty); model != ModelEmpty { // Openai Image Generate
		u.Model = model
	} else {
		u.Model = ModelUnknown
	}
	ctx.SetUserAttribute(CtxKeyModel, u.Model)
}

func ExtractInputTokens(ctx wrapper.HttpContext, body []byte, u *TokenUsage) {
	if inputToken := wrapper.GetValueFromBody(body, []string{
		UsageInputTokensPathOpenAIChatCompletions, // completions , chatcompleations
		UsageInputTokensPathOpenAIImages,          // images, audio
		UsageInputTokensPathOpenAIResponses,       // responses
		UsageInputTokensPathGemini,                // Gemini GenerateContent
		UsageInputTokensPathAnthropicMessages,     // Anthrophic messages
	}); inputToken != nil {
		u.InputToken = inputToken.Int()
	} else {
		inputToken, ok := ctx.GetUserAttribute(CtxKeyInputToken).(int64) // anthropic messages
		if ok && inputToken > 0 {
			u.InputToken = inputToken
		}
	}
	// Within a single GetTokenUsage call the attribute stays raw (netting runs once, post-loop).
	// GetTokenUsage republishes the NET value on this attribute at the end for consumers.
	ctx.SetUserAttribute(CtxKeyInputToken, u.InputToken)
}

func ExtractOutputTokens(ctx wrapper.HttpContext, body []byte, u *TokenUsage) {
	if outputToken := wrapper.GetValueFromBody(body, []string{
		UsageOutputTokensPathOpenAIChatCompletions, // completions , chatcompleations
		UsageOutputTokensPathOpenAIImages,          // images, audio
		UsageOutputTokensPathOpenAIResponses,       // responses
		UsageOutputTokensPathGemini,                // Gemini GeneratenContent
		UsageOutputTokensPathAnthropicMessages,     // Anthropic messages
	}); outputToken != nil {
		u.OutputToken = outputToken.Int()
	} else {
		outputToken, ok := ctx.GetUserAttribute(CtxKeyOutputToken).(int64)
		if ok && outputToken > 0 {
			u.OutputToken = outputToken
		}
	}
	ctx.SetUserAttribute(CtxKeyOutputToken, u.OutputToken)
}

func ExtractInputTokenDetails(ctx wrapper.HttpContext, body []byte, u *TokenUsage) {
	if inputTokenDetails := wrapper.GetValueFromBody(body, []string{
		UsageInputTokensDetailsPathOpenAIChatCompletions, // chatcompletions
		UsageInputTokensDetailsPathOpenAIResponses,       // responses
		UsageInputTokensDetailsPathDoubao,                // Doubao
		UsageInputTokensDetailsPathGemini,                // Gemini GenerateContent
	}); inputTokenDetails != nil && inputTokenDetails.IsObject() {
		for key, value := range inputTokenDetails.Map() {
			u.InputTokenDetails[key] = value.Int()
		}
	}

	// Gemini GenerateContent
	if geminiCachedContentTokenCount := wrapper.GetValueFromBody(body, []string{
		UsageMetadataCachedContentTokenCountPathGemini,
	}); geminiCachedContentTokenCount != nil {
		u.GeminiCachedContentToken = geminiCachedContentTokenCount.Int()
		u.InputTokenDetails[InputTokenDetailsKeyGeminiCachedContentTokenCount] = geminiCachedContentTokenCount.Int()
	}
	if geminiToolUsePromptTokenCount := wrapper.GetValueFromBody(body, []string{
		UsageMetadataToolUsePromptTokenCountPathGemini,
	}); geminiToolUsePromptTokenCount != nil {
		u.InputTokenDetails[InputTokenDetailsKeyGeminiToolUsePromptTokenCount] = geminiToolUsePromptTokenCount.Int()
	}

	// Anthropic Messages
	if cacheCreationInputToken := wrapper.GetValueFromBody(body, []string{
		UsageCacheCreationInputTokensPathAnthropicMessages,
	}); cacheCreationInputToken != nil {
		u.AnthropicCacheCreationInputToken = cacheCreationInputToken.Int()
		u.InputTokenDetails[InputTokenDetailsKeyAnthropicMessagesUsageCacheCreationInputTokens] = cacheCreationInputToken.Int()
	}
	if cacheReadInputToken := wrapper.GetValueFromBody(body, []string{
		UsageCacheReadInputTokensPathAnthropicMessages,
	}); cacheReadInputToken != nil {
		u.AnthropicCacheReadInputToken = cacheReadInputToken.Int()
		u.InputTokenDetails[InputTokenDetailsKeyAnthropicMessagesUsageCacheReadInputTokens] = cacheReadInputToken.Int()
	}

	// OpenAI-family cache-read lives in prompt_tokens_details.cached_tokens (inclusive; netted out of
	// InputToken in normalizeCacheBuckets). Bailian (阿里云百炼) OpenAI/DashScope-compat reports both its
	// explicit and implicit cache-read hits through this same cached_tokens field — they bill at
	// different rates (explicit 10% vs implicit 20%), but that distinction requires knowing whether the
	// request carried a cache_control marker, which this response-only extractor never sees. So the hit
	// is not re-keyed or split here; it is kept as CachedInputToken/cached_tokens like any other OpenAI
	// cache-read, and the explicit/implicit billing split is left entirely to the consumer.
	cachedToken, hasCached := u.InputTokenDetails[InputTokenDetailsKeyCachedTokens]
	if hasCached {
		u.CachedInputToken = cachedToken
	}

	// cache_creation_input_tokens is ambiguous on its own: Bailian's explicit tier reports it inside
	// prompt_tokens_details alongside cached_tokens, but the same map key is also populated above
	// (line ~330) from Anthropic's top-level usage.cache_creation_input_tokens — a completely
	// different field, already captured in AnthropicCacheCreationInputToken. Gating on hasCached is
	// what tells these two sources apart: only prompt_tokens_details ever has BOTH cached_tokens and
	// cache_creation_input_tokens together, so hasCached && hasCacheCreation is required to route the
	// value into BailianCacheCreationInputToken. Without that gate, a plain Anthropic Messages response
	// (cache_read/cache_creation only, no cached_tokens) would double-count its cache_creation into
	// both AnthropicCacheCreationInputToken and BailianCacheCreationInputToken.
	cacheCreationToken, hasCacheCreation := u.InputTokenDetails[InputTokenDetailsKeyAnthropicMessagesUsageCacheCreationInputTokens]
	if hasCached && hasCacheCreation {
		u.BailianCacheCreationInputToken = cacheCreationToken
	} else {
		// Cleared (not just left at zero-value) so a streaming response whose earlier chunk was
		// Bailian-explicit-shaped and whose final usage chunk drops the cached_tokens/cache_creation
		// pairing doesn't leave a stale creation count behind to double-count in normalizeCacheBuckets.
		u.BailianCacheCreationInputToken = 0
	}

	// OpenAI GPT-5.6: cache_write_tokens 记入独立字段,与 Anthropic cache-creation 保持分离
	// (风格对齐 cache-read:OpenAI CachedInputToken 与 Anthropic AnthropicCacheReadInputToken 分开存,
	// 由 ai-quota / ai-statistics 消费侧相加)。不改动 InputTokenDetails map,cache_write_tokens 键原样保留。
	if cacheWriteToken, ok := u.InputTokenDetails[InputTokenDetailsKeyOpenAICacheWriteTokens]; ok {
		u.OpenAICacheWriteInputToken = cacheWriteToken
	}

	// DeepSeek: cache-read 命中在顶层 usage.prompt_cache_hit_tokens(不吐 cached_tokens);
	// 部分网关改挂在 input_token_details 下,故两处都试,取非零者(顶层优先),避免重复计。
	if promptCacheHit := wrapper.GetValueFromBody(body, []string{
		UsagePromptCacheHitTokensPathDeepSeek,
		UsagePromptCacheHitTokensPathDeepSeekInputDetails,
	}); promptCacheHit != nil && promptCacheHit.Int() != 0 {
		u.DeepSeekPromptCacheHitToken = promptCacheHit.Int()
		u.InputTokenDetails[InputTokenDetailsKeyDeepSeekPromptCacheHitTokens] = promptCacheHit.Int()
	}
	ctx.SetUserAttribute(CtxKeyInputTokenDetails, u.InputTokenDetails)
}

func ExtractOutputTokenDetails(ctx wrapper.HttpContext, body []byte, u *TokenUsage) {
	if outputTokensDetails := wrapper.GetValueFromBody(body, []string{
		UsageOutputTokensDetailsPathOpenAIChatCompletions, // completions , chatcompleations
		UsageOutputTokensDetailsPathOpenAIResponses,       // responses
		UsageOutputTokensDetailsPathDoubao,                // doubao
		UsageOutputTokensDetailsPathGemini,                // Gemini GenerateContent
	}); outputTokensDetails != nil && outputTokensDetails.IsObject() {
		for key, val := range outputTokensDetails.Map() {
			u.OutputTokenDetails[key] = val.Int()
		}
	}
	// Gemini GenerateContent
	if geminiThoughtsTokenCount := wrapper.GetValueFromBody(body, []string{
		UsageMetadataThoughtsTokenCountPathGemini,
	}); geminiThoughtsTokenCount != nil {
		u.OutputTokenDetails[OutputTokenDetailsKeyGeminiThoughtsTokenCount] = geminiThoughtsTokenCount.Int()
	}
	// Doubao Image Generate
	if doubaoGeneratedImages := wrapper.GetValueFromBody(body, []string{
		UsageGeneratedImagesPathDoubao,
	}); doubaoGeneratedImages != nil {
		u.OutputTokenDetails[OutputTokenDetailsKeyDoubaoGeneratedImages] = doubaoGeneratedImages.Int()
	}
	ctx.SetUserAttribute(CtxKeyOutputTokenDetails, u.OutputTokenDetails)
}

func ExtractTotalTokens(ctx wrapper.HttpContext, body []byte, u *TokenUsage) {
	if totalToken := wrapper.GetValueFromBody(body, []string{
		UsageTotalTokensPathOpenAIChatCompletions, // completions , chatcompleations, images, audio, responses
		UsageTotalTokensPathOpenAIResponses,       // responses
		UsageTotalTokensPathGemini,                // Gemini GenerationContent
	}); totalToken != nil {
		u.TotalToken = totalToken.Int()
	} else {
		u.TotalToken = u.InputToken + u.OutputToken + u.AnthropicCacheCreationInputToken + u.AnthropicCacheReadInputToken
	}
	ctx.SetUserAttribute(CtxKeyTotalToken, u.TotalToken)
}

func ExtractChatId(ctx wrapper.HttpContext, body []byte) {
	if chatID := wrapper.GetValueFromBody(body, []string{
		ChatIdPathOpenAIChatCompletions,
		ChatIdPathOpenAIResponses,
		ChatIdPathGemini,            // Gemini generateContent
		ChatIdPathAnthropicMessages, // anthropic messages
	}); chatID != nil {
		ctx.SetUserAttribute(CtxKeyChatId, chatID.String())
	}
}
