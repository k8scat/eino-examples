/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"sync"

	"github.com/cloudwego/eino-ext/callbacks/apmplus"
	"github.com/cloudwego/eino-ext/callbacks/langfuse"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/compose"
	flowagent "github.com/cloudwego/eino/flow/agent"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"github.com/hertz-contrib/sse"

	"github.com/cloudwego/eino-examples/quickstart/eino_assistant/eino/einoagent"
	"github.com/cloudwego/eino-examples/quickstart/eino_assistant/pkg/mem"
)

var memory = mem.GetDefaultMemory()

var cbHandler callbacks.Handler

var once sync.Once

func Init() error {
	var err error
	once.Do(func() {
		os.MkdirAll("log", 0755)
		var f *os.File
		f, err = os.OpenFile("log/eino.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return
		}

		cbConfig := &LogCallbackConfig{
			Detail: true,
			Writer: f,
		}
		if os.Getenv("DEBUG") == "true" {
			cbConfig.Debug = true
		}
		// this is for invoke option of WithCallback
		cbHandler = LogCallback(cbConfig)

		// init global callback, for trace and metrics
		callbackHandlers := make([]callbacks.Handler, 0)
		if os.Getenv("APMPLUS_APP_KEY") != "" {
			region := os.Getenv("APMPLUS_REGION")
			if region == "" {
				region = "cn-beijing"
			}
			fmt.Println("[eino agent] INFO: use apmplus as callback, watch at: https://console.volcengine.com/apmplus-server")
			cbh, _, err := apmplus.NewApmplusHandler(&apmplus.Config{
				Host:        fmt.Sprintf("apmplus-%s.volces.com:4317", region),
				AppKey:      os.Getenv("APMPLUS_APP_KEY"),
				ServiceName: "eino-assistant",
				Release:     "release/v0.0.1",
			})
			if err != nil {
				log.Fatal(err)
			}

			callbackHandlers = append(callbackHandlers, cbh)
		}

		if os.Getenv("LANGFUSE_PUBLIC_KEY") != "" && os.Getenv("LANGFUSE_SECRET_KEY") != "" {
			fmt.Println("[eino agent] INFO: use langfuse as callback, watch at: https://cloud.langfuse.com")
			cbh, _ := langfuse.NewLangfuseHandler(&langfuse.Config{
				Host:      "https://cloud.langfuse.com",
				PublicKey: os.Getenv("LANGFUSE_PUBLIC_KEY"),
				SecretKey: os.Getenv("LANGFUSE_SECRET_KEY"),
				Name:      "Eino Assistant",
				Public:    true,
				Release:   "release/v0.0.1",
				UserID:    "eino_god",
				Tags:      []string{"eino", "assistant"},
			})
			callbackHandlers = append(callbackHandlers, cbh)
		}
		if len(callbackHandlers) > 0 {
			callbacks.InitCallbackHandlers(callbackHandlers)
		}
	})
	return err
}

func RunAgent(ctx context.Context, id string, msg string, s *sse.Stream) error {

	runner, err := einoagent.BuildEinoAgent(ctx)
	if err != nil {
		return err
	}

	conversation := memory.GetConversation(id, true)

	userMessage := &einoagent.UserMessage{
		ID:      id,
		Query:   msg,
		History: conversation.GetMessages(),
	}
	if os.Getenv("APMPLUS_APP_KEY") != "" {
		// set session info for apmplus callback
		ctx = apmplus.SetSession(ctx, apmplus.WithSessionID(id), apmplus.WithUserID("eino-assistant-user"))
	}

	msgFutureOpt, msgFuture := react.WithMessageFuture()

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()
		iter := msgFuture.GetMessageStreams()

		var lastAssistantMsg *schema.Message

		for {
			msgSr, ok, e := iter.Next()
			if e != nil {
				slog.Error("error getting next message stream", "error", e)
				break
			}
			if !ok {
				break
			}

			var chunks []*schema.Message
			for {
				chunk, err := msgSr.Recv()
				if err != nil {
					if errors.Is(err, io.EOF) {
						break
					}
					slog.Error("error receiving message chunk", "error", err)
					break
				}
				// fmt.Printf("chunk: %s, time: %s\n", chunk.Content, time.Now().Format("2006-01-02 15:04:05.000"))
				chunks = append(chunks, chunk)
				s.Publish(&sse.Event{
					Data: []byte(chunk.Content),
				})
			}

			fullMsg, err := schema.ConcatMessages(chunks)
			if err != nil || fullMsg == nil {
				continue
			}

			if fullMsg.Role == schema.Assistant && len(fullMsg.ToolCalls) == 0 {
				lastAssistantMsg = fullMsg
			}
		}

		conversation.Append(schema.UserMessage(msg))
		if lastAssistantMsg != nil {
			conversation.Append(lastAssistantMsg)
		}
	}()

	_, err = runner.Stream(ctx, userMessage,
		compose.WithCallbacks(cbHandler),
		flowagent.GetComposeOptions(msgFutureOpt)[0].DesignateNode("ReactAgent"),
	)
	if err != nil {
		return fmt.Errorf("failed to stream: %w", err)
	}

	wg.Wait()

	return nil
}

type LogCallbackConfig struct {
	Detail bool
	Debug  bool
	Writer io.Writer
}

func LogCallback(config *LogCallbackConfig) callbacks.Handler {
	if config == nil {
		config = &LogCallbackConfig{
			Detail: true,
			Writer: os.Stdout,
		}
	}
	if config.Writer == nil {
		config.Writer = os.Stdout
	}
	builder := callbacks.NewHandlerBuilder()
	builder.OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
		fmt.Fprintf(config.Writer, "[view]: start [%s:%s:%s]\n", info.Component, info.Type, info.Name)
		if config.Detail {
			var b []byte
			if config.Debug {
				b, _ = json.MarshalIndent(input, "", "  ")
			} else {
				b, _ = json.Marshal(input)
			}
			fmt.Fprintf(config.Writer, "%s\n", string(b))
		}
		return ctx
	})
	builder.OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
		fmt.Fprintf(config.Writer, "[view]: end [%s:%s:%s]\n", info.Component, info.Type, info.Name)
		return ctx
	})
	return builder.Build()
}
