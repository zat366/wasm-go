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
	"testing"

	"github.com/higress-group/wasm-go/pkg/iface"
)

type testHttpContext struct {
	context       map[string]interface{}
	userAttribute map[string]interface{}
	bufferQueue   [][]byte
}

func newTestHttpContext() *testHttpContext {
	return &testHttpContext{
		context:       make(map[string]interface{}),
		userAttribute: make(map[string]interface{}),
	}
}

func (ctx *testHttpContext) Scheme() string { return "" }
func (ctx *testHttpContext) Host() string   { return "" }
func (ctx *testHttpContext) Path() string   { return "" }
func (ctx *testHttpContext) Method() string { return "" }

func (ctx *testHttpContext) SetContext(key string, value interface{}) {
	ctx.context[key] = value
}

func (ctx *testHttpContext) GetContext(key string) interface{} {
	return ctx.context[key]
}

func (ctx *testHttpContext) GetBoolContext(key string, defaultValue bool) bool {
	value, ok := ctx.context[key].(bool)
	if !ok {
		return defaultValue
	}
	return value
}

func (ctx *testHttpContext) GetStringContext(key, defaultValue string) string {
	value, ok := ctx.context[key].(string)
	if !ok {
		return defaultValue
	}
	return value
}

func (ctx *testHttpContext) GetByteSliceContext(key string, defaultValue []byte) []byte {
	value, ok := ctx.context[key].([]byte)
	if !ok {
		return defaultValue
	}
	return value
}

func (ctx *testHttpContext) GetUserAttribute(key string) interface{} {
	return ctx.userAttribute[key]
}

func (ctx *testHttpContext) SetUserAttribute(key string, value interface{}) {
	ctx.userAttribute[key] = value
}

func (ctx *testHttpContext) SetUserAttributeMap(kvmap map[string]interface{}) {
	ctx.userAttribute = kvmap
}

func (ctx *testHttpContext) GetUserAttributeMap() map[string]interface{} {
	return ctx.userAttribute
}

func (ctx *testHttpContext) WriteUserAttributeToLog() error { return nil }
func (ctx *testHttpContext) WriteUserAttributeToLogWithKey(key string) error {
	return nil
}
func (ctx *testHttpContext) WriteUserAttributeToTrace() error { return nil }
func (ctx *testHttpContext) DontReadRequestBody()             {}
func (ctx *testHttpContext) DontReadResponseBody()            {}
func (ctx *testHttpContext) BufferRequestBody()               {}
func (ctx *testHttpContext) BufferResponseBody()              {}
func (ctx *testHttpContext) NeedPauseStreamingResponse()      {}

func (ctx *testHttpContext) PushBuffer(buffer []byte) {
	ctx.bufferQueue = append(ctx.bufferQueue, buffer)
}

func (ctx *testHttpContext) PopBuffer() []byte {
	if len(ctx.bufferQueue) == 0 {
		return nil
	}
	buffer := ctx.bufferQueue[0]
	ctx.bufferQueue = ctx.bufferQueue[1:]
	return buffer
}

func (ctx *testHttpContext) BufferQueueSize() int { return len(ctx.bufferQueue) }
func (ctx *testHttpContext) DisableReroute()      {}
func (ctx *testHttpContext) SetRequestBodyBufferLimit(byteSize uint32) {
}
func (ctx *testHttpContext) SetResponseBodyBufferLimit(byteSize uint32) {
}
func (ctx *testHttpContext) RouteCall(method, url string, headers [][2]string, body []byte, callback iface.RouteResponseCallback) error {
	return nil
}
func (ctx *testHttpContext) GetExecutionPhase() iface.HTTPExecutionPhase {
	return iface.Done
}
func (ctx *testHttpContext) HasRequestBody() bool       { return false }
func (ctx *testHttpContext) HasResponseBody() bool      { return false }
func (ctx *testHttpContext) IsWebsocket() bool          { return false }
func (ctx *testHttpContext) IsBinaryRequestBody() bool  { return false }
func (ctx *testHttpContext) IsBinaryResponseBody() bool { return false }

// InputToken is the NET pure-text input (raw prompt tokens minus inclusive cache). OpenAI
// cached_tokens is inclusive, so it is subtracted from input and surfaced via CacheReadInputToken.
func TestGetTokenUsageOpenAIChatCompletionsNetsCachedInputTokens(t *testing.T) {
	ctx := newTestHttpContext()
	body := []byte(`{
		"id": "chatcmpl-test",
		"model": "gpt-4.1-mini",
		"usage": {
			"prompt_tokens": 100,
			"completion_tokens": 25,
			"total_tokens": 125,
			"prompt_tokens_details": {
				"cached_tokens": 80
			}
		}
	}`)

	usage := GetTokenUsage(ctx, body)

	assertInt64(t, "net input token", 20, usage.InputToken) // 100 - cached(80)
	assertInt64(t, "cache read bucket", 80, usage.CacheReadInputToken)
	assertInt64(t, "cache write bucket", 0, usage.CacheWriteInputToken)
	assertInt64(t, "cached input token", 80, usage.CachedInputToken)
	assertInt64(t, "cached token detail", 80, usage.InputTokenDetails["cached_tokens"])
	assertInt64(t, "total token", 125, usage.TotalToken)
	// The public CtxKeyInputToken attribute is the NET value, matching the returned struct, so
	// consumers reading the attribute (e.g. ai-statistics metrics) bill net input.
	assertInt64(t, "net input token attribute", 20, ctx.GetUserAttribute(CtxKeyInputToken).(int64))
}

func TestGetTokenUsageOpenAIResponsesNetsCachedInputTokens(t *testing.T) {
	ctx := newTestHttpContext()
	body := []byte(`{
		"response": {
			"id": "resp_test",
			"model": "gpt-4.1-mini",
			"usage": {
				"input_tokens": 200,
				"output_tokens": 30,
				"total_tokens": 230,
				"input_tokens_details": {
					"cached_tokens": 160
				}
			}
		}
	}`)

	usage := GetTokenUsage(ctx, body)

	assertInt64(t, "net input token", 40, usage.InputToken) // 200 - cached(160)
	assertInt64(t, "cache read bucket", 160, usage.CacheReadInputToken)
	assertInt64(t, "cached input token", 160, usage.CachedInputToken)
	assertInt64(t, "cached token detail", 160, usage.InputTokenDetails["cached_tokens"])
	assertInt64(t, "total token", 230, usage.TotalToken)
}

// ExtractInputTokens returns the raw provider input verbatim and no longer nets out cache;
// the dedicated cache fields are populated by ExtractInputTokenDetails instead.
func TestExtractInputTokensReturnsRawInput(t *testing.T) {
	ctx := newTestHttpContext()
	body := []byte(`{
		"usage": {
			"prompt_tokens": 100,
			"prompt_tokens_details": {
				"cached_tokens": 80
			}
		}
	}`)
	usage := TokenUsage{}

	ExtractInputTokens(ctx, body, &usage)

	assertInt64(t, "input token", 100, usage.InputToken)
	assertInt64(t, "input token attribute", 100, ctx.GetUserAttribute(CtxKeyInputToken).(int64))
}

// The Extract* sub-functions leave InputToken raw and populate per-provider cache fields; the
// net + aggregate happens once in normalizeCacheBuckets (called by GetTokenUsage after the loop).
func TestExtractThenNormalizeNetsAndAggregates(t *testing.T) {
	ctx := newTestHttpContext()
	body := []byte(`{
		"usage": {
			"prompt_tokens": 100,
			"prompt_tokens_details": {
				"cached_tokens": 80,
				"cache_write_tokens": 15
			}
		}
	}`)
	usage := TokenUsage{
		InputTokenDetails:  make(map[string]int64),
		OutputTokenDetails: make(map[string]int64),
	}

	ExtractInputTokens(ctx, body, &usage)
	ExtractInputTokenDetails(ctx, body, &usage)

	// Before normalize: input is raw, per-provider fields populated.
	assertInt64(t, "raw input token", 100, usage.InputToken)
	assertInt64(t, "cached input token", 80, usage.CachedInputToken)
	assertInt64(t, "openai cache write input token", 15, usage.OpenAICacheWriteInputToken)
	// CtxKeyInputToken attribute keeps the raw value for the cross-chunk fallback.
	assertInt64(t, "input token attribute", 100, ctx.GetUserAttribute(CtxKeyInputToken).(int64))

	usage.normalizeCacheBuckets()

	// After normalize: input is net (100 - cached80 - cache_write15 = 5), buckets aggregated.
	assertInt64(t, "net input token", 5, usage.InputToken)
	assertInt64(t, "cache read bucket", 80, usage.CacheReadInputToken)
	assertInt64(t, "cache write bucket", 15, usage.CacheWriteInputToken)
}

// Without a provider-supplied total, the fallback total is computed from the RAW input (inside the
// per-chunk loop, before netting), so cached tokens are counted exactly once. The returned
// InputToken is net, but TotalToken still reflects the true provider total (prompt + completion).
func TestGetTokenUsageOpenAIWithoutProviderTotalKeepsCachedTokensInTotal(t *testing.T) {
	ctx := newTestHttpContext()
	body := []byte(`{
		"id": "chatcmpl-test",
		"model": "gpt-4.1-mini",
		"usage": {
			"prompt_tokens": 100,
			"completion_tokens": 25,
			"prompt_tokens_details": {
				"cached_tokens": 80
			}
		}
	}`)

	usage := GetTokenUsage(ctx, body)

	// net input = 100 - cached(80) = 20; fallback total uses raw input: 100 + 25 = 125.
	assertInt64(t, "net input token", 20, usage.InputToken)
	assertInt64(t, "cache read bucket", 80, usage.CacheReadInputToken)
	assertInt64(t, "total token", 125, usage.TotalToken)
}

func TestGetTokenUsageOpenAIResponsesCacheWriteNettedAndAggregated(t *testing.T) {
	ctx := newTestHttpContext()
	// GPT-5.6 responses sample: cached_tokens and cache_write_tokens are both inclusive (part of
	// input_tokens). Both are netted out; cached folds into cache-read, cache_write into cache-write.
	body := []byte(`{
		"response": {
			"id": "resp_test",
			"model": "gpt-5.6-sol",
			"usage": {
				"input_tokens": 71092,
				"output_tokens": 1641,
				"total_tokens": 72733,
				"input_tokens_details": {
					"cached_tokens": 59136,
					"cache_write_tokens": 5000
				}
			}
		}
	}`)

	usage := GetTokenUsage(ctx, body)

	// net input = 71092 - cached(59136) - cache_write(5000) = 6956.
	assertInt64(t, "net input token", 6956, usage.InputToken)
	assertInt64(t, "cache read bucket", 59136, usage.CacheReadInputToken)
	assertInt64(t, "cache write bucket", 5000, usage.CacheWriteInputToken)
	assertInt64(t, "cached input token", 59136, usage.CachedInputToken)
	assertInt64(t, "openai cache write input token", 5000, usage.OpenAICacheWriteInputToken)
	// map key preserved verbatim so ai-statistics log output (input_token_details) is unaffected.
	assertInt64(t, "cache_write_tokens detail preserved", 5000, usage.InputTokenDetails[InputTokenDetailsKeyOpenAICacheWriteTokens])
	assertInt64(t, "cached token detail", 59136, usage.InputTokenDetails["cached_tokens"])
	assertInt64(t, "total token", 72733, usage.TotalToken)
}

func TestGetTokenUsageOpenAIResponsesCacheWriteEqualsAllFreshInput(t *testing.T) {
	ctx := newTestHttpContext()
	// Real production log (gpt-5.6-sol): cache_write_tokens(314499) == input_tokens(314499).
	// Netting clamps input to 0; the whole fresh-input batch bills at the cache-write rate.
	body := []byte(`{
		"response": {
			"id": "resp_test",
			"model": "gpt-5.6-sol",
			"usage": {
				"input_tokens": 314499,
				"output_tokens": 1259,
				"total_tokens": 322670,
				"input_tokens_details": {
					"cached_tokens": 6912,
					"cache_write_tokens": 314499
				}
			}
		}
	}`)

	usage := GetTokenUsage(ctx, body)

	// net input = max(0, 314499 - 6912 - 314499) = 0.
	assertInt64(t, "net input token", 0, usage.InputToken)
	assertInt64(t, "cache read bucket", 6912, usage.CacheReadInputToken)
	assertInt64(t, "cache write bucket", 314499, usage.CacheWriteInputToken)
	assertInt64(t, "cache_write_tokens detail preserved", 314499, usage.InputTokenDetails[InputTokenDetailsKeyOpenAICacheWriteTokens])
	assertInt64(t, "total token", 322670, usage.TotalToken)
}

func TestGetTokenUsageOpenAIResponsesZeroCacheWriteLeavesCacheWriteEmpty(t *testing.T) {
	ctx := newTestHttpContext()
	// The documented GPT-5.6 sample where cache_write_tokens is 0.
	body := []byte(`{
		"response": {
			"id": "resp_test",
			"model": "gpt-5.6-sol",
			"usage": {
				"input_tokens": 71092,
				"output_tokens": 1641,
				"total_tokens": 72733,
				"input_tokens_details": {
					"cached_tokens": 59136,
					"cache_write_tokens": 0
				}
			}
		}
	}`)

	usage := GetTokenUsage(ctx, body)

	// net input = 71092 - cached(59136) = 11956; cache_write is 0.
	assertInt64(t, "net input token", 11956, usage.InputToken)
	assertInt64(t, "cache read bucket", 59136, usage.CacheReadInputToken)
	assertInt64(t, "cache write bucket", 0, usage.CacheWriteInputToken)
	assertInt64(t, "cache_write_tokens detail preserved", 0, usage.InputTokenDetails[InputTokenDetailsKeyOpenAICacheWriteTokens])
	assertInt64(t, "total token", 72733, usage.TotalToken)
}

// Bailian (阿里云百炼) IMPLICIT cache (OpenAI-compat): only cached_tokens is present in
// prompt_tokens_details. It bills at the 20% implicit rate, so it stays in CachedInputToken and the
// cached_tokens key is preserved (parity with the plain-OpenAI cache-read case above).
func TestGetTokenUsageBailianImplicitCacheKeepsCachedTokens(t *testing.T) {
	ctx := newTestHttpContext()
	// From the doc's implicit-cache OpenAI-compat sample (usage.prompt_tokens_details.cached_tokens).
	body := []byte(`{
		"id": "chatcmpl-6ada9ed2",
		"model": "qwen-plus",
		"usage": {
			"prompt_tokens": 3019,
			"completion_tokens": 104,
			"total_tokens": 3123,
			"prompt_tokens_details": {
				"cached_tokens": 2048
			}
		}
	}`)

	usage := GetTokenUsage(ctx, body)

	// Implicit: net input = 3019 - cached(2048) = 971.
	assertInt64(t, "net input token", 971, usage.InputToken)
	assertInt64(t, "cache read bucket", 2048, usage.CacheReadInputToken)
	assertInt64(t, "cache write bucket", 0, usage.CacheWriteInputToken)
	assertInt64(t, "cached input token", 2048, usage.CachedInputToken)
	// No explicit-cache fields set.
	assertInt64(t, "bailian cache read", 0, usage.BailianCacheReadInputToken)
	assertInt64(t, "bailian cache creation", 0, usage.BailianCacheCreationInputToken)
	// cached_tokens key preserved (implicit 20% tier); no cache_read_input_tokens key added.
	assertInt64(t, "cached_tokens detail preserved", 2048, usage.InputTokenDetails[InputTokenDetailsKeyCachedTokens])
	if _, ok := usage.InputTokenDetails[InputTokenDetailsKeyAnthropicMessagesUsageCacheReadInputTokens]; ok {
		t.Fatalf("implicit cache must not emit cache_read_input_tokens")
	}
	assertInt64(t, "total token", 3123, usage.TotalToken)
}

// Bailian EXPLICIT cache (OpenAI-compat): cache_creation_input_tokens sits alongside cached_tokens in
// prompt_tokens_details. The hit bills at 10% (explicit-read) and the creation at 125%, so the hit is
// re-keyed from cached_tokens to cache_read_input_tokens (into the cache-read bucket) and the creation
// folds into the cache-write bucket. Both are inclusive, so both are netted out of InputToken.
func TestGetTokenUsageBailianExplicitCacheReKeysHitAndCapturesCreation(t *testing.T) {
	ctx := newTestHttpContext()
	// Mirrors the doc's explicit-cache 2nd request: a 1605 hit plus a freshly created block, ~15 uncached.
	body := []byte(`{
		"id": "chatcmpl-explicit",
		"model": "qwen3.7-max",
		"usage": {
			"prompt_tokens": 1920,
			"completion_tokens": 50,
			"total_tokens": 1970,
			"prompt_tokens_details": {
				"cached_tokens": 1605,
				"cache_creation_input_tokens": 300
			}
		}
	}`)

	usage := GetTokenUsage(ctx, body)

	// Explicit: net input = 1920 - cache_read(1605) - cache_creation(300) = 15.
	assertInt64(t, "net input token", 15, usage.InputToken)
	assertInt64(t, "cache read bucket", 1605, usage.CacheReadInputToken)
	assertInt64(t, "cache write bucket", 300, usage.CacheWriteInputToken)
	assertInt64(t, "bailian cache read", 1605, usage.BailianCacheReadInputToken)
	assertInt64(t, "bailian cache creation", 300, usage.BailianCacheCreationInputToken)
	// Implicit field must stay empty so the two tiers are not double-counted.
	assertInt64(t, "cached input token", 0, usage.CachedInputToken)
	// Hit re-keyed to cache_read_input_tokens; original cached_tokens key removed.
	assertInt64(t, "cache_read_input_tokens detail", 1605, usage.InputTokenDetails[InputTokenDetailsKeyAnthropicMessagesUsageCacheReadInputTokens])
	assertInt64(t, "cache_creation_input_tokens detail", 300, usage.InputTokenDetails[InputTokenDetailsKeyAnthropicMessagesUsageCacheCreationInputTokens])
	if _, ok := usage.InputTokenDetails[InputTokenDetailsKeyCachedTokens]; ok {
		t.Fatalf("explicit cache must re-key cached_tokens to cache_read_input_tokens")
	}
	assertInt64(t, "total token", 1970, usage.TotalToken)
}

// DeepSeek exposes cache-read hits at the top level of usage (it does not emit cached_tokens).
// tokenusage collects it into DeepSeekPromptCacheHitToken without touching InputToken.
func TestGetTokenUsageDeepSeekPromptCacheHitFromTopLevel(t *testing.T) {
	ctx := newTestHttpContext()
	body := []byte(`{
		"id": "chatcmpl-deepseek",
		"model": "deepseek-chat",
		"usage": {
			"prompt_tokens": 1000,
			"completion_tokens": 200,
			"total_tokens": 1200,
			"prompt_cache_hit_tokens": 768,
			"prompt_cache_miss_tokens": 232
		}
	}`)

	usage := GetTokenUsage(ctx, body)

	// DeepSeek prompt_cache_hit is inclusive: net input = 1000 - 768 = 232.
	assertInt64(t, "net input token", 232, usage.InputToken)
	assertInt64(t, "cache read bucket", 768, usage.CacheReadInputToken)
	assertInt64(t, "deepseek prompt cache hit", 768, usage.DeepSeekPromptCacheHitToken)
	assertInt64(t, "prompt_cache_hit detail", 768, usage.InputTokenDetails[InputTokenDetailsKeyDeepSeekPromptCacheHitTokens])
	assertInt64(t, "total token", 1200, usage.TotalToken)
	// DeepSeek emits no cached_tokens, so the OpenAI per-provider field stays empty.
	assertInt64(t, "cached input token", 0, usage.CachedInputToken)
}

// Some gateways relocate the DeepSeek hit count under prompt_tokens_details; the top-level path
// is preferred, and the details fallback is used only when the top level is absent.
func TestGetTokenUsageDeepSeekPromptCacheHitFromInputDetailsFallback(t *testing.T) {
	ctx := newTestHttpContext()
	body := []byte(`{
		"id": "chatcmpl-deepseek",
		"model": "deepseek-chat",
		"usage": {
			"prompt_tokens": 1000,
			"completion_tokens": 200,
			"total_tokens": 1200,
			"prompt_tokens_details": {
				"prompt_cache_hit_tokens": 512
			}
		}
	}`)

	usage := GetTokenUsage(ctx, body)

	// net input = 1000 - 512 = 488; hit folds into cache-read.
	assertInt64(t, "net input token", 488, usage.InputToken)
	assertInt64(t, "cache read bucket", 512, usage.CacheReadInputToken)
	assertInt64(t, "deepseek prompt cache hit", 512, usage.DeepSeekPromptCacheHitToken)
	assertInt64(t, "prompt_cache_hit detail", 512, usage.InputTokenDetails[InputTokenDetailsKeyDeepSeekPromptCacheHitTokens])
}

// Gemini reports cache-read as usageMetadata.cachedContentTokenCount; it is exposed both as the
// dedicated GeminiCachedContentToken field and in the InputTokenDetails map (for log output).
func TestGetTokenUsageGeminiCachedContentToken(t *testing.T) {
	ctx := newTestHttpContext()
	body := []byte(`{
		"usageMetadata": {
			"promptTokenCount": 1000,
			"candidatesTokenCount": 200,
			"totalTokenCount": 1200,
			"cachedContentTokenCount": 600
		}
	}`)

	usage := GetTokenUsage(ctx, body)

	// Gemini cached_content is inclusive: net input = 1000 - 600 = 400.
	assertInt64(t, "net input token", 400, usage.InputToken)
	assertInt64(t, "cache read bucket", 600, usage.CacheReadInputToken)
	assertInt64(t, "gemini cached content", 600, usage.GeminiCachedContentToken)
	assertInt64(t, "cached_content detail", 600, usage.InputTokenDetails[InputTokenDetailsKeyGeminiCachedContentTokenCount])
	assertInt64(t, "total token", 1200, usage.TotalToken)
}

// Anthropic cache is EXCLUSIVE: the provider's input_tokens never included it, so it must NOT be
// netted out. cache_read + cache_creation still aggregate into the cache buckets.
func TestGetTokenUsageAnthropicCacheNotNetted(t *testing.T) {
	ctx := newTestHttpContext()
	// input_tokens is read from message.usage; the cache fields from top-level usage (as the
	// extractor paths define). output_tokens also lives under message.usage.
	body := []byte(`{
		"type": "message",
		"message": {
			"model": "claude-sonnet-4",
			"usage": {
				"input_tokens": 100,
				"output_tokens": 40
			}
		},
		"usage": {
			"cache_read_input_tokens": 50,
			"cache_creation_input_tokens": 20
		}
	}`)

	usage := GetTokenUsage(ctx, body)

	// input stays 100 (Anthropic input excludes cache — nothing to net).
	assertInt64(t, "input token unchanged", 100, usage.InputToken)
	assertInt64(t, "cache read bucket", 50, usage.CacheReadInputToken)
	assertInt64(t, "cache write bucket", 20, usage.CacheWriteInputToken)
	assertInt64(t, "anthropic cache read", 50, usage.AnthropicCacheReadInputToken)
	assertInt64(t, "anthropic cache creation", 20, usage.AnthropicCacheCreationInputToken)
	// fallback total (no provider total_tokens): raw input 100 + output 40 + anthropic 50 + 20 = 210.
	assertInt64(t, "total token", 210, usage.TotalToken)
}

func assertInt64(t *testing.T, name string, want, got int64) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: want %d, got %d", name, want, got)
	}
}
