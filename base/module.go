package base

import (
	"fmt"
	"github.com/1set/starlet"
	"github.com/1set/starlet/dataconv"
	"go.starlark.net/starlark"
)

type ConfigGetter[T any] func() T

type BaseModule[T any] struct {
	configs map[string]ConfigGetter[T]
}

func NewBaseModule[T any]() *BaseModule[T] {
	return &BaseModule[T]{configs: make(map[string]ConfigGetter[T])}
}

func (m *BaseModule[T]) SetConfig(name string, getter ConfigGetter[T]) {
	m.configs[name] = getter
}

func (m *BaseModule[T]) genSetConfig(name string) starlark.Callable {
	return starlark.NewBuiltin(name, func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var v starlark.Value
		if err := starlark.UnpackArgs(b.Name(), args, kwargs, name, &v); err != nil {
			return starlark.None, err
		}
		m.configs[name] = func() T { return dataconv.ToGoValue(v).(T) }
		return starlark.None, nil
	})
}

func (m *BaseModule[T]) GetConfig(name string) (T, error) {
	getter, exists := m.configs[name]
	if !exists {
		var zero T
		return zero, fmt.Errorf("config %s not set", name)
	}
	return getter(), nil
}

func (m *BaseModule[T]) LoadModule(moduleName string, additionalFuncs starlark.StringDict) starlet.ModuleLoader {
	sd := starlark.StringDict{}
	for name := range m.configs {
		sd["set_"+name] = m.genSetConfig(name)
	}
	for k, v := range additionalFuncs {
		sd[k] = v
	}
	return dataconv.WrapModuleData(moduleName, sd)
}
