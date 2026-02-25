package readfile

import (
	"context"
	"fmt"
	"os"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type ReadFileToolImpl struct {
	config *ReadFileToolConfig
}

type ReadFileToolConfig struct {
	MaxSize int64
}

func defaultConfig() *ReadFileToolConfig {
	return &ReadFileToolConfig{
		MaxSize: 1 << 20, // 1MB
	}
}

func NewReadFileTool(ctx context.Context, config *ReadFileToolConfig) (tool.BaseTool, error) {
	if config == nil {
		config = defaultConfig()
	}
	t := &ReadFileToolImpl{config: config}
	return t.ToEinoTool()
}

func (r *ReadFileToolImpl) ToEinoTool() (tool.InvokableTool, error) {
	return utils.InferTool(
		"read_file",
		"read the text content of a file given its path, returns the file content as string",
		r.Invoke,
	)
}

type ReadFileReq struct {
	Path string `json:"path" jsonschema_description:"The absolute or relative path of the file to read"`
}

type ReadFileRes struct {
	Content string `json:"content" jsonschema_description:"The text content of the file"`
	Message string `json:"message" jsonschema_description:"Status message of the operation"`
}

func (r *ReadFileToolImpl) Invoke(ctx context.Context, req ReadFileReq) (ReadFileRes, error) {
	if req.Path == "" {
		return ReadFileRes{Message: "path is required"}, nil
	}

	info, err := os.Stat(req.Path)
	if err != nil {
		return ReadFileRes{Message: fmt.Sprintf("file not found: %s", req.Path)}, nil
	}
	if info.IsDir() {
		return ReadFileRes{Message: fmt.Sprintf("path is a directory, not a file: %s", req.Path)}, nil
	}
	if r.config.MaxSize > 0 && info.Size() > r.config.MaxSize {
		return ReadFileRes{Message: fmt.Sprintf("file too large (%d bytes), max allowed %d bytes", info.Size(), r.config.MaxSize)}, nil
	}

	data, err := os.ReadFile(req.Path)
	if err != nil {
		return ReadFileRes{Message: fmt.Sprintf("failed to read file: %s", err.Error())}, nil
	}

	return ReadFileRes{
		Content: string(data),
		Message: fmt.Sprintf("success, read %d bytes from %s", len(data), req.Path),
	}, nil
}
