// Package cfs provides a Starlark module for Charm FS.
package cfs

import (
	"bytes"
	"fmt"
	"io"

	"github.com/1set/starlet"
	"github.com/PureMature/starport/base"
	"github.com/PureMature/starport/charm/core"
	"github.com/charmbracelet/charm/fs"
	"go.starlark.net/starlark"
)

// ModuleName defines the expected name for this module when used in Starlark's load() function, e.g., load('cfs', 'listdir')
const ModuleName = "cfs"

// Module wraps the ConfigurableModule with specific functionality for Charm FS.
type Module struct {
	*core.CommonModule
	cf *fs.FS
}

// NewModule creates a new instance of Module. It doesn't set any configuration values, nor provide any setters.
func NewModule() *Module {
	return &Module{
		core.NewCommonModule(),
		nil,
	}
}

// NewModuleWithConfig creates a new instance of Module with the given configuration values.
func NewModuleWithConfig(host, dataDirPath, keyFilePath string, sshPort, httpPort uint16) *Module {
	return &Module{
		core.NewCommonModuleWithConfig(host, dataDirPath, keyFilePath, sshPort, httpPort),
		nil,
	}
}

// NewModuleWithGetter creates a new instance of Module with the given configuration getters.
func NewModuleWithGetter(host, dataDirPath, keyFilePath, sshPort, httpPort base.ConfigGetter[string]) *Module {
	return &Module{
		core.NewCommonModuleWithGetter(host, dataDirPath, keyFilePath, sshPort, httpPort),
		nil,
	}
}

// LoadModule returns the Starlark module loader with the email-specific functions.
func (m *Module) LoadModule() starlet.ModuleLoader {
	additionalFuncs := starlark.StringDict{
		"read":  starlark.NewBuiltin("read", m.readFile),
		"write": starlark.NewBuiltin("write", m.writeFile),

		//// kv ops
		//"get":         starlark.NewBuiltin("get", m.getString),
		//"set":         starlark.NewBuiltin("set", m.setString),
		//"get_json":    starlark.NewBuiltin("get_json", m.getJSON),
		//"set_json":    starlark.NewBuiltin("set_json", m.setJSON),
		//"delete":      starlark.NewBuiltin("delete", m.deleteKey),
		//"list":        starlark.NewBuiltin("list", m.listAll),
		//"list_keys":   starlark.NewBuiltin("list_keys", m.listKeys),
		//"list_values": starlark.NewBuiltin("list_values", m.listValues),
		//// db ops
		//"list_db": starlark.NewBuiltin("list_db", m.listDB),
		//"sync":    starlark.NewBuiltin("sync", m.syncDB),
		//"reset":   starlark.NewBuiltin("reset", m.resetLocalCopy),
	}
	return m.ExtendModuleLoader(ModuleName, additionalFuncs)
}

var (
	emptyStr string
	none     = starlark.None
)

func (m *Module) getClient() (*fs.FS, error) {
	// return the client if it's already created
	if m.cf != nil {
		return m.cf, nil
	}

	// create the client
	cc, err := m.InitializeClient()
	if err != nil {
		return nil, err
	}

	// create fs instance
	cf, err := fs.NewFSWithClient(cc)
	if err != nil {
		return nil, err
	}
	m.cf = cf
	return cf, nil
}

func (m *Module) readFile(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "name", &name); err != nil {
		return nil, err
	}

	// get the client
	cf, err := m.getClient()
	if err != nil {
		return nil, err
	}

	// open the file for reading
	f, err := cf.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close() // nolint:errcheck

	// check the file
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return nil, fmt.Errorf("is a directory: %s", name)
	}

	// read the content
	buf := bytes.NewBuffer(nil)
	_, err = io.Copy(buf, f)
	if err != nil {
		return nil, err
	}
	return starlark.String(buf.Bytes()), nil
}

func (m *Module) writeFile(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name, content string
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "name", &name, "content", &content); err != nil {
		return nil, err
	}

	// get the client
	cf, err := m.getClient()
	if err != nil {
		return nil, err
	}

	// write as file
	vf := CreateVirtualFileFromString(name, content)
	err = cf.WriteFile(name, vf)
	return none, err
}
