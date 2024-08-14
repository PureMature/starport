// Package ckv provides a Starlark module for Charm KV.
package ckv

import (
	"errors"
	"os"
	"path/filepath"
	"sort"

	"github.com/1set/starlet"
	"github.com/1set/starlet/dataconv"
	"github.com/PureMature/starport/base"
	"github.com/PureMature/starport/charm/core"
	"github.com/charmbracelet/charm/kv"
	"github.com/dgraph-io/badger/v3"
	"go.starlark.net/starlark"
)

// ModuleName defines the expected name for this module when used in Starlark's load() function, e.g., load('ckv', 'get_id')
const ModuleName = "ckv"

// Module wraps the ConfigurableModule with specific functionality for sending emails.
type Module struct {
	*core.CommonModule
	dbs map[string]*kv.KV
}

// NewModule creates a new instance of Module. It doesn't set any configuration values, nor provide any setters.
func NewModule() *Module {
	return &Module{
		core.NewCommonModule(),
		make(map[string]*kv.KV),
	}
}

// NewModuleWithConfig creates a new instance of Module with the given configuration values.
func NewModuleWithConfig(host, dataDirPath, keyFilePath string, sshPort, httpPort uint16) *Module {
	return &Module{
		core.NewCommonModuleWithConfig(host, dataDirPath, keyFilePath, sshPort, httpPort),
		make(map[string]*kv.KV),
	}
}

// NewModuleWithGetter creates a new instance of Module with the given configuration getters.
func NewModuleWithGetter(host, dataDirPath, keyFilePath, sshPort, httpPort base.ConfigGetter[string]) *Module {
	return &Module{
		core.NewCommonModuleWithGetter(host, dataDirPath, keyFilePath, sshPort, httpPort),
		make(map[string]*kv.KV),
	}
}

// LoadModule returns the Starlark module loader with the email-specific functions.
func (m *Module) LoadModule() starlet.ModuleLoader {
	additionalFuncs := starlark.StringDict{
		"list_db":  m.genBuiltin("list_db", m.listDB),
		"get":      m.genBuiltin("get", m.getString),
		"set":      m.genBuiltin("set", m.setString),
		"get_json": m.genBuiltin("get_json", m.getJSON),
		"set_json": m.genBuiltin("set_json", m.setJSON),
		"delete":   m.genBuiltin("delete", m.deleteKey),
	}
	return m.ExtendModuleLoader(ModuleName, additionalFuncs)
}

var (
	emptyStr  string
	none      = starlark.None
	defaultDB = "starcli.kv.user.default"
)

func (m *Module) getDBClient(name string) (*kv.KV, error) {
	// use default db if name is empty
	if name == "" {
		name = defaultDB
	}
	// check if db is already opened
	if db, ok := m.dbs[name]; ok {
		return db, nil
	}

	// get client for opening db
	cc, err := m.InitializeClient()
	if err != nil {
		return nil, err
	}
	// get data path
	dd, err := cc.DataPath()
	if err != nil {
		return nil, err
	}
	pn := filepath.Join(dd, "/kv/", name)
	// BadgerDB options
	opts := badger.DefaultOptions(pn).WithLoggingLevel(badger.ERROR)
	opts.Logger = nil
	opts = opts.WithValueLogFileSize(10000000)

	// open db & save to cache
	db, err := kv.Open(cc, name, opts)
	if err != nil {
		return nil, err
	}
	m.dbs[name] = db
	return db, nil
}

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

func (m *Module) getValue(key, db string) (string, error) {
	// get db client
	dc, err := m.getDBClient(db)
	if err != nil {
		return emptyStr, err
	}

	// get value
	val, err := dc.Get([]byte(key))
	if err != nil {
		if nf := errors.Is(err, badger.ErrKeyNotFound); nf {
			return emptyStr, nil
		}
		return emptyStr, err
	}
	return string(val), nil
}

func (m *Module) setValue(key, value, db string) error {
	// get db client
	dc, err := m.getDBClient(db)
	if err != nil {
		return err
	}

	// set value
	err = dc.Set([]byte(key), []byte(value))
	if err != nil {
		return err
	}
	return nil
}

func (m *Module) getString(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		key string
		db  string
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "key", &key, "db?", &db); err != nil {
		return none, err
	}

	// get value
	vs, err := m.getValue(key, db)
	if err != nil {
		return none, err
	}
	return starlark.String(vs), nil
}

func (m *Module) setString(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		key   string
		value string
		db    string
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "key", &key, "value", &value, "db?", &db); err != nil {
		return none, err
	}

	// set value
	err := m.setValue(key, value, db)
	return none, err
}

func (m *Module) getJSON(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		key string
		db  string
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "key", &key, "db?", &db); err != nil {
		return none, err
	}

	// get value as string
	value, err := m.getValue(key, db)
	if err != nil {
		return none, err
	}

	// parse JSON
	return dataconv.DecodeStarlarkJSON([]byte(value))
}

func (m *Module) setJSON(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		key   string
		value starlark.Value
		db    string
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "key", &key, "value", &value, "db?", &db); err != nil {
		return none, err
	}

	// convert value to JSON
	js, err := dataconv.EncodeStarlarkJSON(value)
	if err != nil {
		return none, err
	}
	return none, m.setValue(key, js, db)
}

func (m *Module) deleteKey(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		key string
		db  string
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "key", &key, "db?", &db); err != nil {
		return none, err
	}

	// get db client
	dc, err := m.getDBClient(db)
	if err != nil {
		return none, err
	}

	// delete key
	err = dc.Delete([]byte(key))
	return none, err
}
