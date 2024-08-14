// Package core provides the basic module for Charm API client.
package core

import (
	"fmt"
	"strconv"

	"github.com/1set/starlet"
	"github.com/1set/starlet/dataconv"
	"github.com/PureMature/starport/base"
	cmcli "github.com/charmbracelet/charm/client"
	"go.starlark.net/starlark"
)

// CommonModule wraps the ConfigurableModule with specific functionality for Charm API client.
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
	commonFuncs := starlark.StringDict{
		"get_config": m.genBuiltin("get_config", m.getConfig),
	}
	for k, v := range addons {
		commonFuncs[k] = v
	}
	return m.cfgMod.LoadModule(name, commonFuncs)
}

// InitializeClient creates a new Charm API client with the given configuration values.
func (m *CommonModule) InitializeClient() (*cmcli.Client, error) {
	// get default configuration from environment variables
	cfg, err := cmcli.ConfigFromEnv()
	if err != nil {
		return nil, err
	}
	// set configuration values from the module
	if host, err := m.cfgMod.GetConfig("host"); err == nil {
		cfg.Host = host
	}
	if dataDir, err := m.cfgMod.GetConfig("data_dir"); err == nil {
		cfg.DataDir = dataDir
	}
	if keyFile, err := m.cfgMod.GetConfig("key_file"); err == nil {
		cfg.IdentityKey = keyFile
	}
	if sshPort, err := m.cfgMod.GetConfig("ssh_port"); err == nil {
		cfg.SSHPort, err = strconv.Atoi(sshPort)
		if err != nil {
			return nil, fmt.Errorf("invalid SSH port: %w", err)
		}
	}
	if httpPort, err := m.cfgMod.GetConfig("http_port"); err == nil {
		cfg.HTTPPort, err = strconv.Atoi(httpPort)
		if err != nil {
			return nil, fmt.Errorf("invalid HTTP port: %w", err)
		}
	}
	// create a new client
	return cmcli.NewClient(cfg)
}

var (
	none = starlark.None
)

func (m *CommonModule) genBuiltin(name string, fn dataconv.StarlarkFunc) starlark.Callable {
	return starlark.NewBuiltin(name, fn)
}

// genGetConfig generates the Starlark callable function to get the configuration value.
func (m *CommonModule) getConfig(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// check arguments
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 0, 0); err != nil {
		return none, err
	}
	// get the client
	cli, err := m.InitializeClient()
	if err != nil {
		return none, err
	}
	// return the configuration
	return dataconv.GoToStarlarkViaJSON(cli.Config)
}
