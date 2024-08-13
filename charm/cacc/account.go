// Package cacc provides a Starlark module for Charm Accounts.
package cacc

import (
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
		"set_username":  m.genSetUserName(),
		"get_username":  m.genGetUserName(),
		"get_host":      m.genGetHost(),
		"get_bio":       m.genGetBio(),
		"get_userid":    m.genGetUserID(),
		"get_key_files": m.genGetKeyFiles(),
		"get_keys":      m.genGetKeys(),
	}
	return m.ExtendModuleLoader(ModuleName, additionalFuncs)
}

var (
	none = starlark.None
)

// genSetUserName generates the Starlark callable function to set the user's name.
func (m *Module) genSetUserName() starlark.Callable {
	return starlark.NewBuiltin("set_username", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var name string
		if err := starlark.UnpackArgs(b.Name(), args, kwargs, "name", &name); err != nil {
			return none, err
		}

		// create a new client
		cc, err := m.InitializeClient()
		if err != nil {
			return none, err
		}

		// set the user's name
		if _, err := cc.SetName(name); err != nil {
			return none, err
		}
		return none, nil
	})
}

// genGetUserName generates the Starlark callable function to get the user's name.
func (m *Module) genGetUserName() starlark.Callable {
	return starlark.NewBuiltin("get_username", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		// check arguments
		if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 0, 0); err != nil {
			return none, err
		}

		// create a new client
		cc, err := m.InitializeClient()
		if err != nil {
			return none, err
		}

		// get the user's name from bio
		bio, err := cc.Bio()
		if err != nil {
			return none, err
		}
		return starlark.String(bio.Name), nil
	})
}

// getGetHost generates the Starlark callable function to get the user's host.
func (m *Module) genGetHost() starlark.Callable {
	return starlark.NewBuiltin("get_host", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		// check arguments
		if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 0, 0); err != nil {
			return none, err
		}

		// create a new client
		cc, err := m.InitializeClient()
		if err != nil {
			return none, err
		}

		// get the user's host
		return starlark.String(cc.Config.Host), nil
	})
}

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
		return dataconv.GoToStarlarkViaJSON(bio)
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

// genGetKeyFiles generates the Starlark callable function to get the user's key file paths.
func (m *Module) genGetKeyFiles() starlark.Callable {
	return starlark.NewBuiltin("get_key_files", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		// check arguments
		if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 0, 0); err != nil {
			return none, err
		}

		// create a new client
		cc, err := m.InitializeClient()
		if err != nil {
			return none, err
		}

		// get the user's key file paths
		keyFiles := cc.AuthKeyPaths()
		return dataconv.GoToStarlarkViaJSON(keyFiles)
	})
}

// getGetKeys generates the Starlark callable function to get the user's keys.
func (m *Module) genGetKeys() starlark.Callable {
	return starlark.NewBuiltin("get_keys", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		// check arguments
		if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 0, 0); err != nil {
			return none, err
		}

		// create a new client
		cc, err := m.InitializeClient()
		if err != nil {
			return none, err
		}

		// get the user's keys
		keys, err := cc.AuthorizedKeysWithMetadata()
		if err != nil {
			return none, err
		}
		return dataconv.GoToStarlarkViaJSON(keys)
	})
}
