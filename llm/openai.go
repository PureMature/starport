// Package llm provides a Starlark module that calls OpenAI models.
package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/1set/starlet"
	"github.com/1set/starlet/dataconv/types"
	"github.com/PureMature/starport/base"
	oai "github.com/sashabaranov/go-openai"
	"go.starlark.net/starlark"
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
	none     = starlark.None
	emptyStr string
)

func newMessageStruct(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// Parse arguments
	var (
		role          = types.NewNullableStringOrBytes(oai.ChatMessageRoleUser)
		msgText       = types.NewNullableStringOrBytesNoDefault()
		msgImageBytes = types.NewNullableStringOrBytesNoDefault()
		msgImageFile  = types.NewNullableStringOrBytesNoDefault()
		msgImageURL   = types.NewNullableStringOrBytesNoDefault()
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "role?", role,
		"text?", msgText, "image?", msgImageBytes, "image_file?", msgImageFile, "image_url?", msgImageURL,
	); err != nil {
		return none, err
	}

	// Create a new message
	md := starlark.NewDict(2)

	// Add key values
	prepared := map[string]*types.NullableStringOrBytes{
		"role":       role,
		"text":       msgText,
		"image":      msgImageBytes,
		"image_file": msgImageFile,
		"image_url":  msgImageURL,
	}
	for key, val := range prepared {
		if !val.IsNullOrEmpty() {
			// md.SetKey(starlark.String(key), starlark.String(val.GoString())) // TODO: use .StarlarkString()
			md.SetKey(starlark.String(key), val.StarlarkString())
		}
	}

	return md, nil
}

func (m *Module) genChatFunc() starlark.Callable {
	return starlark.NewBuiltin(ModuleName+".chat", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var (
			// message
			msgText       = types.NewNullableStringOrBytesNoDefault()
			msgImageBytes = types.NewNullableStringOrBytesNoDefault()
			msgImageFile  = types.NewNullableStringOrBytesNoDefault()
			msgImageURL   = types.NewNullableStringOrBytesNoDefault()
			messages      = types.NewOneOrManyNoDefault[*starlark.Dict]()
			// model request
			userModel        = types.NewNullableStringOrBytesNoDefault()
			numOfChoices     = 1
			maxTokens        = 64
			temperature      = types.FloatOrInt(1.0)
			topP             = types.FloatOrInt(1.0)
			frequencyPenalty = types.FloatOrInt(0.0)
			presencePenalty  = types.FloatOrInt(0.0)
			stopSequences    = types.NewOneOrManyNoDefault[starlark.String]()
			responseFormat   = types.NewNullableStringOrBytes("text")
			// call
			retryTimes   = 1
			fullResponse = false
			allowError   = false
		)
		if err := starlark.UnpackArgs(b.Name(), args, kwargs,
			"text?", msgText, "image?", msgImageBytes, "image_file?", msgImageFile, "image_url?", msgImageURL, "messages?", messages,
			"model?", userModel, "n?", &numOfChoices, "max_tokens?", &maxTokens, "temperature?", &temperature, "top_p?", &topP, "frequency_penalty?", &frequencyPenalty, "presence_penalty?", &presencePenalty, "stop?", stopSequences, "response_format?", responseFormat,
			"retry?", &retryTimes, "full_response?", &fullResponse, "allow_error?", &allowError,
		); err != nil {
			return none, err
		}

		// history messages, prepend user message if defined
		allMsgs := messages.Slice()
		usrMd := starlark.NewDict(1)
		prepared := map[string]*types.NullableStringOrBytes{
			"text":       msgText,
			"image":      msgImageBytes,
			"image_file": msgImageFile,
			"image_url":  msgImageURL,
		}
		for key, val := range prepared {
			if !val.IsNullOrEmpty() {
				usrMd.SetKey(starlark.String(key), val.StarlarkString())
			}
		}
		if usrMd.Len() > 0 {
			usrMd.SetKey(starlark.String("role"), starlark.String(oai.ChatMessageRoleUser))
			allMsgs = append([]*starlark.Dict{usrMd}, allMsgs...)
		}

		// define the function behavior
		fmt.Println("😄 text:", msgText.GoString())
		fmt.Println("image:", msgImageBytes.GoString())
		fmt.Println("image_file:", msgImageFile.GoString())
		fmt.Println("image_url:", msgImageURL.GoString())
		fmt.Println("messages:", allMsgs)
		fmt.Println("model:", userModel.GoString())
		fmt.Println("n:", numOfChoices)
		fmt.Println("max_tokens:", maxTokens)
		fmt.Println("temperature:", temperature)
		fmt.Println("top_p:", topP)
		fmt.Println("frequency_penalty:", frequencyPenalty)
		fmt.Println("presence_penalty:", presencePenalty)
		fmt.Println("stop:", stopSequences)
		fmt.Println("response_format:", responseFormat.GoString())
		fmt.Println("retry:", retryTimes)
		fmt.Println("full_response:", fullResponse)
		fmt.Println("allow_error:", allowError)

		stopSequences.Slice()

		return none, nil

		/*

		   nm = chat(
		       model="GPT-4o", # or from the config
		       messages=msg, # or [msg, msg2, msg3]
		       max_tokens=64,
		       temperature=1.0,    # int or float [0,2]
		       top_p=1.0,          # int or float [0,1]
		       frequency_penalty=0.0,  # int or float [-2,2]
		       presence_penalty=0.0,   # int or float [-2,2]
		       stop=["\n", "User:"],   # string or list of strings
		       user="User",    # track the user
		   )

		*/

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
			Content: msgText.GoString(),
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
				ResponseFormat: &oai.ChatCompletionResponseFormat{
					Type: oai.ChatCompletionResponseFormatTypeJSONObject,
				},
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
