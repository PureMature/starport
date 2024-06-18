// Package llm provides a Starlark module that calls OpenAI models.
package llm

import (
	"github.com/1set/starlet"
	"github.com/PureMature/starport/base"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// ModuleName defines the expected name for this module when used in Starlark's load() function, e.g., load('llm', 'chat')
const ModuleName = "llm"

// Module wraps the ConfigurableModule with specific functionality for calling OpenAI models.
type Module struct {
	cfgMod *base.ConfigurableModule[string]
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

func newMessageStruct(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// Parse arguments
	var (
		message starlark.String
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "message", &message); err != nil {
		return nil, err
	}

	// Create a new message struct
	return starlarkstruct.FromStringDict(starlark.String("Message"), starlark.StringDict{
		"message": message,
	}), nil
}

func (m *Module) genChatFunc() starlark.Callable {
	return starlark.NewBuiltin(ModuleName+".chat", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		//// Load config
		//provider, err := m.cfgMod.GetConfig("openai_provider")
		//if err != nil {
		//	return starlark.None, err
		//}
		//endpointURL, err := m.cfgMod.GetConfig("openai_endpoint_url")
		//if err != nil {
		//	return starlark.None, err
		//}
		return starlark.None, nil
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
		return starlark.None, nil
	})
}
