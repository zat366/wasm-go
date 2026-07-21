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

// InputToken is returned verbatim from the provider (still includes cached tokens); the
// dedicated CachedInputToken field exposes the cache-read bucket for consumers to net out.
func TestGetTokenUsageOpenAIChatCompletionsIncludesCachedInputTokens(t *testing.T) {
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

	assertInt64(t, "input token", 100, usage.InputToken)
	assertInt64(t, "cached input token", 80, usage.CachedInputToken)
	assertInt64(t, "cached token detail", 80, usage.InputTokenDetails["cached_tokens"])
	assertInt64(t, "total token", 125, usage.TotalToken)
	assertInt64(t, "input token attribute", 100, ctx.GetUserAttribute(CtxKeyInputToken).(int64))
}

func TestGetTokenUsageOpenAIResponsesIncludesCachedInputTokens(t *testing.T) {
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

	assertInt64(t, "input token", 200, usage.InputToken)
	assertInt64(t, "cached input token", 160, usage.CachedInputToken)
	assertInt64(t, "cached token detail", 160, usage.InputTokenDetails["cached_tokens"])
	assertInt64(t, "total token", 230, usage.TotalToken)
	assertInt64(t, "input token attribute", 200, ctx.GetUserAttribute(CtxKeyInputToken).(int64))
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

// ExtractInputTokenDetails populates CachedInputToken and OpenAICacheWriteInputToken from the
// details map WITHOUT subtracting them from InputToken. Netting is a consumer-side concern now.
func TestExtractInputTokenDetailsPopulatesCacheFieldsWithoutNetting(t *testing.T) {
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

	// input stays raw (100); cache is reported separately, not subtracted.
	assertInt64(t, "input token", 100, usage.InputToken)
	assertInt64(t, "cached input token", 80, usage.CachedInputToken)
	assertInt64(t, "openai cache write input token", 15, usage.OpenAICacheWriteInputToken)
	assertInt64(t, "input token attribute", 100, ctx.GetUserAttribute(CtxKeyInputToken).(int64))
}

// Without a provider-supplied total, the fallback is Input(raw) + Output + Anthropic cache.
// Since InputToken now includes cached tokens (inclusive family), they are covered by Input and
// must NOT be added again, else they would be double-counted.
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

	// input raw 100 (includes 80 cached); fallback total = 100 + 25 = 125.
	assertInt64(t, "input token", 100, usage.InputToken)
	assertInt64(t, "total token", 125, usage.TotalToken)
}

func TestGetTokenUsageOpenAIResponsesCacheWriteReportedSeparately(t *testing.T) {
	ctx := newTestHttpContext()
	// GPT-5.6 responses sample: cached_tokens and cache_write_tokens are reported in dedicated
	// fields but NOT subtracted from input_token here — input stays raw and consumers net it out.
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

	// input_token stays at the raw provider value (71092); cache is exposed separately.
	assertInt64(t, "input token", 71092, usage.InputToken)
	assertInt64(t, "cached input token", 59136, usage.CachedInputToken)
	assertInt64(t, "openai cache write input token", 5000, usage.OpenAICacheWriteInputToken)
	// map key preserved verbatim so ai-statistics log output (input_token_details) is unaffected.
	assertInt64(t, "cache_write_tokens detail preserved", 5000, usage.InputTokenDetails[InputTokenDetailsKeyOpenAICacheWriteTokens])
	assertInt64(t, "cached token detail", 59136, usage.InputTokenDetails["cached_tokens"])
	assertInt64(t, "total token", 72733, usage.TotalToken)
	assertInt64(t, "input token attribute", 71092, ctx.GetUserAttribute(CtxKeyInputToken).(int64))
}

func TestGetTokenUsageOpenAIResponsesCacheWriteEqualsAllFreshInput(t *testing.T) {
	ctx := newTestHttpContext()
	// Real production log (gpt-5.6-sol): cache_write_tokens(314499) == input_tokens(314499).
	// tokenusage returns input verbatim; the consumer nets out inclusive cache when billing.
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

	// input stays raw (314499); consumer computes textInput = max(0, 314499 - 6912 - 314499) = 0.
	assertInt64(t, "input token", 314499, usage.InputToken)
	assertInt64(t, "cached input token", 6912, usage.CachedInputToken)
	assertInt64(t, "openai cache write input token", 314499, usage.OpenAICacheWriteInputToken)
	assertInt64(t, "cache_write_tokens detail preserved", 314499, usage.InputTokenDetails[InputTokenDetailsKeyOpenAICacheWriteTokens])
	assertInt64(t, "total token", 322670, usage.TotalToken)
}

func TestGetTokenUsageOpenAIResponsesZeroCacheWriteLeavesCacheCreationEmpty(t *testing.T) {
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

	// input stays raw (71092); cache_write is 0 so nothing folds into the write bucket.
	assertInt64(t, "input token", 71092, usage.InputToken)
	assertInt64(t, "cached input token", 59136, usage.CachedInputToken)
	assertInt64(t, "openai cache write input token", 0, usage.OpenAICacheWriteInputToken)
	assertInt64(t, "cache_write_tokens detail preserved", 0, usage.InputTokenDetails[InputTokenDetailsKeyOpenAICacheWriteTokens])
	assertInt64(t, "total token", 72733, usage.TotalToken)
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

	assertInt64(t, "input token", 1000, usage.InputToken)
	assertInt64(t, "deepseek prompt cache hit", 768, usage.DeepSeekPromptCacheHitToken)
	assertInt64(t, "prompt_cache_hit detail", 768, usage.InputTokenDetails[InputTokenDetailsKeyDeepSeekPromptCacheHitTokens])
	assertInt64(t, "total token", 1200, usage.TotalToken)
	// DeepSeek emits no cached_tokens, so the OpenAI bucket stays empty.
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

	assertInt64(t, "input token", 1000, usage.InputToken)
	assertInt64(t, "deepseek prompt cache hit", 512, usage.DeepSeekPromptCacheHitToken)
	assertInt64(t, "prompt_cache_hit detail", 512, usage.InputTokenDetails[InputTokenDetailsKeyDeepSeekPromptCacheHitTokens])
}

func assertInt64(t *testing.T, name string, want, got int64) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: want %d, got %d", name, want, got)
	}
}
