package {{.PackageName}}

// Code generated by replaygen. DO NOT EDIT.

import (
	"context"
	"time"

    "github.com/corverroos/replay"
	"github.com/golang/protobuf/proto"
	"github.com/luno/reflex"

	// TODO(corver): Support importing other packages.
)

const (
	_ns     = "{{.Name}}"
	{{- range .Workflows}}
	_w{{.Pascal}} = "{{.Name}}"
	{{- end}}
	{{- range .Activities}}
    _a{{.Pascal}} = "{{.Name}}"
    {{- end}}
)

{{range .Workflows}} {{$workflowName := .Name}} {{$workflowCamel := .Camel}} {{$workflowPascal := .Pascal}}
{{if .Signals}}
type {{$workflowCamel}}Signal int

const (
	{{- range $i, $s := .Signals}}
	_s{{$workflowPascal}}{{$s.Pascal}} {{$workflowCamel}}Signal = {{inc $i}}
	{{- end}}
)

var {{$workflowCamel}}SignalMessages = map[{{$workflowCamel}}Signal]proto.Message{
    {{- range .Signals}}
	_s{{$workflowPascal}}{{.Pascal}}: new({{.Message}}),
	{{- end}}
}

func (s {{$workflowCamel}}Signal) SignalType() int {
	return int(s)
}

func (s {{$workflowCamel}}Signal) MessageType() proto.Message {
	return {{$workflowCamel}}SignalMessages[s]
}
{{end}}

{{- range .Signals}}
// Signal{{$workflowPascal}}{{.Pascal}} provides a typed API for signalling a {{$workflowName}} workflow run with signal {{.Name}}.
func Signal{{$workflowPascal}}{{.Pascal}}(ctx context.Context, cl replay.Client, run string, message *{{.Message}}, extID string) error {
	return cl.SignalRun(ctx, _ns, _w{{$workflowPascal}}, run, _s{{$workflowPascal}}{{.Pascal}}, message, extID)
}
{{- end}}

// Run{{$workflowPascal}} provides a type API for running the {{.Name}} workflow.
func Run{{$workflowPascal}}(ctx context.Context, cl replay.Client, run string, message *{{.Input}}) error {
	return cl.RunWorkflow(ctx, _ns, _w{{$workflowPascal}}, run, message)
}
{{- end}}

// startReplayLoops registers the workflow and activities for typed workflow functions.
func startReplayLoops(getCtx func() context.Context, cl replay.Client, cstore reflex.CursorStore, b Backends,
    {{range .Workflows}} {{.Camel}} func({{.Camel}}Flow, *{{.Input}}), {{end}} ){

	{{range .Workflows}}
	{{.Camel}}Func := func(ctx replay.RunContext, message *{{.Input}}) {
		{{.Camel}}({{.Camel}}FlowImpl{ctx}, message)
	}
	replay.RegisterWorkflow(getCtx, cl, cstore, _ns, {{.Camel}}Func, replay.WithName(_w{{.Pascal}}))
	{{end}}

	{{- range .Activities}}
	replay.RegisterActivity(getCtx, cl, cstore, b, _ns, {{.FuncName}}, replay.WithName(_a{{.Pascal}}))
	{{- end}}
}

{{$al := .Activities}}
{{- range .Workflows}} {{$workflowCamel := .Camel}} {{$workflowPascal := .Pascal}}
// {{.Camel}}Flow defines a typed API for the {{.Name}} workflow.
type {{.Camel}}Flow interface {

    // Sleep blocks for at least d duration.
    // Note that replay sleeps aren't very accurate and
    // a few seconds is the practical minimum.
	Sleep(d time.Duration)

	// CreateEvent returns the reflex event that started the run iteration (type is internal.CreateRun).
	// The event timestamp could be used to reason about run age.
	CreateEvent() *reflex.Event

	// LastEvent returns the latest reflex event (type is either internal.CreateRun or internal.ActivityResponse).
    // The event timestamp could be used to reason about run age.
	LastEvent() *reflex.Event

	// Run returns the run name/identifier.
	Run() string

	// Restart completes the current run iteration and starts a new run iteration with the provided input message.
	// The run state is effectively reset. This is handy to mitigate bootstrap load for long running tasks.
	// It also allows updating the activity logic/ordering.
	Restart(message *{{.Input}})

	{{range $al}}
	// {{.FuncTitle}} results in the {{.FuncName}} activity being called asynchronously
	// with the provided parameter and returns the response once available.
	{{.FuncTitle}}(message *{{.Input}}) *{{.Output}}
	{{end}}
	{{- range .Signals}}

	// Await{{.Pascal}} blocks and returns true when a {{.Name}} signal is/was
	// received for this run. If no signal is/was received it returns false after d duration.
	Await{{.Pascal}}(d time.Duration) (*{{.Message}}, bool)
	{{- end}}
}

type {{.Camel}}FlowImpl struct {
	ctx replay.RunContext
}

func (f {{.Camel}}FlowImpl) Sleep(d time.Duration) {
	f.ctx.Sleep(d)
}

func (f {{.Camel}}FlowImpl) CreateEvent() *reflex.Event {
	return f.ctx.CreateEvent()
}

func (f {{.Camel}}FlowImpl) LastEvent() *reflex.Event {
	return f.ctx.LastEvent()
}

func (f {{.Camel}}FlowImpl) Run() string {
	return f.ctx.Run()
}

func (f {{.Camel}}FlowImpl) Restart(message *{{.Input}}) {
	f.ctx.Restart(message)
}

{{range $al}}
func (f {{$workflowCamel}}FlowImpl) {{.FuncTitle}}(message *{{.Input}}) *{{.Output}} {
	return f.ctx.ExecActivity({{.FuncName}}, message, replay.WithName(_a{{.Pascal}})).(*{{.Output}})
}
{{end}}

{{range .Signals}}
func (f {{$workflowCamel}}FlowImpl) Await{{.Pascal}}(d time.Duration) (*{{.Message}}, bool) {
	res, ok := f.ctx.AwaitSignal(_s{{$workflowPascal}}{{.Pascal}}, d)
	if !ok {
		return nil, false
	}
	return res.(*{{.Message}}), true
}
{{end}}
{{end}}
