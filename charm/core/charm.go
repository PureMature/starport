// Package core provides the basic module for Charm API client.
package core

import (
	"encoding/json"
	"os"
	"strconv"

	"github.com/1set/starlet"
	"github.com/1set/starlet/dataconv"
	"github.com/PureMature/starport/base"
	"github.com/charmbracelet/charm/client"
	"go.starlark.net/starlark"
)

// CommonModule wraps the ConfigurableModule with specific functionality for sending emails.
type CommonModule struct {
	cfgMod *base.ConfigurableModule[string]
}

// NewCommonModule creates a new instance of CommonModule. It doesn't set any configuration values, nor provide any setters.
func NewCommonModule() *CommonModule {
	cm := base.NewConfigurableModule[string]()
	return &CommonModule{cfgMod: cm}
}

// NewCommonModuleWithConfig creates a new instance of CommonModule with the given configuration values.
func NewCommonModuleWithConfig(host, dataDirPath, keyFilePath string, sshPort, httpPort uint16) *CommonModule {
	cm := base.NewConfigurableModule[string]()
	cm.SetConfigValue("host", host)
	cm.SetConfigValue("data_dir", dataDirPath)
	cm.SetConfigValue("key_file", keyFilePath)
	cm.SetConfigValue("ssh_port", strconv.Itoa(int(sshPort)))
	cm.SetConfigValue("http_port", strconv.Itoa(int(httpPort)))
	return &CommonModule{cfgMod: cm}
}

// NewCommonModuleWithGetter creates a new instance of CommonModule with the given configuration getters.
func NewCommonModuleWithGetter(host, dataDirPath, keyFilePath, sshPort, httpPort base.ConfigGetter[string]) *CommonModule {
	cm := base.NewConfigurableModule[string]()
	cm.SetConfig("host", host)
	cm.SetConfig("data_dir", dataDirPath)
	cm.SetConfig("key_file", keyFilePath)
	cm.SetConfig("ssh_port", sshPort)
	cm.SetConfig("http_port", httpPort)
	return &CommonModule{cfgMod: cm}
}

// ExtendModuleLoader extends the module loader with given name and additional functions.
func (m *CommonModule) ExtendModuleLoader(name string, addons starlark.StringDict) starlet.ModuleLoader {
	additionalFuncs := starlark.StringDict{
		"get_bio": m.genGetBio(),
	}
	for k, v := range addons {
		additionalFuncs[k] = v
	}
	return m.cfgMod.LoadModule(name, additionalFuncs)
}

// prepareEnvirons prepares the environment variables for the module.
func (m *CommonModule) prepareEnvirons() error {
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
			continue
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
func (m *CommonModule) genGetBio() starlark.Callable {
	return starlark.NewBuiltin("get_bio", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		// check arguments
		if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 0, 0); err != nil {
			return none, err
		}
		if err := m.prepareEnvirons(); err != nil {
			return none, err
		}

		// create a new client
		cc, err := client.NewClientWithDefaults()
		if err != nil {
			return none, err
		}

		// get the user's bio
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
