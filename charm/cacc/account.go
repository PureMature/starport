// Package cacc provides a Starlark module for Charm Accounts.
package cacc

import (
	"encoding/json"
	"github.com/1set/starlet"
	"github.com/1set/starlet/dataconv"
	"github.com/PureMature/starport/base"
	"github.com/PureMature/starport/charm/core"
	"go.starlark.net/starlark"
)

// ModuleName defines the expected name for this module when used in Starlark's load() function, e.g., load('cacc', 'get_id')
const ModuleName = "cacc"

// Module wraps the ConfigurableModule with specific functionality for sending emails.
type Module struct {
	*core.CommonModule
}

// NewModule creates a new instance of Module. It doesn't set any configuration values, nor provide any setters.
func NewModule() *Module {
	return &Module{
		core.NewCommonModule(),
	}
}

// NewModuleWithConfig creates a new instance of Module with the given configuration values.
func NewModuleWithConfig(host, dataDirPath, keyFilePath string, sshPort, httpPort uint16) *Module {
	return &Module{
		core.NewCommonModuleWithConfig(host, dataDirPath, keyFilePath, sshPort, httpPort),
	}
}

// NewModuleWithGetter creates a new instance of Module with the given configuration getters.
func NewModuleWithGetter(host, dataDirPath, keyFilePath, sshPort, httpPort base.ConfigGetter[string]) *Module {
	return &Module{
		core.NewCommonModuleWithGetter(host, dataDirPath, keyFilePath, sshPort, httpPort),
	}
}

// LoadModule returns the Starlark module loader with the email-specific functions.
func (m *Module) LoadModule() starlet.ModuleLoader {
	additionalFuncs := starlark.StringDict{
		"get_bio":    m.genGetBio(),
		"get_userid": m.genGetUserID(),
	}
	return m.ExtendModuleLoader(ModuleName, additionalFuncs)
}

var (
	none = starlark.None
)

// genGetBio generates the Starlark callable function to get the user's profile.
func (m *Module) genGetBio() starlark.Callable {
	return starlark.NewBuiltin("get_bio", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		// check arguments
		if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 0, 0); err != nil {
			return none, err
		}

		// create a new client
		cc, err := m.InitializeClient()
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

// genGetUserID generates the Starlark callable function to get the user's ID.
func (m *Module) genGetUserID() starlark.Callable {
	return starlark.NewBuiltin("get_userid", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		// check arguments
		if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 0, 0); err != nil {
			return none, err
		}

		// create a new client
		cc, err := m.InitializeClient()
		if err != nil {
			return none, err
		}

		// get the user's ID
		id, err := cc.ID()
		if err != nil {
			return none, err
		}
		return starlark.String(id), nil
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
