// Package llm provides a Starlark module that calls OpenAI models.
package llm

import (
	"context"
	"errors"
	"fmt"
	"github.com/1set/starlet"
	"github.com/1set/starlet/dataconv/types"
	"github.com/PureMature/starport/base"
	oai "github.com/sashabaranov/go-openai"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"strings"
)

// ModuleName defines the expected name for this module when used in Starlark's load() function, e.g., load('llm', 'chat')
const ModuleName = "llm"

// Module wraps the ConfigurableModule with specific functionality for calling OpenAI models.
type Module struct {
	cfgMod *base.ConfigurableModule[string]
	cli    *oai.Client
}

// NewModule creates a new instance of Module.
func NewModule() *Module {
	cm := base.NewConfigurableModule[string]()
	return &Module{cfgMod: cm}
}

// NewModuleWithConfig creates a new instance of Module with the given configuration values.
func NewModuleWithConfig(serviceProvider, endpointURL, apiKey, gptModel, dalleModel string) *Module {
	cm := base.NewConfigurableModule[string]()
	prefix := "openai_"
	cm.SetConfigValue(prefix+"provider", serviceProvider)
	cm.SetConfigValue(prefix+"endpoint_url", endpointURL)
	cm.SetConfigValue(prefix+"api_key", apiKey)
	cm.SetConfigValue(prefix+"gpt_model", gptModel)
	cm.SetConfigValue(prefix+"dalle_model", dalleModel)
	return &Module{cfgMod: cm}
}

// NewModuleWithGetter creates a new instance of Module with the given configuration getters.
func NewModuleWithGetter(serviceProvider, endpointURL, apiKey, gptModel, dalleModel base.ConfigGetter[string]) *Module {
	cm := base.NewConfigurableModule[string]()
	prefix := "openai_"
	cm.SetConfig(prefix+"provider", serviceProvider)
	cm.SetConfig(prefix+"endpoint_url", endpointURL)
	cm.SetConfig(prefix+"api_key", apiKey)
	cm.SetConfig(prefix+"gpt_model", gptModel)
	cm.SetConfig(prefix+"dalle_model", dalleModel)
	return &Module{cfgMod: cm}
}

// LoadModule returns the Starlark module loader with the email-specific functions.
func (m *Module) LoadModule() starlet.ModuleLoader {
	additionalFuncs := starlark.StringDict{
		"new_message": starlark.NewBuiltin("new_message", newMessageStruct),
		"chat":        m.genChatFunc(),
		"draw":        m.genDrawFunc(),
	}
	return m.cfgMod.LoadModule(ModuleName, additionalFuncs)
}

var (
	none = starlark.None
)

func newMessageStruct(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// Parse arguments
	var (
		message starlark.String
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "message", &message); err != nil {
		return none, err
	}

	// Create a new message struct
	return starlarkstruct.FromStringDict(starlark.String("Message"), starlark.StringDict{
		"message": message,
	}), nil
}

func (m *Module) genChatFunc() starlark.Callable {
	return starlark.NewBuiltin(ModuleName+".chat", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var (
			textMsg   types.NullableStringOrBytes
			userModel types.NullableStringOrBytes
		)
		if err := starlark.UnpackArgs(b.Name(), args, kwargs, "text?", &textMsg, "model?", &userModel); err != nil {
			return none, err
		}

		// get model
		model := m.getModel("openai_gpt_model", userModel.GoString())
		if model == "" {
			return none, errors.New("gpt model is not set")
		}

		// get client
		cli, err := m.getClient(model)
		if err != nil {
			return nil, err
		}

		// call OpenAI API
		msg := oai.ChatCompletionMessage{
			Role:    oai.ChatMessageRoleUser,
			Content: textMsg.GoString(),
		}
		resp, err := cli.CreateChatCompletion(
			context.Background(), // TODO: for context cancel
			//oai.ChatCompletionRequest{
			//	Model:            "",
			//	Messages:         nil,
			//	MaxTokens:        0,
			//	Temperature:      0,
			//	TopP:             0,
			//	N:                0,
			//	Stream:           false,
			//	Stop:             nil,
			//	PresencePenalty:  0,
			//	ResponseFormat:   nil,
			//	Seed:             nil,
			//	FrequencyPenalty: 0,
			//	LogitBias:        nil,
			//	LogProbs:         false,
			//	TopLogProbs:      0,
			//	User:             "",
			//	Functions:        nil,
			//	FunctionCall:     nil,
			//	Tools:            nil,
			//	ToolChoice:       nil,
			//	StreamOptions:    nil,
			//},
			oai.ChatCompletionRequest{
				Model:    model,
				Messages: []oai.ChatCompletionMessage{msg},
			},
		)
		if err != nil {
			return none, err
		}
		fmt.Println("Result:", resp)
		return starlark.String(resp.Choices[0].Message.Content), nil

		//// Load config
		//provider, err := m.cfgMod.GetConfig("openai_provider")
		//if err != nil {
		//	return starlark.None, err
		//}
		//endpointURL, err := m.cfgMod.GetConfig("openai_endpoint_url")
		//if err != nil {
		//	return starlark.None, err
		//}
		//return none, nil
	})
}

func (m *Module) genDrawFunc() starlark.Callable {
	return starlark.NewBuiltin(ModuleName+".draw", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		//// Load config
		//provider, err := m.cfgMod.GetConfig("openai_provider")
		//if err != nil {
		//	return starlark.None, err
		//}
		//endpointURL, err := m.cfgMod.GetConfig("openai_endpoint_url")
		//if err != nil {
		//	return starlark.None, err
		//}
		return none, nil
	})
}

// SetClient sets the OpenAI client for this module.
func (m *Module) SetClient(cli *oai.Client) {
	m.cli = cli
}

// getClient retrieves the OpenAI client for this module.
func (m *Module) getClient(model string) (*oai.Client, error) {
	if m.cli != nil {
		// use the existing client
		return m.cli, nil
	}

	provider, err := m.cfgMod.GetConfig("openai_provider")
	if err != nil {
		provider = "openai"
	}
	apiKey, err := m.cfgMod.GetConfig("openai_api_key")
	if err != nil {
		return nil, err
	}
	endpointURL, err := m.cfgMod.GetConfig("openai_endpoint_url")

	// create client configuration
	var cfg oai.ClientConfig
	switch strings.ToLower(provider) {
	case "azure": // Azure OpenAI services
		if err != nil {
			return nil, err // endpointURL is required for Azure
		}
		cfg = oai.DefaultAzureConfig(apiKey, endpointURL)
		cfg.APIVersion = `2024-02-01`
		cfg.AzureModelMapperFunc = func(_ string) string {
			return model
		}
	case "openai": // Vanilla OpenAI services
		cfg = oai.DefaultConfig(apiKey)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	// create a new client
	return oai.NewClientWithConfig(cfg), nil
}

// getModel retrieves the model name.
// If modelVal is empty, it will use the modelKey to retrieve the model value from the configuration.
func (m *Module) getModel(key, val string) string {
	// use the provided model value
	if val != "" {
		return val
	}
	// or retrieve the model value from the configuration
	model, err := m.cfgMod.GetConfig(key)
	if err == nil {
		return model
	}
	// return an empty string if the model is not found
	return ""
}
