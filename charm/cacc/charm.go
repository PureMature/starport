// Package cacc provides a Starlark module for Charm Accounts.
package cacc

import (
	"github.com/1set/starlet"
	"github.com/PureMature/starport/base"
	"go.starlark.net/starlark"
)

// ModuleName defines the expected name for this module when used in Starlark's load() function, e.g., load('cacc', 'get_id')
const ModuleName = "cacc"

// Module wraps the ConfigurableModule with specific functionality for sending emails.
type Module struct {
	cfgMod *base.ConfigurableModule[string]
}

// NewModule creates a new instance of Module.
func NewModule() *Module {
	cm := base.NewConfigurableModule[string]()
	return &Module{cfgMod: cm}
}

// NewModuleWithConfig creates a new instance of Module with the given configuration values.
func NewModuleWithConfig(resendAPIKey, senderDomain string) *Module {
	cm := base.NewConfigurableModule[string]()
	cm.SetConfigValue("resend_api_key", resendAPIKey)
	cm.SetConfigValue("sender_domain", senderDomain)
	return &Module{cfgMod: cm}
}

// NewModuleWithGetter creates a new instance of Module with the given configuration getters.
func NewModuleWithGetter(resendAPIKey, senderDomain base.ConfigGetter[string]) *Module {
	cm := base.NewConfigurableModule[string]()
	cm.SetConfig("resend_api_key", resendAPIKey)
	cm.SetConfig("sender_domain", senderDomain)
	return &Module{cfgMod: cm}
}

// LoadModule returns the Starlark module loader with the email-specific functions.
func (m *Module) LoadModule() starlet.ModuleLoader {
	additionalFuncs := starlark.StringDict{
		// "send": m.genSendFunc(),
	}
	return m.cfgMod.LoadModule(ModuleName, additionalFuncs)
}
