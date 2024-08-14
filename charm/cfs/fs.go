// Package cfs provides a Starlark module for Charm FS.
package cfs

import (
	"bytes"
	"fmt"
	"io"
	gofs "io/fs"
	"path/filepath"

	"github.com/1set/starlet"
	"github.com/1set/starlet/dataconv"
	tps "github.com/1set/starlet/dataconv/types"
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
		"read":    starlark.NewBuiltin(ModuleName+".read", m.readFile),
		"write":   starlark.NewBuiltin(ModuleName+".write", m.writeFile),
		"remove":  starlark.NewBuiltin(ModuleName+".remove", m.removeFile),
		"stat":    starlark.NewBuiltin(ModuleName+".stat", m.statFile),
		"listdir": starlark.NewBuiltin(ModuleName+".listdir", m.listDirContents),
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
	var name tps.StringOrBytes
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "name", &name); err != nil {
		return nil, err
	}

	// get the client
	cf, err := m.getClient()
	if err != nil {
		return nil, err
	}

	// open file for reading
	f, err := cf.Open(name.GoString())
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
	var name, content tps.StringOrBytes
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "name", &name, "content", &content); err != nil {
		return nil, err
	}

	// get the client
	cf, err := m.getClient()
	if err != nil {
		return nil, err
	}

	// write as file
	fn := name.GoString()
	vf := CreateVirtualFile(fn, content.GoBytes())
	err = cf.WriteFile(fn, vf)
	return none, err
}

func (m *Module) removeFile(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name tps.StringOrBytes
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "name", &name); err != nil {
		return nil, err
	}

	// get the client
	cf, err := m.getClient()
	if err != nil {
		return nil, err
	}

	// delete the file
	err = cf.Remove(name.GoString())
	return none, err
}

func (m *Module) statFile(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name tps.StringOrBytes
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "name", &name); err != nil {
		return nil, err
	}

	// get the client
	cf, err := m.getClient()
	if err != nil {
		return nil, err
	}

	// open file for stat
	f, err := cf.Open(name.GoString())
	if err != nil {
		return nil, err
	}
	defer f.Close() // nolint:errcheck

	// get file info
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	// convert
	// TODO: like https://github.com/1set/starlet/blob/master/lib/file/stat.go
	return dataconv.GoToStarlarkViaJSON(fi)
}

// listDirContents returns a list of directory contents.
func (m *Module) listDirContents(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		path       tps.StringOrBytes
		recursive  bool
		filterFunc = tps.NullableCallable{}
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "path", &path, "recursive?", &recursive, "filter?", &filterFunc); err != nil {
		return nil, err
	}
	// get filter func
	var ff starlark.Callable
	if !filterFunc.IsNull() {
		ff = filterFunc.Value()
	}

	// get the client
	cf, err := m.getClient()
	if err != nil {
		return nil, err
	}

	// scan directory contents
	var (
		ps = path.GoString()
		sl []starlark.Value
	)
	if err := gofs.WalkDir(cf, ps, func(p string, info gofs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// filter path
		sp := starlark.String(p)
		if ff != nil {
			filtered, err := starlark.Call(thread, ff, starlark.Tuple{sp}, nil)
			if err != nil {
				return fmt.Errorf("filter %q: %w", p, err)
			}
			if fb, ok := filtered.(starlark.Bool); !ok {
				return fmt.Errorf("filter %q: got %s, want bool", p, filtered.Type())
			} else if fb == false {
				return nil // skip path
			}
		}

		// add path to list
		sl = append(sl, sp)

		// check if we should list recursively
		if !recursive && p != ps && info.IsDir() {
			return filepath.SkipDir
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("%s: %w", b.Name(), err)
	}
	return starlark.NewList(sl), nil
}
