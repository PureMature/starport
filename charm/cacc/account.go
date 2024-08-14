// Package cacc provides a Starlark module for Charm Accounts.
package cacc

import (
	"github.com/1set/starlet"
	"github.com/1set/starlet/dataconv"
	tps "github.com/1set/starlet/dataconv/types"
	"github.com/PureMature/starport/base"
	"github.com/PureMature/starport/charm/core"
	"go.starlark.net/starlark"
)

// ModuleName defines the expected name for this module when used in Starlark's load() function, e.g., load('cacc', 'get_bio')
const ModuleName = "cacc"

// Module wraps the ConfigurableModule with specific functionality for Charm Accounts.
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
		"set_username":  starlark.NewBuiltin(ModuleName+".set_username", m.setUsername),
		"get_username":  starlark.NewBuiltin(ModuleName+".get_username", m.getUsername),
		"get_host":      starlark.NewBuiltin(ModuleName+".get_host", m.getHost),
		"get_bio":       starlark.NewBuiltin(ModuleName+".get_bio", m.getBio),
		"get_userid":    starlark.NewBuiltin(ModuleName+".get_userid", m.getUserID),
		"get_key_files": starlark.NewBuiltin(ModuleName+".get_key_files", m.getKeyFiles),
		"get_keys":      starlark.NewBuiltin(ModuleName+".get_keys", m.getKeys),
	}
	return m.ExtendModuleLoader(ModuleName, additionalFuncs)
}

var (
	none = starlark.None
)

func (m *Module) genBuiltin(name string, fn dataconv.StarlarkFunc) starlark.Callable {
	return starlark.NewBuiltin(name, fn)
}

func (m *Module) setUsername(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name tps.StringOrBytes
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "name", &name); err != nil {
		return none, err
	}

	cc, err := m.InitializeClient()
	if err != nil {
		return none, err
	}

	if _, err := cc.SetName(name.GoString()); err != nil {
		return none, err
	}
	return none, nil
}

func (m *Module) getUsername(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 0, 0); err != nil {
		return none, err
	}

	cc, err := m.InitializeClient()
	if err != nil {
		return none, err
	}

	bio, err := cc.Bio()
	if err != nil {
		return none, err
	}
	return starlark.String(bio.Name), nil
}

func (m *Module) getHost(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 0, 0); err != nil {
		return none, err
	}

	cc, err := m.InitializeClient()
	if err != nil {
		return none, err
	}

	return starlark.String(cc.Config.Host), nil
}

func (m *Module) getBio(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 0, 0); err != nil {
		return none, err
	}

	cc, err := m.InitializeClient()
	if err != nil {
		return none, err
	}

	bio, err := cc.Bio()
	if err != nil {
		return none, err
	}
	return dataconv.GoToStarlarkViaJSON(bio)
}

func (m *Module) getUserID(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 0, 0); err != nil {
		return none, err
	}

	cc, err := m.InitializeClient()
	if err != nil {
		return none, err
	}

	id, err := cc.ID()
	if err != nil {
		return none, err
	}
	return starlark.String(id), nil
}

func (m *Module) getKeyFiles(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 0, 0); err != nil {
		return none, err
	}

	cc, err := m.InitializeClient()
	if err != nil {
		return none, err
	}

	keyFiles := cc.AuthKeyPaths()
	return core.StringsToStarlarkList(keyFiles), nil
}

func (m *Module) getKeys(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 0, 0); err != nil {
		return none, err
	}

	cc, err := m.InitializeClient()
	if err != nil {
		return none, err
	}

	keys, err := cc.AuthorizedKeysWithMetadata()
	if err != nil {
		return none, err
	}
	return dataconv.GoToStarlarkViaJSON(keys)
}
