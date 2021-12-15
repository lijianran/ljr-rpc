// 2021.12.15
// rpc service

package service

import (
	"go/ast"
	"log"
	"reflect"
	"sync/atomic"
)

// 调用方法
type MethodType struct {
	method    reflect.Method // 方法
	ArgType   reflect.Type   // 参数 1
	ReplyType reflect.Type   // 参数 2
	numCalls  uint64         // 调用次数
}

// 调用次数
func (m *MethodType) NumCalls() uint64 {
	return atomic.LoadUint64(&m.numCalls)
}

func (m *MethodType) NewArgv() reflect.Value {
	var argv reflect.Value

	if m.ArgType.Kind() == reflect.Ptr {
		argv = reflect.New(m.ArgType.Elem())
	} else {
		argv = reflect.New(m.ArgType).Elem()
	}

	return argv
}

func (m *MethodType) NewReplyv() reflect.Value {
	replyv := reflect.New(m.ReplyType.Elem())

	switch m.ReplyType.Elem().Kind() {
	case reflect.Map:
		replyv.Elem().Set(reflect.MakeMap(m.ReplyType.Elem()))

	case reflect.Slice:
		replyv.Elem().Set(reflect.MakeSlice(m.ReplyType.Elem(), 0, 0))
	}

	return replyv
}

// 结构体映射为服务
type Service struct {
	Name   string                 // 映射的结构体名称
	typ    reflect.Type           // 结构体类型
	rcvr   reflect.Value          // 结构体实例
	Method map[string]*MethodType // 结构体符合条件的方法
}

// 映射结构体为服务
func NewService(rcvr interface{}) *Service {
	s := new(Service)
	s.rcvr = reflect.ValueOf(rcvr)
	s.Name = reflect.Indirect(s.rcvr).Type().Name()
	s.typ = reflect.TypeOf(rcvr)

	if !ast.IsExported(s.Name) {
		log.Fatalf("ljr rpc server: %s is not a valid Service name", s.Name)
	}

	s.registerMethods()
	return s
}

// 将结构体的方法映射为服务方法
func (s *Service) registerMethods() {
	s.Method = make(map[string]*MethodType, 0)
	for i := 0; i < s.typ.NumMethod(); i++ {
		// 方法
		method := s.typ.Method(i)
		mType := method.Type

		if mType.NumIn() != 3 || mType.NumOut() != 1 {
			// 参数个数或返回值个数不符合要求
			continue
		}

		if mType.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
			// 返回值不是 error 类型
			continue
		}

		// 参数 1 参数 2
		argType, replyType := mType.In(1), mType.In(2)
		if !isExportedOrBuildinType(argType) || !isExportedOrBuildinType(replyType) {
			// 参数类型不是导出类型或内置类型
			continue
		}

		s.Method[method.Name] = &MethodType{
			method:    method,
			ArgType:   argType,
			ReplyType: replyType,
		}

		log.Printf("ljr rpc server: register %s.%s\n", s.Name, method.Name)

	}
}

// 判断参数类型是不是导出类型或内置类型
func isExportedOrBuildinType(t reflect.Type) bool {
	return ast.IsExported(t.Name()) || t.PkgPath() == ""
}

func (s *Service) Call(m *MethodType, argv reflect.Value, replyv reflect.Value) error {
	// 调用次数加一
	atomic.AddUint64(&m.numCalls, 1)
	f := m.method.Func
	returnValues := f.Call([]reflect.Value{s.rcvr, argv, replyv})
	if errInter := returnValues[0].Interface(); errInter != nil {
		return errInter.(error)
	}
	return nil
}
