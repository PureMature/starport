// Package ckv provides a Starlark module for Charm KV.
package ckv

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/1set/starlet"
	"github.com/1set/starlet/dataconv"
	"github.com/PureMature/starport/base"
	"github.com/PureMature/starport/charm/core"
	"go.starlark.net/starlark"
)

// ModuleName defines the expected name for this module when used in Starlark's load() function, e.g., load('ckv', 'get_id')
const ModuleName = "ckv"

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
		"list_db": m.genBuiltin("list_db", m.listDB),
	}
	return m.ExtendModuleLoader(ModuleName, additionalFuncs)
}

var (
	none = starlark.None
)

func (m *Module) genBuiltin(name string, fn dataconv.StarlarkFunc) starlark.Callable {
	return starlark.NewBuiltin(name, fn)
}

func (m *Module) listDB(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 0, 0); err != nil {
		return none, err
	}

	cc, err := m.InitializeClient()
	if err != nil {
		return none, err
	}

	// get data path
	dd, err := cc.DataPath()
	if err != nil {
		return none, err
	}
	dp := filepath.Join(dd, "kv")

	// list db folders
	entries, err := os.ReadDir(dp)
	if err != nil {
		return nil, err
	}
	var dbList []string
	for _, e := range entries {
		if e.IsDir() {
			dbList = append(dbList, e.Name())
		}
	}

	// sort dbList
	sort.Strings(dbList)

	// return dbList
	return core.StringsToStarlarkList(dbList), nil
}
