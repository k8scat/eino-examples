package einoagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	utilcb "github.com/cloudwego/eino/utils/callbacks"
)

func skipIfNoEnv(t *testing.T) {
	t.Helper()
	for _, env := range []string{"DEEPSEEK_BASE_URL", "DEEPSEEK_CHAT_MODEL", "DEEPSEEK_API_KEY"} {
		if os.Getenv(env) == "" {
			t.Skipf("skipping: env %s not set", env)
		}
	}
}

func newToolCallbackHandler(t *testing.T) callbacks.Handler {
	return utilcb.NewHandlerHelper().Tool(&utilcb.ToolCallbackHandler{
		OnStart: func(ctx context.Context, info *callbacks.RunInfo, input *tool.CallbackInput) context.Context {
			var pretty json.RawMessage = []byte(input.ArgumentsInJSON)
			b, err := json.MarshalIndent(pretty, "", "  ")
			if err != nil {
				b = []byte(input.ArgumentsInJSON)
			}
			t.Logf("[ToolCall] %s input: %s", info.Name, string(b))
			return ctx
		},
		OnEnd: func(ctx context.Context, info *callbacks.RunInfo, output *tool.CallbackOutput) context.Context {
			t.Logf("[ToolCall] %s output: %s", info.Name, output.Response)
			return ctx
		},
	}).Handler()
}

func TestReactAgentOpenFile(t *testing.T) {
	ctx := context.Background()
	skipIfNoEnv(t)

	chatModel, err := newChatModel(ctx)
	if err != nil {
		t.Fatalf("failed to create chat model: %v", err)
	}

	openTool, err := NewOpenFileTool(ctx)
	if err != nil {
		t.Fatalf("failed to create open file tool: %v", err)
	}

	reactAgent, err := react.NewAgent(ctx, &react.AgentConfig{
		MaxStep:          10,
		ToolCallingModel: chatModel,
		ToolsConfig: compose.ToolsNodeConfig{
			Tools: []tool.BaseTool{openTool},
		},
	})
	if err != nil {
		t.Fatalf("failed to create react agent: %v", err)
	}

	targetFile := "/tmp/eino_test_open.txt"
	if err := os.WriteFile(targetFile, []byte("hello eino"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(targetFile)

	handler := newToolCallbackHandler(t)
	msg, err := reactAgent.Generate(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "请帮我打开文件: " + targetFile,
		},
	}, agent.WithComposeOptions(compose.WithCallbacks(handler)))
	if err != nil {
		t.Fatalf("agent generate failed: %v", err)
	}

	t.Logf("agent response: %s", msg.Content)
}

func TestReactAgentReadFileAndSummarize(t *testing.T) {
	ctx := context.Background()
	skipIfNoEnv(t)

	chatModel, err := newChatModel(ctx)
	if err != nil {
		t.Fatalf("failed to create chat model: %v", err)
	}

	readFileTool, err := NewReadFileTool(ctx)
	if err != nil {
		t.Fatalf("failed to create read file tool: %v", err)
	}

	reactAgent, err := react.NewAgent(ctx, &react.AgentConfig{
		MaxStep:          10,
		ToolCallingModel: chatModel,
		ToolsConfig: compose.ToolsNodeConfig{
			Tools: []tool.BaseTool{readFileTool},
		},
	})
	if err != nil {
		t.Fatalf("failed to create react agent: %v", err)
	}

	targetFile := "/tmp/eino_test_read.txt"
	content := `Eino（谐音"爱诺"）是基于 Golang 的 AI 应用开发框架，由字节跳动开源。
它提供了丰富的 AI 组件抽象和编排能力，包括：
1. ChatModel - 大语言模型接口抽象
2. Tool - 工具调用能力
3. Retriever - 检索增强生成(RAG)
4. Flow/Agent - ReAct 等 Agent 编排模式
5. Compose - 灵活的组件编排图
Eino 的目标是让 Go 开发者能够快速构建高质量的 AI 应用。`

	if err := os.WriteFile(targetFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(targetFile)

	handler := newToolCallbackHandler(t)
	sr, err := reactAgent.Stream(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "请读取文件 " + targetFile + " 的内容，并用三句话总结其核心要点。",
		},
	}, agent.WithComposeOptions(compose.WithCallbacks(handler)))
	if err != nil {
		t.Fatalf("agent stream failed: %v", err)
	}
	defer sr.Close()

	for {
		msg, err := sr.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("stream recv failed: %v", err)
		}
		if msg.Content != "" {
			fmt.Printf("[role=%s] content=%q toolCalls=%v\n", msg.Role, msg.Content, msg.ToolCalls)
			// os.Stderr.WriteString(msg.Content)
			// os.Stderr.Sync()
		}
	}
	// os.Stderr.WriteString("\n")
}
