// Package ckv provides a Starlark module for Charm KV.
package ckv

import (
	"errors"
	"os"
	"path/filepath"
	"sort"

	"github.com/1set/starlet"
	"github.com/1set/starlet/dataconv"
	tps "github.com/1set/starlet/dataconv/types"
	"github.com/PureMature/starport/base"
	"github.com/PureMature/starport/charm/core"
	"github.com/charmbracelet/charm/kv"
	"github.com/dgraph-io/badger/v3"
	"go.starlark.net/starlark"
)

// ModuleName defines the expected name for this module when used in Starlark's load() function, e.g., load('ckv', 'list_db')
const ModuleName = "ckv"

// Module wraps the ConfigurableModule with specific functionality for Charm KV.
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
		// kv ops
		"get":         starlark.NewBuiltin(ModuleName+".get", m.getString),
		"set":         starlark.NewBuiltin(ModuleName+".set", m.setString),
		"get_json":    starlark.NewBuiltin(ModuleName+".get_json", m.getJSON),
		"set_json":    starlark.NewBuiltin(ModuleName+".set_json", m.setJSON),
		"delete":      starlark.NewBuiltin(ModuleName+".delete", m.deleteKey),
		"list":        starlark.NewBuiltin(ModuleName+".list", m.listAll),
		"list_keys":   starlark.NewBuiltin(ModuleName+".list_keys", m.listKeys),
		"list_values": starlark.NewBuiltin(ModuleName+".list_values", m.listValues),
		// db ops
		"list_db": starlark.NewBuiltin(ModuleName+".list_db", m.listDB),
		"sync":    starlark.NewBuiltin(ModuleName+".sync", m.syncDB),
		"reset":   starlark.NewBuiltin(ModuleName+".reset", m.resetLocalCopy),
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

func (m *Module) getValue(db string, key []byte, failOnMissing bool) ([]byte, error) {
	// get db client
	dc, err := m.getDBClient(db)
	if err != nil {
		return nil, err
	}

	// get value
	val, err := dc.Get(key)
	if err != nil {
		if !failOnMissing {
			if nf := errors.Is(err, badger.ErrKeyNotFound); nf {
				return nil, nil
			}
		}
		return nil, err
	}
	return val, nil
}

func (m *Module) setValue(db string, key, value []byte) error {
	// get db client
	dc, err := m.getDBClient(db)
	if err != nil {
		return err
	}

	// set value
	err = dc.Set(key, value)
	if err != nil {
		return err
	}
	return nil
}

func (m *Module) getString(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		key           tps.StringOrBytes
		failOnMissing bool
		db            tps.StringOrBytes
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "key", &key, "fail_missing?", &failOnMissing, "db?", &db); err != nil {
		return none, err
	}

	// get value
	vs, err := m.getValue(db.GoString(), key.GoBytes(), failOnMissing)
	if err != nil {
		return none, err
	}
	return starlark.String(vs), nil
}

func (m *Module) setString(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		key   tps.StringOrBytes
		value starlark.Value
		db    tps.StringOrBytes
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "key", &key, "value", &value, "db?", &db); err != nil {
		return none, err
	}

	// set string representation of value
	err := m.setValue(db.GoString(), key.GoBytes(), []byte(dataconv.StarString(value)))
	return none, err
}

func (m *Module) getJSON(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		key           tps.StringOrBytes
		failOnMissing bool
		db            tps.StringOrBytes
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "key", &key, "fail_missing?", &failOnMissing, "db?", &db); err != nil {
		return none, err
	}

	// get value as string
	vs, err := m.getValue(db.GoString(), key.GoBytes(), failOnMissing)
	if err != nil {
		return none, err
	}

	// for unset key, return None
	if vs == nil {
		return none, nil
	}

	// parse JSON
	return dataconv.DecodeStarlarkJSON(vs)
}

func (m *Module) setJSON(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		key   tps.StringOrBytes
		value starlark.Value
		db    tps.StringOrBytes
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "key", &key, "value", &value, "db?", &db); err != nil {
		return none, err
	}

	// convert value to JSON and set
	js, err := dataconv.EncodeStarlarkJSON(value)
	if err != nil {
		return none, err
	}
	return none, m.setValue(db.GoString(), key.GoBytes(), []byte(js))
}

func (m *Module) deleteKey(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		key tps.StringOrBytes
		db  tps.StringOrBytes
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "key", &key, "db?", &db); err != nil {
		return none, err
	}

	// get db client
	dc, err := m.getDBClient(db.GoString())
	if err != nil {
		return none, err
	}

	// delete key
	err = dc.Delete(key.GoBytes())
	return none, err
}

func (m *Module) listItems(db string, syncFirst, keyOnly, valueOnly, reverse bool, limit int) (starlark.Value, error) {
	// get db client
	dc, err := m.getDBClient(db)
	if err != nil {
		return none, err
	}

	// sync before listing
	if syncFirst {
		err = dc.Sync()
		if err != nil {
			return none, err
		}
	}

	// list items
	var (
		cnt = 0
		res = make([]starlark.Value, 0, limit)
	)
	if err := dc.View(func(txn *badger.Txn) error {
		// set iterator options
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		opts.Reverse = reverse
		opts.PrefetchValues = !keyOnly
		it := txn.NewIterator(opts)
		defer it.Close()

		// iterate and collect items
		for it.Rewind(); it.Valid(); it.Next() {
			// check limit
			if cnt++; limit > 0 && cnt > limit {
				break
			}

			// get key
			item := it.Item()
			k := item.Key()
			if keyOnly {
				res = append(res, starlark.String(k))
				continue
			}
			// get value
			err := item.Value(func(v []byte) error {
				if valueOnly {
					res = append(res, starlark.String(v))
				} else {
					res = append(res, starlark.Tuple{starlark.String(k), starlark.String(v)})
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return none, err
	}

	// return list
	return starlark.NewList(res), nil
}

func (m *Module) listKeys(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		db      tps.StringOrBytes
		sync    = true
		reverse bool
		limit   = 0
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "db?", &db, "sync?", &sync, "reverse?", &reverse, "limit?", &limit); err != nil {
		return none, err
	}

	// list keys
	return m.listItems(db.GoString(), sync, true, false, reverse, limit)
}

func (m *Module) listValues(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		db      tps.StringOrBytes
		sync    = true
		reverse bool
		limit   = 0
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "db?", &db, "sync?", &sync, "reverse?", &reverse, "limit?", &limit); err != nil {
		return none, err
	}

	// list values
	return m.listItems(db.GoString(), sync, false, true, reverse, limit)
}

func (m *Module) listAll(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		db      tps.StringOrBytes
		sync    = true
		reverse bool
		limit   = 0
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "db?", &db, "sync?", &sync, "reverse?", &reverse, "limit?", &limit); err != nil {
		return none, err
	}

	// list items
	return m.listItems(db.GoString(), sync, false, false, reverse, limit)
}

func (m *Module) syncDB(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var db tps.StringOrBytes
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "db?", &db); err != nil {
		return none, err
	}

	// get db client
	dc, err := m.getDBClient(db.GoString())
	if err != nil {
		return none, err
	}

	// sync db
	err = dc.Sync()
	return none, err
}

func (m *Module) resetLocalCopy(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var db tps.StringOrBytes
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "db?", &db); err != nil {
		return none, err
	}

	// get db client
	dc, err := m.getDBClient(db.GoString())
	if err != nil {
		return none, err
	}

	// reset local copy
	if err := dc.Reset(); err != nil {
		return none, err
	}

	// remove from cache
	delete(m.dbs, db.GoString())
	return none, nil
}
