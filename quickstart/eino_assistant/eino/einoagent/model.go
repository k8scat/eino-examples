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

package einoagent

import (
	"context"
	"os"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
)

func newChatModel(ctx context.Context) (cm model.ToolCallingChatModel, err error) {
	// TODO Modify component configuration here.
	// config := &deepseek.ChatModelConfig{
	// 	BaseURL:     os.Getenv("DEEPSEEK_BASE_URL"),
	// 	Model:       os.Getenv("DEEPSEEK_CHAT_MODEL"),
	// 	APIKey:      os.Getenv("DEEPSEEK_API_KEY"),
	// 	Temperature: 0.7,
	// }
	// cm, err = deepseek.NewChatModel(ctx, config)
	// if err != nil {
	// 	return nil, err
	// }

	config := &openai.ChatModelConfig{
		BaseURL: os.Getenv("OPENAI_BASE_URL"),
		Model:   os.Getenv("OPENAI_MODEL_NAME"),
		APIKey:  os.Getenv("OPENAI_API_KEY"),
	}
	cm, err = openai.NewChatModel(ctx, config)
	if err != nil {
		return nil, err
	}
	return cm, nil
}
