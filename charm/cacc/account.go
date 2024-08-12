// Package cacc provides a Starlark module for Charm Accounts.
package cacc

import (
	"encoding/json"
	"github.com/1set/starlet"
	"github.com/1set/starlet/dataconv"
	"github.com/PureMature/starport/base"
	"github.com/charmbracelet/charm/client"
	"go.starlark.net/starlark"
	"os"
	"strconv"
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
func NewModuleWithConfig(host, dataDirPath, keyFilePath string, sshPort, httpPort uint16) *Module {
	cm := base.NewConfigurableModule[string]()
	cm.SetConfigValue("host", host)
	cm.SetConfigValue("data_dir", dataDirPath)
	cm.SetConfigValue("key_file", keyFilePath)
	cm.SetConfigValue("ssh_port", strconv.Itoa(int(sshPort)))
	cm.SetConfigValue("http_port", strconv.Itoa(int(httpPort)))
	return &Module{cfgMod: cm}
}

// NewModuleWithGetter creates a new instance of Module with the given configuration getters.
func NewModuleWithGetter(host, dataDirPath, keyFilePath, sshPort, httpPort base.ConfigGetter[string]) *Module {
	cm := base.NewConfigurableModule[string]()
	cm.SetConfig("host", host)
	cm.SetConfig("data_dir", dataDirPath)
	cm.SetConfig("key_file", keyFilePath)
	cm.SetConfig("ssh_port", sshPort)
	cm.SetConfig("http_port", httpPort)
	return &Module{cfgMod: cm}
}

// LoadModule returns the Starlark module loader with the email-specific functions.
func (m *Module) LoadModule() starlet.ModuleLoader {
	additionalFuncs := starlark.StringDict{
		"get_bio": m.genGetBio(),
	}
	return m.cfgMod.LoadModule(ModuleName, additionalFuncs)
}

// prepareEnvirons prepares the environment variables for the module.
func (m *Module) prepareEnvirons() error {
	keyMaps := map[string]string{
		"host":      "CHARM_HOST",
		"data_dir":  "CHARM_DATA_DIR",
		"key_file":  "CHARM_IDENTITY_KEY",
		"ssh_port":  "CHARM_SSH_PORT",
		"http_port": "CHARM_HTTP_PORT",
	}
	for cfgKey, envKey := range keyMaps {
		val, err := m.cfgMod.GetConfig(cfgKey)
		if err != nil {
			return err
		}
		if val != "" {
			err = os.Setenv(envKey, val)
			if err != nil {
				return err
			}
		}
	}
	// TODO: rewrite with default values + override by non-empty values
	return nil
}

var (
	none = starlark.None
)

// genGetBio generates the Starlark callable function to get the user's profile.
func (m *Module) genGetBio() starlark.Callable {
	return starlark.NewBuiltin("get_host", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		err := m.prepareEnvirons()
		if err != nil {
			return none, err
		}
		cc, err := client.NewClientWithDefaults()
		if err != nil {
			return none, err
		}
		bio, err := cc.Bio()
		if err != nil {
			return none, err
		}
		return structToStarlark(bio)
	})
}

// structToStarlark converts a Go struct to a Starlark value via JSON conversion.
func structToStarlark(v interface{}) (starlark.Value, error) {
	bs, err := json.Marshal(v)
	if err != nil {
		return none, err
	}
	return dataconv.DecodeStarlarkJSON(bs)
}
