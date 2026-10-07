package tui

import (
	"context"
	"io"
	"time"

	"github.com/metaspartan/gotui/v5/widgets"
	"projemble/internal/auth"
)

func startChatGPTAuth(newRegistration bool) (context.CancelFunc, chan chatGPTAuthResult) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	results := make(chan chatGPTAuthResult, 1)
	go func() {
		var tokenSource *auth.ChatGPTTokenSource
		var err error
		if newRegistration {
			err = auth.LoginNewRegistration(ctx, io.Discard)
		}
		if err == nil {
			tokenSource, err = auth.EnsureChatGPT(ctx, io.Discard)
		}
		results <- chatGPTAuthResult{source: tokenSource, err: err}
	}()
	return cancel, results
}

func applyChatGPTAuthResult(result chatGPTAuthResult, pending *bool, cancel *context.CancelFunc, models *modelPicker, options *generationOptions, modelInput *widgets.Input, current *page, validation *string) {
	*pending = false
	if *cancel != nil {
		(*cancel)()
		*cancel = nil
	}
	if result.err != nil {
		*validation = chatGPTSignInMessage(result.err)
		return
	}
	models.invalidate()
	options.Credentials = result.source
	modelInput.Placeholder = "Model ID available to your ChatGPT plan"
	*current = aiModelPage
	*validation = ""
}
