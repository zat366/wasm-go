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

func TestGetTokenUsageOpenAIChatCompletionsUsesNonCachedInputTokens(t *testing.T) {
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

	assertInt64(t, "input token", 20, usage.InputToken)
	assertInt64(t, "cached input token", 80, usage.CachedInputToken)
	assertInt64(t, "cached token detail", 80, usage.InputTokenDetails["cached_tokens"])
	assertInt64(t, "total token", 125, usage.TotalToken)
	assertInt64(t, "input token attribute", 20, ctx.GetUserAttribute(CtxKeyInputToken).(int64))
}

func TestGetTokenUsageOpenAIResponsesUsesNonCachedInputTokens(t *testing.T) {
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

	assertInt64(t, "input token", 40, usage.InputToken)
	assertInt64(t, "cached input token", 160, usage.CachedInputToken)
	assertInt64(t, "cached token detail", 160, usage.InputTokenDetails["cached_tokens"])
	assertInt64(t, "total token", 230, usage.TotalToken)
	assertInt64(t, "input token attribute", 40, ctx.GetUserAttribute(CtxKeyInputToken).(int64))
}

func TestExtractInputTokensOpenAIUsesNonCachedInputTokens(t *testing.T) {
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

	assertInt64(t, "input token", 20, usage.InputToken)
	assertInt64(t, "cached input token", 80, usage.CachedInputToken)
	assertInt64(t, "input token attribute", 20, ctx.GetUserAttribute(CtxKeyInputToken).(int64))
}

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

	assertInt64(t, "input token", 20, usage.InputToken)
	assertInt64(t, "total token", 125, usage.TotalToken)
}

func assertInt64(t *testing.T, name string, want, got int64) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: want %d, got %d", name, want, got)
	}
}
