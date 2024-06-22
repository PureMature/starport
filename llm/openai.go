// Package llm provides a Starlark module that calls OpenAI models.
package llm

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
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
		"message": starlark.NewBuiltin("message", newMessageStruct),
		"chat":    m.genChatFunc(),
		"draw":    m.genDrawFunc(),
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

		// TODO: convert list of history messages to OpenAI format struct

		// convert messages to OpenAI format
		chatMessages, err := messagesToChatMessages(allMsgs)
		if err != nil {
			return none, err
		}

		// Get the client

		// Send the request to OpenAI with Retry

		// Return the response or error

		// For response, extract of data or full response

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
		//msg := oai.ChatCompletionMessage{
		//	Role:    oai.ChatMessageRoleUser,
		//	Content: msgText.GoString(),
		//}
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
				Messages: chatMessages,
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

// getStringFromDict retrieves a string value from a dictionary and whether the key exists
func getStringFromDict(d *starlark.Dict, key string) (string, bool) {
	v, ok, err := d.Get(starlark.String(key))
	// if the key is not found, or the value is nil, or there is an error, return an empty string
	if err != nil || !ok || v == nil {
		return emptyStr, false
	}
	// if the value is a string, return the string
	if s, ok := v.(starlark.String); ok {
		return string(s), true
	} else if b, ok := v.(starlark.Bytes); ok {
		return string(b), true
	}
	// otherwise, return an empty string
	return emptyStr, false
}

// Function to read file and convert it to base64 data
func imageFileToBase64(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return "", err
	}

	fileSize := fileInfo.Size()
	fileBuffer := make([]byte, fileSize)

	bytesRead, err := file.Read(fileBuffer)
	if err != nil {
		return "", err
	}
	if bytesRead != int(fileSize) {
		return "", fmt.Errorf("expected to read %d bytes but read %d", fileSize, bytesRead)
	}

	base64Data := base64.StdEncoding.EncodeToString(fileBuffer)
	mimeType := mime.TypeByExtension(filepath.Ext(filePath))
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data), nil
}

func imageDataToBase64(data []byte) string {
	base64Data := base64.StdEncoding.EncodeToString(data)
	mimeType := http.DetectContentType(data)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)
}

func messagesToChatMessages(msgs []*starlark.Dict) ([]oai.ChatCompletionMessage, error) {
	var res []oai.ChatCompletionMessage
	for i, md := range msgs {
		msg := oai.ChatCompletionMessage{}
		role, ok := getStringFromDict(md, "role")
		if !ok {
			return nil, fmt.Errorf("message %d: role is required", i+1)
		}
		msg.Role = role

		// get the content
		text, okT := getStringFromDict(md, "text")
		imageBytes, okI := getStringFromDict(md, "image")
		imageFile, okF := getStringFromDict(md, "image_file")
		imageURL, okU := getStringFromDict(md, "image_url")
		okImg := okI || okF || okU

		// if all are empty, return an error
		if !(okT || okImg) {
			return nil, fmt.Errorf("message %d: at least one of text, image, image_file, or image_url is required", i+1)
		}

		// check if text and image are both set
		if okT && !okImg {
			// only text is set, treat as one text message
			msg.Content = text
			res = append(res, msg)
			continue
		}

		// build the message parts
		var mcp []oai.ChatMessagePart
		if okT { // for text part
			mcp = append(mcp, oai.ChatMessagePart{
				Type: oai.ChatMessagePartTypeText,
				Text: text,
			})
		}
		if okU { // for image URL part
			mcp = append(mcp, oai.ChatMessagePart{
				Type: oai.ChatMessagePartTypeImageURL,
				ImageURL: &oai.ChatMessageImageURL{
					URL:    imageURL,
					Detail: oai.ImageURLDetailAuto,
				},
			})
		}
		if okI { // for image content part, convert to mime & base64
			b64 := imageDataToBase64([]byte(imageBytes))
			mcp = append(mcp, oai.ChatMessagePart{
				Type: oai.ChatMessagePartTypeImageURL,
				ImageURL: &oai.ChatMessageImageURL{
					URL:    b64,
					Detail: oai.ImageURLDetailAuto,
				},
			})
		}
		if okF { // for image file part, read and convert to mime & base64
			b64, err := imageFileToBase64(imageFile)
			if err != nil {
				return nil, fmt.Errorf("message %d: %w", i+1, err)
			}
			mcp = append(mcp, oai.ChatMessagePart{
				Type: oai.ChatMessagePartTypeImageURL,
				ImageURL: &oai.ChatMessageImageURL{
					URL:    b64,
					Detail: oai.ImageURLDetailAuto,
				},
			})
		}

		// set the message parts back to the message
		msg.MultiContent = mcp
		res = append(res, msg)
	}
	return res, nil
}
