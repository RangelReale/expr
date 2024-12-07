package vm

//go:generate sh -c "go run ./func_types > ./func_types[generated].go"

import (
	"fmt"
	"math"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/expr-lang/expr/builtin"
	"github.com/expr-lang/expr/file"
	"github.com/expr-lang/expr/internal/deref"
	"github.com/expr-lang/expr/vm/runtime"
	operin_poc1 "github.com/rrgmc/operin-poc1"
	"github.com/rrgmc/operin-poc1/operincore"
	"github.com/rrgmc/operin-poc1/operintypeslice"
)

func Run(program *Program, env any) (any, error) {
	if program == nil {
		return nil, fmt.Errorf("program is nil")
	}

	vm := VM{}
	InitVM(&vm)
	return vm.Run(program, env)
}

func InitVM(vm *VM) {
	vm.operinConfig = operin_poc1.NewConfig(
		operin_poc1.WithCastIntsAndFloats(true),
		operin_poc1.WithDefaultIntValue(func(value int64) operin_poc1.Type {
			return operin_poc1.IntValue(value)
		}),
		operin_poc1.WithDefaultUintValue(func(value uint64) operin_poc1.Type {
			return operin_poc1.UintValue(value)
		}),
		operin_poc1.WithDefaultFloatValue(func(value float64) operin_poc1.Type {
			return operin_poc1.Float64Value(value)
		}),
		operin_poc1.WithTypeFactory(operintypeslice.NewPrimitiveTypeFactory()),
		// operin_poc1.WithTypeFactory(operintype.NewPrimitiveReflectTypeFactory()),
		operin_poc1.WithTypeFactory(operincore.TypeFactoryFunc(func(value any, options operincore.Options) (operincore.Type, bool) {
			return operin_poc1.ValueDeepEqual{value}, true
		})),
	)
}

func Debug() *VM {
	vm := &VM{
		debug: true,
		step:  make(chan struct{}, 0),
		curr:  make(chan int, 0),
	}
	InitVM(vm)
	return vm
}

type VM struct {
	Stack        []any
	Scopes       []*Scope
	Variables    []any
	ip           int
	memory       uint
	memoryBudget uint
	debug        bool
	step         chan struct{}
	curr         chan int
	operinConfig *operin_poc1.Config
}

func (vm *VM) Run(program *Program, env any) (_ any, err error) {
	defer func() {
		if r := recover(); r != nil {
			var location file.Location
			if vm.ip-1 < len(program.locations) {
				location = program.locations[vm.ip-1]
			}
			f := &file.Error{
				Location: location,
				Message:  fmt.Sprintf("%v", r),
			}
			if err, ok := r.(error); ok {
				f.Wrap(err)
			}
			err = f.Bind(program.source)
		}
	}()

	if vm.Stack == nil {
		vm.Stack = make([]any, 0, 2)
	} else {
		vm.Stack = vm.Stack[0:0]
	}
	if vm.Scopes != nil {
		vm.Scopes = vm.Scopes[0:0]
	}
	if len(vm.Variables) < program.variables {
		vm.Variables = make([]any, program.variables)
	}

	vm.memoryBudget = MemoryBudget
	vm.memory = 0
	vm.ip = 0

	for vm.ip < len(program.Bytecode) {
		if debug && vm.debug {
			<-vm.step
		}

		op := program.Bytecode[vm.ip]
		arg := program.Arguments[vm.ip]
		vm.ip += 1

		switch op {

		case OpInvalid:
			panic("invalid opcode")

		case OpPush:
			vm.push(program.Constants[arg])

		case OpInt:
			vm.push(arg)

		case OpPop:
			vm.pop()

		case OpStore:
			vm.Variables[arg] = vm.pop()

		case OpLoadVar:
			vm.push(vm.Variables[arg])

		case OpLoadConst:
			vm.push(runtime.Fetch(env, program.Constants[arg]))

		case OpLoadField:
			vm.push(runtime.FetchField(env, program.Constants[arg].(*runtime.Field)))

		case OpLoadFast:
			vm.push(env.(map[string]any)[program.Constants[arg].(string)])

		case OpLoadMethod:
			vm.push(runtime.FetchMethod(env, program.Constants[arg].(*runtime.Method)))

		case OpLoadFunc:
			vm.push(program.functions[arg])

		case OpFetch:
			b := vm.pop()
			a := vm.pop()
			vm.push(runtime.Fetch(a, b))

		case OpFetchField:
			a := vm.pop()
			vm.push(runtime.FetchField(a, program.Constants[arg].(*runtime.Field)))

		case OpLoadEnv:
			vm.push(env)

		case OpMethod:
			a := vm.pop()
			vm.push(runtime.FetchMethod(a, program.Constants[arg].(*runtime.Method)))

		case OpTrue:
			vm.push(true)

		case OpFalse:
			vm.push(false)

		case OpNil:
			vm.push(nil)

		case OpNegate:
			// v := runtime.Negate(vm.pop())
			// vm.push(v)
			vm.pushType(vm.operinConfig.Negate(vm.pop()))

		case OpNot:
			v := vm.pop().(bool)
			vm.push(!v)

		case OpEqual:
			b := vm.pop()
			a := vm.pop()
			// vm.push(runtime.Equal(a, b))
			v, err := vm.operinConfig.EqualsCheck(a, b)
			if err != nil {
				panic(err)
			}
			vm.push(v)

		case OpEqualInt:
			b := vm.pop()
			a := vm.pop()
			// vm.push(a.(int) == b.(int))
			v, err := vm.operinConfig.EqualsCheck(a.(int), b.(int))
			if err != nil {
				panic(err)
			}
			vm.push(v)

		case OpEqualString:
			b := vm.pop()
			a := vm.pop()
			// vm.push(a.(string) == b.(string))
			v, err := vm.operinConfig.EqualsCheck(a.(string), b.(string))
			if err != nil {
				panic(err)
			}
			vm.push(v)

		case OpJump:
			vm.ip += arg

		case OpJumpIfTrue:
			if vm.current().(bool) {
				vm.ip += arg
			}

		case OpJumpIfFalse:
			if !vm.current().(bool) {
				vm.ip += arg
			}

		case OpJumpIfNil:
			if runtime.IsNil(vm.current()) {
				vm.ip += arg
			}

		case OpJumpIfNotNil:
			if !runtime.IsNil(vm.current()) {
				vm.ip += arg
			}

		case OpJumpIfEnd:
			scope := vm.scope()
			if scope.Index >= scope.Len {
				vm.ip += arg
			}

		case OpJumpBackward:
			vm.ip -= arg

		case OpIn:
			b := vm.pop()
			a := vm.pop()
			vm.push(runtime.In(a, b))

		case OpLess:
			b := vm.pop()
			a := vm.pop()
			// vm.push(runtime.Less(a, b))
			v, err := vm.operinConfig.LessCheck(a, b)
			if err != nil {
				panic(err)
			}
			vm.push(v)

		case OpMore:
			b := vm.pop()
			a := vm.pop()
			// vm.push(runtime.More(a, b))
			v, err := vm.operinConfig.GreaterCheck(a, b)
			if err != nil {
				panic(err)
			}
			vm.push(v)

		case OpLessOrEqual:
			b := vm.pop()
			a := vm.pop()
			// vm.push(runtime.LessOrEqual(a, b))
			v, err := vm.operinConfig.LessEqCheck(a, b)
			if err != nil {
				panic(err)
			}
			vm.push(v)

		case OpMoreOrEqual:
			b := vm.pop()
			a := vm.pop()
			// vm.push(runtime.MoreOrEqual(a, b))
			v, err := vm.operinConfig.GreaterEqCheck(a, b)
			if err != nil {
				panic(err)
			}
			vm.push(v)

		case OpAdd:
			b := vm.pop()
			a := vm.pop()
			// vm.push(runtime.Add(a, b))
			vm.pushType(vm.operinConfig.Add(a, b))

		case OpSubtract:
			b := vm.pop()
			a := vm.pop()
			// vm.push(runtime.Subtract(a, b))
			vm.pushType(vm.operinConfig.Subtract(a, b))

		case OpMultiply:
			b := vm.pop()
			a := vm.pop()
			// vm.push(runtime.Multiply(a, b))
			vm.pushType(vm.operinConfig.Multiply(a, b))

		case OpDivide:
			b := vm.pop()
			a := vm.pop()
			// vm.push(runtime.Divide(a, b))
			// vm.pushType(vm.operinConfig.Divide(a, b))
			vm.pushType(vm.operinConfig.DivideFloat(a, b))

		case OpModulo:
			b := vm.pop()
			a := vm.pop()
			// vm.push(runtime.Modulo(a, b))
			vm.pushType(vm.operinConfig.Modulo(a, b))

		case OpExponent:
			b := vm.pop()
			a := vm.pop()
			// vm.push(runtime.Exponent(a, b))
			v, err := vm.operinConfig.CalcFloat(a, b, func(x, y float64) float64 {
				return math.Pow(x, y)
			})
			if err != nil {
				panic(fmt.Sprintf("invalid operation: float(%T, %T): %s", a, b, err))
			}
			vm.push(v)

		case OpRange:
			b := vm.pop()
			a := vm.pop()
			min := runtime.ToInt(a)
			max := runtime.ToInt(b)
			size := max - min + 1
			if size <= 0 {
				size = 0
			}
			vm.memGrow(uint(size))
			vm.push(runtime.MakeRange(min, max))

		case OpMatches:
			b := vm.pop()
			a := vm.pop()
			if runtime.IsNil(a) || runtime.IsNil(b) {
				vm.push(false)
				break
			}
			match, err := regexp.MatchString(b.(string), a.(string))
			if err != nil {
				panic(err)
			}
			vm.push(match)

		case OpMatchesConst:
			a := vm.pop()
			if runtime.IsNil(a) {
				vm.push(false)
				break
			}
			r := program.Constants[arg].(*regexp.Regexp)
			vm.push(r.MatchString(a.(string)))

		case OpContains:
			b := vm.pop()
			a := vm.pop()
			if runtime.IsNil(a) || runtime.IsNil(b) {
				vm.push(false)
				break
			}
			vm.push(strings.Contains(a.(string), b.(string)))

		case OpStartsWith:
			b := vm.pop()
			a := vm.pop()
			if runtime.IsNil(a) || runtime.IsNil(b) {
				vm.push(false)
				break
			}
			vm.push(strings.HasPrefix(a.(string), b.(string)))

		case OpEndsWith:
			b := vm.pop()
			a := vm.pop()
			if runtime.IsNil(a) || runtime.IsNil(b) {
				vm.push(false)
				break
			}
			vm.push(strings.HasSuffix(a.(string), b.(string)))

		case OpSlice:
			from := vm.pop()
			to := vm.pop()
			node := vm.pop()
			vm.push(runtime.Slice(node, from, to))

		case OpCall:
			fn := reflect.ValueOf(vm.pop())
			size := arg
			in := make([]reflect.Value, size)
			for i := int(size) - 1; i >= 0; i-- {
				param := vm.pop()
				if param == nil && reflect.TypeOf(param) == nil {
					// In case of nil value and nil type use this hack,
					// otherwise reflect.Call will panic on zero value.
					in[i] = reflect.ValueOf(&param).Elem()
				} else {
					in[i] = reflect.ValueOf(param)
				}
			}
			out := fn.Call(in)
			if len(out) == 2 && out[1].Type() == errorType && !out[1].IsNil() {
				panic(out[1].Interface().(error))
			}
			vm.push(out[0].Interface())

		case OpCall0:
			out, err := program.functions[arg]()
			if err != nil {
				panic(err)
			}
			vm.push(out)

		case OpCall1:
			a := vm.pop()
			out, err := program.functions[arg](a)
			if err != nil {
				panic(err)
			}
			vm.push(out)

		case OpCall2:
			b := vm.pop()
			a := vm.pop()
			out, err := program.functions[arg](a, b)
			if err != nil {
				panic(err)
			}
			vm.push(out)

		case OpCall3:
			c := vm.pop()
			b := vm.pop()
			a := vm.pop()
			out, err := program.functions[arg](a, b, c)
			if err != nil {
				panic(err)
			}
			vm.push(out)

		case OpCallN:
			fn := vm.pop().(Function)
			size := arg
			in := make([]any, size)
			for i := int(size) - 1; i >= 0; i-- {
				in[i] = vm.pop()
			}
			out, err := fn(in...)
			if err != nil {
				panic(err)
			}
			vm.push(out)

		case OpCallFast:
			fn := vm.pop().(func(...any) any)
			size := arg
			in := make([]any, size)
			for i := int(size) - 1; i >= 0; i-- {
				in[i] = vm.pop()
			}
			vm.push(fn(in...))

		case OpCallSafe:
			fn := vm.pop().(SafeFunction)
			size := arg
			in := make([]any, size)
			for i := int(size) - 1; i >= 0; i-- {
				in[i] = vm.pop()
			}
			out, mem, err := fn(in...)
			if err != nil {
				panic(err)
			}
			vm.memGrow(mem)
			vm.push(out)

		case OpCallTyped:
			vm.push(vm.call(vm.pop(), arg))

		case OpCallBuiltin1:
			vm.push(builtin.Builtins[arg].Fast(vm.pop()))

		case OpArray:
			size := vm.pop().(int)
			vm.memGrow(uint(size))
			array := make([]any, size)
			for i := size - 1; i >= 0; i-- {
				array[i] = vm.pop()
			}
			vm.push(array)

		case OpMap:
			size := vm.pop().(int)
			vm.memGrow(uint(size))
			m := make(map[string]any)
			for i := size - 1; i >= 0; i-- {
				value := vm.pop()
				key := vm.pop()
				m[key.(string)] = value
			}
			vm.push(m)

		case OpLen:
			vm.push(runtime.Len(vm.current()))

		case OpCast:
			switch arg {
			case 0:
				vm.push(runtime.ToInt(vm.pop()))
			case 1:
				vm.push(runtime.ToInt64(vm.pop()))
			case 2:
				vm.push(runtime.ToFloat64(vm.pop()))
			}

		case OpDeref:
			a := vm.pop()
			vm.push(deref.Deref(a))

		case OpIncrementIndex:
			vm.scope().Index++

		case OpDecrementIndex:
			scope := vm.scope()
			scope.Index--

		case OpIncrementCount:
			scope := vm.scope()
			scope.Count++

		case OpGetIndex:
			vm.push(vm.scope().Index)

		case OpGetCount:
			scope := vm.scope()
			vm.push(scope.Count)

		case OpGetLen:
			scope := vm.scope()
			vm.push(scope.Len)

		case OpGetAcc:
			vm.push(vm.scope().Acc)

		case OpSetAcc:
			vm.scope().Acc = vm.pop()

		case OpSetIndex:
			scope := vm.scope()
			scope.Index = vm.pop().(int)

		case OpPointer:
			scope := vm.scope()
			vm.push(scope.Array.Index(scope.Index).Interface())

		case OpThrow:
			panic(vm.pop().(error))

		case OpCreate:
			switch arg {
			case 1:
				vm.push(make(groupBy))
			case 2:
				scope := vm.scope()
				var desc bool
				switch vm.pop().(string) {
				case "asc":
					desc = false
				case "desc":
					desc = true
				default:
					panic("unknown order, use asc or desc")
				}
				vm.push(&runtime.SortBy{
					Desc:   desc,
					Array:  make([]any, 0, scope.Len),
					Values: make([]any, 0, scope.Len),
				})
			default:
				panic(fmt.Sprintf("unknown OpCreate argument %v", arg))
			}

		case OpGroupBy:
			scope := vm.scope()
			key := vm.pop()
			item := scope.Array.Index(scope.Index).Interface()
			scope.Acc.(groupBy)[key] = append(scope.Acc.(groupBy)[key], item)

		case OpSortBy:
			scope := vm.scope()
			value := vm.pop()
			item := scope.Array.Index(scope.Index).Interface()
			sortable := scope.Acc.(*runtime.SortBy)
			sortable.Array = append(sortable.Array, item)
			sortable.Values = append(sortable.Values, value)

		case OpSort:
			scope := vm.scope()
			sortable := scope.Acc.(*runtime.SortBy)
			sort.Sort(sortable)
			vm.memGrow(uint(scope.Len))
			vm.push(sortable.Array)

		case OpProfileStart:
			span := program.Constants[arg].(*Span)
			span.start = time.Now()

		case OpProfileEnd:
			span := program.Constants[arg].(*Span)
			span.Duration += time.Since(span.start).Nanoseconds()

		case OpBegin:
			a := vm.pop()
			array := reflect.ValueOf(a)
			vm.Scopes = append(vm.Scopes, &Scope{
				Array: array,
				Len:   array.Len(),
			})

		case OpEnd:
			vm.Scopes = vm.Scopes[:len(vm.Scopes)-1]

		default:
			panic(fmt.Sprintf("unknown bytecode %#x", op))
		}

		if debug && vm.debug {
			vm.curr <- vm.ip
		}
	}

	if debug && vm.debug {
		close(vm.curr)
		close(vm.step)
	}

	if len(vm.Stack) > 0 {
		return vm.pop(), nil
	}

	return nil, nil
}

func (vm *VM) push(value any) {
	vm.Stack = append(vm.Stack, value)
}

func (vm *VM) pushType(value operin_poc1.Type) {
	if !value.IsValueValid() {
		panic(fmt.Sprintf("type '%s' has no valid value", value.TypeName()))
	}
	vm.push(value.Value())
}

func (vm *VM) current() any {
	return vm.Stack[len(vm.Stack)-1]
}

func (vm *VM) pop() any {
	value := vm.Stack[len(vm.Stack)-1]
	vm.Stack = vm.Stack[:len(vm.Stack)-1]
	return value
}

func (vm *VM) memGrow(size uint) {
	vm.memory += size
	if vm.memory >= vm.memoryBudget {
		panic("memory budget exceeded")
	}
}

func (vm *VM) scope() *Scope {
	return vm.Scopes[len(vm.Scopes)-1]
}

func (vm *VM) Step() {
	vm.step <- struct{}{}
}

func (vm *VM) Position() chan int {
	return vm.curr
}
